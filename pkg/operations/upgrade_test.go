/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"context"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

func TestUpgradeInstanceTemplates(t *testing.T) {
	for _, sharding := range []bool{false, true} {
		for _, tc := range []struct {
			name                string
			definition, version *string
			parent, supersede   bool
			wantDef, wantVer    string
		}{
			{name: "definition only", definition: pointer.String("pg-new"), wantDef: "pg-new", wantVer: "18.0.0"},
			{name: "version only", version: pointer.String("19.0.0"), wantDef: "pg-canary", wantVer: "19.0.0"},
			{name: "both", definition: pointer.String("pg-new"), version: pointer.String("19.0.0"), wantDef: "pg-new", wantVer: "19.0.0"},
			{name: "mixed parent", definition: pointer.String("pg-new"), parent: true, wantDef: "pg-new", wantVer: "18.0.0"},
			{name: "clear definition", definition: pointer.String(""), wantVer: "18.0.0"},
			{name: "clear version", version: pointer.String(""), wantDef: "pg-canary"},
			{name: "clear both", definition: pointer.String(""), version: pointer.String("")},
			{name: "supersede same-owner template upgrade", definition: pointer.String("pg-new"), supersede: true, wantDef: "pg-new", wantVer: "18.0.0"},
		} {
			owner := "component/"
			if sharding {
				owner = "sharding/"
			}
			t.Run(owner+tc.name, func(t *testing.T) {
				ctx := context.Background()
				reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				component := appsv1.ClusterComponentSpec{Name: "pg", ComponentDef: "pg-old", ServiceVersion: "17.0.0", Replicas: 3,
					Instances: []appsv1.InstanceTemplate{
						{Name: "stable", CompDef: "pg-stable", ServiceVersion: "17.0.0", Replicas: pointer.Int32(1)},
						{Name: "canary", CompDef: "pg-canary", ServiceVersion: "18.0.0", Replicas: pointer.Int32(1), Canary: pointer.Bool(true),
							Ordinals: appsv1.Ordinals{Discrete: []int32{2}}, Labels: map[string]string{"test": "keep"}, Annotations: map[string]string{"test": "keep"},
							Resources:        &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}},
							SchedulingPolicy: &appsv1.SchedulingPolicy{NodeSelector: map[string]string{"zone": "a"}},
							VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
							}}}},
						{Name: "sibling", CompDef: "pg-sibling", ServiceVersion: "16.0.0", Replicas: pointer.Int32(1)},
					}}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
					Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}},
					Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
				fullComponentName := "pg"
				if sharding {
					cluster.Spec.ComponentSpecs = nil
					cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "pg", Shards: 1, Template: component,
						ShardTemplates: []appsv1.ShardTemplate{{Name: "zone-a", Shards: pointer.Int32(1)}}}}
					fullComponentName = "pg-a"
				}
				upgrade := opsv1alpha1.UpgradeComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "pg"},
					Instances: []opsv1alpha1.UpgradeInstanceTemplate{
						{Name: "canary", ComponentDefinitionName: tc.definition, ServiceVersion: tc.version},
						{Name: "stable", ServiceVersion: pointer.String("19.0.0")},
					}}
				want := component.DeepCopy()
				want.Instances[0].ServiceVersion = "19.0.0"
				want.Instances[1].CompDef, want.Instances[1].ServiceVersion = tc.wantDef, tc.wantVer
				if tc.parent {
					upgrade.ComponentDefinitionName, upgrade.ServiceVersion = pointer.String("pg-parent"), pointer.String("20.0.0")
					want.ComponentDef, want.ServiceVersion = "pg-parent", "20.0.0"
				}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "default"},
					Spec: opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.UpgradeType, Force: tc.supersede,
						SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{upgrade}}}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, cluster).WithObjects(cluster, ops).Build()
				res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(50)}
				previous := &opsv1alpha1.OpsRequest{}
				if tc.supersede {
					previous = ops.DeepCopy()
					previous.Name, previous.ResourceVersion = "earlier-upgrade", ""
					previous.Spec.Force = false
					previous.Spec.Upgrade.Components[0].Instances = []opsv1alpha1.UpgradeInstanceTemplate{{Name: "sibling", ServiceVersion: pointer.String("17.0.0")}}
					previous.Status.Phase = opsv1alpha1.OpsRunningPhase
					if err := cli.Create(ctx, previous); err != nil {
						t.Fatal(err)
					}
					if err := opsutil.UpdateClusterOpsAnnotations(ctx, cli, cluster, []opsv1alpha1.OpsRecorder{{Name: previous.Name, Type: opsv1alpha1.UpgradeType}}); err != nil {
						t.Fatal(err)
					}
				}
				reload := func() {
					t.Helper()
					res.Cluster, res.OpsRequest, res.Runtimes = &appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}, nil
					if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), res.Cluster); err != nil {
						t.Fatal(err)
					}
					if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), res.OpsRequest); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := GetOpsManager().Do(reqCtx, cli, res); err != nil {
					t.Fatal(err)
				}
				reload()
				if res.OpsRequest.Status.Phase != opsv1alpha1.OpsCreatingPhase {
					t.Fatalf("phase = %s, want Creating", res.OpsRequest.Status.Phase)
				}
				for i := 0; i < 2; i++ {
					if _, err := GetOpsManager().Do(reqCtx, cli, res); err != nil {
						t.Fatal(err)
					}
					reload()
				}
				got := getComponentSpecOrShardingTemplate(res.Cluster, "pg")
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("component = %#v, want %#v", got, want)
				}
				last := res.OpsRequest.Status.LastConfiguration.Components["pg"]
				if !reflect.DeepEqual(last.InstanceTemplates, component.Instances) || last.ComponentDefinitionName != component.ComponentDef || last.ServiceVersion != component.ServiceVersion {
					t.Fatalf("last configuration = %#v, want original component", last)
				}
				if tc.supersede {
					if err := cli.Get(ctx, client.ObjectKeyFromObject(previous), previous); err != nil {
						t.Fatal(err)
					}
					if previous.Status.Phase != opsv1alpha1.OpsAbortedPhase || previous.Status.CompletionTimestamp.IsZero() {
						t.Fatal("earlier Upgrade on another template of the same owner was not aborted")
					}
				}
				if sharding {
					comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + fullComponentName, Namespace: "default",
						Labels: constant.GetCompLabels("demo", fullComponentName, map[string]string{constant.KBAppShardingNameLabelKey: "pg"})}}
					if err := cli.Create(ctx, comp); err != nil {
						t.Fatal(err)
					}
				}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + fullComponentName, Namespace: "default"}, Spec: workloads.InstanceSetSpec{Replicas: pointer.Int32(3)}}
				for _, name := range []string{"stable-0", "canary-2", "sibling-0"} {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: its.Name + "-" + name,
						DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent,
						UpToDate: true, Ready: true, Available: true})
				}
				if err := cli.Create(ctx, its); err != nil {
					t.Fatal(err)
				}
				res.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
				res.OpsRequest.Status.ClusterGeneration = res.Cluster.Generation
				if err := cli.Status().Update(ctx, res.OpsRequest); err != nil {
					t.Fatal(err)
				}
				if _, err := GetOpsManager().Reconcile(reqCtx, cli, res); err != nil {
					t.Fatal(err)
				}
				reload()
				if res.OpsRequest.Status.Phase != opsv1alpha1.OpsRunningPhase {
					t.Fatal("upgrade completed before the owner published current status")
				}
				var canary *appsv1.InstanceTemplate
				if sharding {
					canary = &res.Cluster.Spec.Shardings[0].Template.Instances[1]
				} else {
					canary = &res.Cluster.Spec.ComponentSpecs[0].Instances[1]
				}
				if canary.CompDef == "" && canary.ServiceVersion != "" {
					canary.CompDef = "pg-resolved"
				}
				if canary.ServiceVersion == "" && canary.CompDef != "" {
					canary.ServiceVersion = "21.0.0"
				}
				if err := cli.Update(ctx, res.Cluster); err != nil {
					t.Fatal(err)
				}
				status := appsv1.ClusterComponentStatus{Phase: appsv1.RunningComponentPhase, ObservedGeneration: res.Cluster.Generation, UpToDate: true}
				if sharding {
					res.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{"pg": {
						Phase: status.Phase, ObservedGeneration: status.ObservedGeneration, UpToDate: status.UpToDate,
					}}
				} else {
					res.Cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{"pg": status}
				}
				if err := cli.Status().Update(ctx, res.Cluster); err != nil {
					t.Fatal(err)
				}
				if _, err := GetOpsManager().Reconcile(reqCtx, cli, res); err != nil {
					t.Fatal(err)
				}
				reload()
				if res.OpsRequest.Status.Phase != opsv1alpha1.OpsSucceedPhase || res.OpsRequest.Status.Progress != "3/3" {
					t.Fatalf("phase = %s, progress = %s, want Succeed 3/3", res.OpsRequest.Status.Phase, res.OpsRequest.Status.Progress)
				}
			})
		}
	}
}

func TestUpgradeTargetsUnchanged(t *testing.T) {
	explicitCompDef := "mysql-8.0"
	explicitServiceVersion := "8.0.36"
	latest := ""
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{{
			Name: "mysql", ComponentDef: explicitCompDef, ServiceVersion: explicitServiceVersion,
		}},
		Shardings: []appsv1.ClusterSharding{{
			Name: "shard", Template: appsv1.ClusterComponentSpec{ComponentDef: explicitCompDef, ServiceVersion: explicitServiceVersion},
		}},
	}}
	handler := upgradeOpsHandler{}
	newOpsResource := func(componentName string, componentDef, serviceVersion *string) *OpsResource {
		return &OpsResource{Cluster: cluster, OpsRequest: &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
				Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
					ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: componentName},
					ComponentDefinitionName: componentDef,
					ServiceVersion:          serviceVersion,
				}}},
			},
		}}}
	}

	if !handler.targetsUnchanged(newOpsResource("mysql", &explicitCompDef, &explicitServiceVersion)) {
		t.Fatal("explicit component target did not match")
	}
	if !handler.targetsUnchanged(newOpsResource("shard", &explicitCompDef, &explicitServiceVersion)) {
		t.Fatal("explicit sharding target did not match")
	}
	cluster.Spec.Services = []appsv1.ClusterService{{Service: appsv1.Service{Name: "unrelated"}}}
	if !handler.targetsUnchanged(newOpsResource("mysql", &explicitCompDef, &explicitServiceVersion)) {
		t.Fatal("unrelated Cluster change replaced the upgrade target")
	}
	cluster.Spec.ComponentSpecs[0].ServiceVersion = "8.4.0"
	if handler.targetsUnchanged(newOpsResource("mysql", &explicitCompDef, &explicitServiceVersion)) {
		t.Fatal("replaced serviceVersion was accepted")
	}
	cluster.Spec.ComponentSpecs[0].ServiceVersion = explicitServiceVersion
	cluster.Spec.ComponentSpecs[0].ComponentDef = "mysql-8.4"
	if handler.targetsUnchanged(newOpsResource("mysql", &explicitCompDef, &explicitServiceVersion)) {
		t.Fatal("replaced componentDef was accepted")
	}
	cluster.Spec.ComponentSpecs[0].ServiceVersion = "resolved-latest"
	if !handler.targetsUnchanged(newOpsResource("mysql", &latest, &latest)) {
		t.Fatal("non-exact latest target rejected the owner-resolved values")
	}

	opsRes := newOpsResource("mysql", &explicitCompDef, &explicitServiceVersion)
	opsRes.Cluster.Generation = 7
	opsRes.OpsRequest.Status.ClusterGeneration = 8
	phase, _, err := handler.ReconcileAction(intctrlutil.RequestCtx{}, nil, opsRes)
	if err != nil || phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("phase=%s err=%v, want Running before the action generation is observed", phase, err)
	}
	opsRes.Cluster.Generation = 8
	phase, _, err = handler.ReconcileAction(intctrlutil.RequestCtx{}, nil, opsRes)
	if err != nil || phase != opsv1alpha1.OpsAbortedPhase {
		t.Fatalf("phase=%s err=%v, want Aborted for a replaced explicit target", phase, err)
	}
}

func TestUpgradeInstanceTemplateTargetsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name                string
		definition, version *string
		missing             bool
		want                bool
	}{
		{name: "exact pair", definition: pointer.String("pg-canary"), version: pointer.String("18.0.0"), want: true},
		{name: "definition only", definition: pointer.String("pg-canary"), want: true},
		{name: "version only", version: pointer.String("18.0.0"), want: true},
		{name: "owner resolved empty pair", definition: pointer.String(""), version: pointer.String(""), want: true},
		{name: "replaced definition", definition: pointer.String("pg-new")},
		{name: "replaced version", version: pointer.String("19.0.0")},
		{name: "removed template", definition: pointer.String("pg-canary"), missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			component := appsv1.ClusterComponentSpec{Name: "pg", Instances: []appsv1.InstanceTemplate{{Name: "canary", CompDef: "pg-canary", ServiceVersion: "18.0.0"}, {Name: "unrelated"}}}
			if tc.missing {
				component.Instances = component.Instances[1:]
			}
			res := &OpsResource{Cluster: &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}}},
				OpsRequest: &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
					Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "pg"},
						Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ComponentDefinitionName: tc.definition, ServiceVersion: tc.version}},
					}}},
				}}}}
			if got := (upgradeOpsHandler{}).targetsUnchanged(res); got != tc.want {
				t.Fatalf("targets unchanged = %t, want %t", got, tc.want)
			}
			res.Cluster.Spec.ComponentSpecs = nil
			res.Cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "pg", Template: component}}
			if got := (upgradeOpsHandler{}).targetsUnchanged(res); got != tc.want {
				t.Fatalf("sharding targets unchanged = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestUpgradeInstanceTemplateSnapshotDoesNotAliasCluster(t *testing.T) {
	res := &OpsResource{Cluster: &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "pg",
		Instances: []appsv1.InstanceTemplate{{Name: "canary", CompDef: "pg-old", Labels: map[string]string{"test": "keep"}}},
	}}}}, OpsRequest: &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
		Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "pg"},
			Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ComponentDefinitionName: pointer.String("pg-new")}},
		}}},
	}}}}
	if err := (upgradeOpsHandler{}).SaveLastConfiguration(intctrlutil.RequestCtx{}, nil, res); err != nil {
		t.Fatal(err)
	}
	res.Cluster.Spec.ComponentSpecs[0].Instances[0].CompDef = "pg-new"
	res.Cluster.Spec.ComponentSpecs[0].Instances[0].Labels["test"] = "changed"
	last := res.OpsRequest.Status.LastConfiguration.Components["pg"].InstanceTemplates[0]
	if last.CompDef != "pg-old" || last.Labels["test"] != "keep" {
		t.Fatalf("snapshot changed with Cluster = %#v", last)
	}
}

func TestUpgradeRevalidatesInstanceTemplatesBeforeAction(t *testing.T) {
	for _, change := range []string{"remove template", "remove component", "override shard instances"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			component := appsv1.ClusterComponentSpec{Name: "pg", ComponentDef: "pg-old", Instances: []appsv1.InstanceTemplate{{Name: "canary", CompDef: "pg-old"}}}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
				Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}},
				Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
			if change == "override shard instances" {
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "pg", Shards: 1, Template: component}}
			}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{ClusterName: "demo", Type: opsv1alpha1.UpgradeType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
						ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "pg"}, Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ComponentDefinitionName: pointer.String("pg-new")}},
					}}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(cluster, ops).Build()
			res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(20)}
			if _, err := GetOpsManager().Do(intctrlutil.RequestCtx{Ctx: ctx}, cli, res); err != nil {
				t.Fatal(err)
			}
			if res.OpsRequest.Status.Phase != opsv1alpha1.OpsCreatingPhase {
				t.Fatalf("phase = %s, want Creating", res.OpsRequest.Status.Phase)
			}
			switch change {
			case "remove template":
				cluster.Spec.ComponentSpecs[0].Instances = nil
			case "remove component":
				cluster.Spec.ComponentSpecs = nil
			case "override shard instances":
				cluster.Spec.Shardings[0].ShardTemplates = []appsv1.ShardTemplate{{Name: "independent", Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}}
			}
			if err := cli.Update(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			want := cluster.Spec.DeepCopy()
			res = &OpsResource{Cluster: &appsv1.Cluster{}, OpsRequest: &opsv1alpha1.OpsRequest{}, Recorder: record.NewFakeRecorder(20)}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), res.Cluster); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if _, err := GetOpsManager().Do(intctrlutil.RequestCtx{Ctx: ctx}, cli, res); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
				t.Fatal(err)
			}
			if ops.Status.Phase != opsv1alpha1.OpsFailedPhase || !reflect.DeepEqual(&cluster.Spec, want) {
				t.Fatalf("phase = %s, Cluster spec changed = %t", ops.Status.Phase, !reflect.DeepEqual(&cluster.Spec, want))
			}
		})
	}
}

func TestUpgradeAllowsLaterUnrelatedClusterGeneration(t *testing.T) {
	const (
		namespace      = "default"
		clusterName    = "cluster"
		component      = "mysql"
		componentDef   = "mysql-8.0"
		serviceVersion = "8.0.36"
		instanceName   = "cluster-mysql-0"
	)
	replicas := int32(1)
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 9},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: component, ComponentDef: componentDef, ServiceVersion: serviceVersion, Replicas: replicas,
			}},
			Services: []appsv1.ClusterService{{Service: appsv1.Service{Name: "unrelated"}}},
		},
		Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
			component: {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 9, UpToDate: true},
		}},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "upgrade"},
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
				ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: component},
				ComponentDefinitionName: pointer.String(componentDef),
				ServiceVersion:          pointer.String(serviceVersion),
			}}},
		}},
		Status: opsv1alpha1.OpsRequestStatus{
			ClusterGeneration: 8,
			Components:        map[string]opsv1alpha1.OpsRequestComponentStatus{},
		},
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, component),
		},
		Spec: workloads.InstanceSetSpec{Replicas: &replicas},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
			PodName:      instanceName,
			DesiredState: workloads.InstanceDesiredStateActive,
			CurrentState: workloads.InstanceCurrentStatePresent,
			UpToDate:     true,
			Ready:        true,
			Available:    true,
		}}},
	}
	testScheme := runtime.NewScheme()
	for name, addToScheme := range map[string]func(*runtime.Scheme) error{
		"apps":       appsv1.AddToScheme,
		"operations": opsv1alpha1.AddToScheme,
		"workloads":  workloads.AddToScheme,
	} {
		if err := addToScheme(testScheme); err != nil {
			t.Fatalf("add %s scheme: %v", name, err)
		}
	}
	cli := fake.NewClientBuilder().WithScheme(testScheme).
		WithStatusSubresource(&opsv1alpha1.OpsRequest{}).
		WithObjects(opsRequest, its).Build()
	opsRes := &OpsResource{
		Cluster:    cluster,
		OpsRequest: opsRequest,
		Recorder:   record.NewFakeRecorder(10),
	}
	var err error
	opsRes.Runtimes, err = buildOpsRuntimes(context.Background(), cli, opsRes)
	if err != nil {
		t.Fatalf("build runtimes: %v", err)
	}

	phase, _, err := (upgradeOpsHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes)
	if err != nil {
		t.Fatalf("reconcile upgrade: %v", err)
	}
	if phase != opsv1alpha1.OpsSucceedPhase {
		t.Fatalf("phase=%s, want Succeed", phase)
	}
	if opsRequest.Status.Progress != "1/1" {
		t.Fatalf("progress=%s, want 1/1", opsRequest.Status.Progress)
	}
}

var _ = Describe("Upgrade OpsRequest", func() {

	var (
		randomStr   = testCtx.GetRandomStr()
		clusterName = "cluster-for-ops-" + randomStr
		serviceVer0 = testapps.ServiceVersion("v0")
		serviceVer1 = testapps.ServiceVersion("v1")
		serviceVer2 = testapps.ServiceVersion("v2")
		release0    = testapps.ReleaseID("r0")
		release1    = testapps.ReleaseID("r1")
		release2    = testapps.ReleaseID("r2")
		release3    = testapps.ReleaseID("r3")
		release4    = testapps.ReleaseID("r4")
	)
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	makeUpgradeOpsIsRunning := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
		By("mock upgrade OpsRequest phase is Running")
		_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		// do upgrade
		_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		targetComponents := opsRes.OpsRequest.Status.Components
		mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
		opsRes.OpsRequest.Status.ClusterGeneration = opsRes.Cluster.Generation
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(ops *opsv1alpha1.OpsRequest) {
			ops.Status.Phase = opsv1alpha1.OpsRunningPhase
			ops.Status.ClusterGeneration = opsRes.Cluster.Generation
			ops.Status.Components = targetComponents
		})).Should(Succeed())
	}

	mockClusterRunning := func(clusterObject *appsv1.Cluster) {
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1.RunningClusterPhase
			clusterObject.Status.Components = map[string]appsv1.ClusterComponentStatus{
				defaultCompName: {
					Phase: appsv1.RunningComponentPhase,
				},
			}
		})).Should(Succeed())
	}

	initOpsResWithComponentDef := func(createCompVersion bool) (*appsv1.ComponentDefinition, *appsv1.ComponentDefinition, *OpsResource) {
		compDef1 := testapps.NewComponentDefinitionFactory(testapps.CompDefName("cmpd-1")).
			SetServiceVersion(testapps.ServiceVersion(serviceVer0)).
			SetRuntime(&corev1.Container{
				Name: testapps.DefaultMySQLContainerName, Image: testapps.AppImage(testapps.AppName, release0),
			}).Create(&testCtx).GetObject()

		compDef2 := testapps.NewComponentDefinitionFactory(testapps.CompDefName("cmpd-2")).
			SetServiceVersion(testapps.ServiceVersion(serviceVer2)).SetRuntime(&corev1.Container{
			Name: testapps.DefaultMySQLContainerName, Image: testapps.AppImage(testapps.AppName, release2),
		}).Create(&testCtx).GetObject()
		if createCompVersion {
			compVersion := testapps.NewComponentVersionFactory(testapps.CompVersionName).
				SetSpec(appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{
							// use prefix
							CompDefs: []string{compDef1.Name},
							Releases: []string{release0, release1, release4}, // sv: v0, v1
						},
						{
							// use prefix
							CompDefs: []string{compDef2.Name},
							Releases: []string{release2, release3}, // sv: v2
						},
					},
					Releases: []appsv1.ComponentVersionRelease{
						{
							Name:           release0,
							Changes:        "init release",
							ServiceVersion: serviceVer0,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release0),
							},
						},
						{
							Name:           release1,
							Changes:        "update app image",
							ServiceVersion: serviceVer1,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release1),
							},
						},
						{
							Name:           release2,
							Changes:        "update app image",
							ServiceVersion: serviceVer2,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, serviceVer2),
							},
						},
						{
							Name:           release3,
							Changes:        "publish a new service version",
							ServiceVersion: serviceVer2,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release3),
							},
						},
						{
							Name:           release4,
							Changes:        "update all app images for previous service version",
							ServiceVersion: serviceVer1,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release4),
							},
						},
					},
				}).
				Create(&testCtx).
				GetObject()
			// label the componentDef info to the ComponentVersion
			Expect(testapps.ChangeObj(&testCtx, compVersion, func(version *appsv1.ComponentVersion) {
				if version.Labels == nil {
					version.Labels = map[string]string{}
				}
				version.Labels[compDef1.Name] = compDef1.Name
				version.Labels[compDef2.Name] = compDef2.Name
			})).Should(Succeed())

			// mock ComponentVersion to Available
			Expect(testapps.ChangeObjStatus(&testCtx, compVersion, func() {
				compVersion.Status.Phase = appsv1.AvailablePhase
				compVersion.Status.ObservedGeneration = compVersion.Generation
			})).Should(Succeed())
		}
		// create the cluster with no clusterDefinition
		clusterObject := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			AddComponent(defaultCompName, compDef1.Name).
			SetServiceVersion(testapps.ServiceVersion("v0")).
			SetReplicas(int32(3)).Create(&testCtx).GetObject()
		opsRes := &OpsResource{
			Cluster:  clusterObject,
			Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
		}
		// mock component phase to running
		mockClusterRunning(clusterObject)
		return compDef1, compDef2, opsRes
	}

	createUpgradeOpsRequest := func(clusterObject *appsv1.Cluster, upgradeSpec opsv1alpha1.Upgrade) *opsv1alpha1.OpsRequest {
		ops := testops.NewOpsRequestObj("upgrade-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterObject.Name, opsv1alpha1.UpgradeType)
		ops.Spec.Upgrade = &upgradeSpec
		opsRequest := testops.CreateOpsRequest(ctx, testCtx, ops)
		// set ops phase to Pending
		opsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
		return opsRequest
	}

	expectOpsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, compNames ...string) {
		for _, name := range compNames {
			its := &workloads.InstanceSet{}
			key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
				Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, name)}
			if err := k8sClient.Get(ctx, key, its); apierrors.IsNotFound(err) {
				its = testapps.MockInstanceSetComponent(&testCtx, opsRes.Cluster.Name, name)
			} else {
				Expect(err).ShouldNot(HaveOccurred())
			}
			testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, name)
		}
		mockRunningInstanceStatus(opsRes.Cluster, compNames...)
		// mock component to running
		mockRollingTargetStatus(opsRes.Cluster, appsv1.RunningComponentPhase, compNames...)
		_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))
	}

	Context("Test OpsRequest", func() {
		It("Test upgrade OpsRequest with ComponentDef and no ComponentVersion", func() {
			By("init operations resources ")
			compDef1, compDef2, opsRes := initOpsResWithComponentDef(false)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("mock upgrade OpsRequest phase is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.LastConfiguration.Components[defaultCompName].ComponentDefinitionName).Should(Equal(compDef1.Name))
			})).Should(Succeed())

			By("the ops succeeds when the current target status is Running and UpToDate")
			Expect(opsRes.OpsRequest.Status.ClusterGeneration).Should(Equal(opsRes.Cluster.Generation))
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest with ComponentDef and ComponentVersion", func() {
			By("init operations resources")
			_, compDef2, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion:          pointer.String(serviceVer2),
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef2.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer2))
			})).Should(Succeed())

			By("expect upgrade successfully")
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest without ComponentDefinitionName but the cluster is created without clusterDefinition", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(""),
						ServiceVersion:          pointer.String(serviceVer1),
					},
				},
			})

			By("expect for this opsRequest is Running and reuse the original ComponentDefinition")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer1))
			})).Should(Succeed())

			By("expect upgrade successfully with the latest release of the specified serviceVersion")
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest when specified serviceVersion is empty", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)
			generationBeforeUpgrade := opsRes.Cluster.Generation
			Expect(opsRes.Cluster.Spec.ComponentSpecs[0].ServiceVersion).ShouldNot(BeEmpty())

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(compDef1.Name),
						ServiceVersion:          pointer.String(""),
					},
				},
			})

			By(" expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(BeEmpty())
				g.Expect(cluster.Generation).Should(BeNumerically(">", generationBeforeUpgrade))
			})).Should(Succeed())

			By("looking forward to using the latest serviceVersion and releaseVersion")
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest when serviceVersion is nil", func() {
			By("init operations resources")
			_, compDef2, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(compDef2.Name),
					},
				},
			})

			By("expecting no changes to serviceVersion")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef2.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer0))
			})).Should(Succeed())
		})

		It("Test upgrade OpsRequest with invalid service version should fail validation", func() {
			By("init operations resources")
			_, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops with invalid service version")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:   opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion: pointer.String("17.5"),
					},
				},
			})

			By("expect Do() fails with validation error and OpsRequest enters Failed phase")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("Test upgrade OpsRequest when componentDefinitionName is nil", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:   opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion: pointer.String(""),
					},
				},
			})

			By("expecting no changes to cluster.spec.componentSpec[0].componentDef")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(""))
			})).Should(Succeed())
		})
		// TODO: add case with ClusterDefinition and topology
	})
})

func TestUpgradeInstanceTemplateOwnerConvergence(t *testing.T) {
	for _, tc := range []struct {
		name                                       string
		clear, shard, sidecar, rewritten, deferred bool
	}{
		{name: "init-only"},
		{name: "clear-both", clear: true},
		{name: "shard init-only", shard: true},
		{name: "shard clear-both", shard: true, clear: true},
		{name: "running init sidecar", sidecar: true},
		{name: "admission registry rewrite", rewritten: true},
		{name: "deferred tools init", deferred: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			def := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "db"}, Spec: appsv1.ComponentDefinitionSpec{
				ServiceVersion: "1.0.0", Runtime: corev1.PodSpec{
					Containers:     []corev1.Container{{Name: "db", Image: "db:same"}},
					InitContainers: []corev1.Container{{Name: "setup", Image: "setup:1"}},
				},
			}}
			if tc.sidecar {
				def.Spec.Runtime.InitContainers[0].RestartPolicy = ptr.To(corev1.ContainerRestartPolicyAlways)
			}
			if tc.deferred {
				oldToolsImage := viper.GetString(constant.KBToolsImage)
				defer viper.Set(constant.KBToolsImage, oldToolsImage)
				viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.1.0")
				def.Spec.Runtime.InitContainers[0] = corev1.Container{Name: kbagent.InitContainerName, Image: "docker.io/apecloud/kubeblocks-tools:1.0.0", Command: kbagent.InitCommand()}
				def.Spec.Runtime.Containers = append(def.Spec.Runtime.Containers, corev1.Container{Name: "kbagent", Image: "custom-action:1", Command: []string{kbagent.SharedBinaryPath}})
			}
			initName, targetInit := def.Spec.Runtime.InitContainers[0].Name, "setup:2"
			if tc.deferred {
				targetInit = "docker.io/apecloud/kubeblocks-tools:1.1.0"
			}
			ver := &appsv1.ComponentVersion{ObjectMeta: metav1.ObjectMeta{Name: "db-ver", Generation: 1, Labels: map[string]string{"db": "db"}}, Spec: appsv1.ComponentVersionSpec{
				CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{{CompDefs: []string{"db"}, Releases: []string{"v2"}}},
				Releases:           []appsv1.ComponentVersionRelease{{Name: "v2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:same", initName: targetInit, "kbagent": "custom-action:1"}}},
			}, Status: appsv1.ComponentVersionStatus{ObservedGeneration: 1, Phase: appsv1.AvailablePhase}}
			spec := appsv1.ClusterComponentSpec{Name: "db", ComponentDef: "db", ServiceVersion: "1.0.0", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1)), CompDef: "db", ServiceVersion: "1.0.0"}}}
			if tc.clear {
				spec.Instances[0].ServiceVersion = "2.0.0"
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 2}, Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{spec}}, Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 1, UpToDate: false}}}}
			targetName, componentName := "db", "db"
			if tc.shard {
				targetName, componentName = "shards", "shards-abc"
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: targetName, Shards: 1, Template: spec}}
				cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{targetName: {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 1, UpToDate: false}}
			}
			target := opsv1alpha1.UpgradeInstanceTemplate{Name: "canary", ServiceVersion: ptr.To("2.0.0")}
			if tc.clear {
				target.ComponentDefinitionName, target.ServiceVersion = ptr.To(""), ptr.To("")
			}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "default"}, Spec: opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.UpgradeType, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: targetName}, Instances: []opsv1alpha1.UpgradeInstanceTemplate{target}}}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.Now(), ClusterGeneration: 2}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, cluster, &workloads.InstanceSet{}, &corev1.Pod{}).WithObjects(def, ver, cluster, ops).Build()
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			resource := func() *OpsResource {
				return &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)}
			}
			handler := GetOpsManager().OpsMap[opsv1alpha1.UpgradeType].OpsHandler
			if err := handler.Action(reqCtx, cli, resource()); err != nil {
				t.Fatal(err)
			}
			spec = *getComponentSpecOrShardingTemplate(cluster, targetName).DeepCopy()
			if tc.clear && (spec.Instances[0].CompDef != "" || spec.Instances[0].ServiceVersion != "") {
				t.Fatalf("Action did not clear overrides: %#v", spec.Instances[0])
			}
			spec.Name = componentName
			comp, err := component.BuildComponent(cluster, &spec, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			comp.Generation = 2
			if tc.shard {
				comp.Labels[constant.KBAppShardingNameLabelKey] = targetName
			}
			if err := cli.Create(ctx, comp); err != nil {
				t.Fatal(err)
			}
			images := map[string]map[string]string{}
			if !tc.clear {
				images["canary"], err = component.ResolveInstanceTemplateImages4ServiceVersion(ctx, cli, def, "2.0.0")
				if err != nil {
					t.Fatal(err)
				}
				if !intctrlutil.EqualContainerImageInSpec(images["canary"][initName], targetInit) {
					t.Fatalf("resolved init image=%#v", images)
				}
			}
			synth := &component.SynthesizedComponent{Namespace: cluster.Namespace, ClusterName: cluster.Name, Name: componentName, CompDefName: "db", ServiceVersion: "1.0.0", Generation: "2", Replicas: 1, PodSpec: def.Spec.Runtime.DeepCopy(), Instances: spec.Instances, InstanceImages: images}
			its, err := component.BuildInstanceSet(synth, def)
			if err != nil {
				t.Fatal(err)
			}
			its.Generation = 2
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			for _, reconciler := range []kubebuilderx.Reconciler{instanceset.NewRevisionUpdateReconciler(), instanceset.NewReplicasAlignmentReconciler()} {
				if _, err := reconciler.Reconcile(tree); err != nil {
					t.Fatal(err)
				}
			}
			pods := tree.List(&corev1.Pod{})
			if len(pods) != 1 {
				t.Fatalf("built %d Pods", len(pods))
			}
			pod := pods[0].(*corev1.Pod)
			desiredInit := pod.Spec.InitContainers[0].Image
			pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Hour))
			pod.UID = "existing-pod"
			pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: pod.CreationTimestamp}}}
			for _, container := range pod.Spec.Containers {
				pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{Name: container.Name, Image: container.Image, ImageID: "containerd://sha256:1234", Ready: true})
			}
			pod.Spec.InitContainers[0].Image = "setup:1"
			if tc.clear {
				pod.Spec.InitContainers[0].Image = "setup:2"
			}
			if tc.deferred {
				pod.Spec.InitContainers[0].Image = "docker.io/apecloud/kubeblocks-tools:1.0.0"
				pod.Spec.InitContainers[0].Command = kbagent.LegacyInitCommand()
				pod.Spec.Containers[0].Image = "db:old"
				pod.Status.ContainerStatuses[0].Image = "db:old"
			}
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: initName, Image: pod.Spec.InitContainers[0].Image, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
			if tc.sidecar {
				pod.Status.InitContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}}
			}
			publish := func() {
				t.Helper()
				tree.SetRoot(its)
				if err := tree.Update(pod); err != nil {
					t.Fatal(err)
				}
				if _, err := instanceset.NewStatusReconciler().Reconcile(tree); err != nil {
					t.Fatal(err)
				}
			}
			publish()
			if status := its.FindInstanceStatus(pod.Name); status == nil || status.UpToDate {
				t.Fatalf("old init unexpectedly applied: %#v", status)
			}
			for _, object := range []client.Object{its, pod} {
				if err := cli.Create(ctx, object); err != nil {
					t.Fatal(err)
				}
			}
			reconcile := func(want opsv1alpha1.OpsPhase, progress string) {
				t.Helper()
				if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
					t.Fatal(err)
				}
				phase, requeue, err := GetOpsManager().OpsMap[opsv1alpha1.UpgradeType].OpsHandler.ReconcileAction(reqCtx, cli, resource())
				if err != nil || phase != want || ops.Status.Progress != progress {
					t.Fatalf("phase=%s progress=%s requeue=%s err=%v, want %s %s", phase, ops.Status.Progress, requeue, err, want, progress)
				}
			}
			reconcile(opsv1alpha1.OpsRunningPhase, "0/1")
			reconcile(opsv1alpha1.OpsRunningPhase, "0/1")
			pod.Spec.InitContainers[0].Image = desiredInit
			if tc.rewritten {
				pod.Spec.InitContainers[0].Image = "admission.local/tenant/" + desiredInit
			}
			if tc.deferred {
				pod.Spec.InitContainers[0].Image = "docker.io/apecloud/kubeblocks-tools:1.0.0"
				pod.Spec.Containers[0].Image = "db:same"
				pod.Status.ContainerStatuses[0].Image = "db:same"
			}
			pod.Status.InitContainerStatuses[0].Image = pod.Spec.InitContainers[0].Image
			podStatus := *pod.Status.DeepCopy()
			if err := cli.Update(ctx, pod); err != nil {
				t.Fatal(err)
			}
			pod.Status = podStatus
			if err := cli.Status().Update(ctx, pod); err != nil {
				t.Fatal(err)
			}
			publish()
			if status := its.FindInstanceStatus(pod.Name); status == nil || !status.UpToDate {
				t.Fatalf("updated Pod not applied by owner: %#v", status)
			}
			currentStatus := *its.Status.DeepCopy()
			for _, state := range []string{"missing", "stale", "not applied", "offline", "terminating"} {
				its.Status = *currentStatus.DeepCopy()
				switch state {
				case "missing":
					its.Status.InstanceStatus = nil
				case "stale":
					its.Status.ObservedGeneration--
				case "not applied":
					its.Status.InstanceStatus[0].UpToDate = false
				case "offline":
					its.Status.InstanceStatus[0].DesiredState = workloads.InstanceDesiredStateOffline
					its.Status.InstanceStatus[0].UpToDate = false
				case "terminating":
					its.Status.InstanceStatus[0].CurrentState = workloads.InstanceCurrentStateTerminating
					its.Status.InstanceStatus[0].UpToDate = false
				}
				if err := cli.Status().Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				progress := "0/1"
				if state == "stale" {
					progress = "1/1"
				}
				reconcile(opsv1alpha1.OpsRunningPhase, progress)
			}
			its.Status = currentStatus
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			reconcile(opsv1alpha1.OpsRunningPhase, "1/1")
			if tc.shard {
				status := cluster.Status.Shardings[targetName]
				status.ObservedGeneration, status.UpToDate = cluster.Generation, true
				cluster.Status.Shardings[targetName] = status
			} else {
				status := cluster.Status.Components[targetName]
				status.ObservedGeneration, status.UpToDate = cluster.Generation, true
				cluster.Status.Components[targetName] = status
			}
			if err := cli.Status().Update(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			reconcile(opsv1alpha1.OpsSucceedPhase, "1/1")
		})
	}
}
