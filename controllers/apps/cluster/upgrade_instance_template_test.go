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

package cluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/operations"
)

func TestUpgradeSingleClearSnapshotNormalizationRace(t *testing.T) {
	for _, sharding := range []bool{false, true} {
		name := "component"
		if sharding {
			name = "sharding"
		}
		t.Run(name, func(t *testing.T) { testUpgradeSingleClearSnapshotNormalizationRace(t, sharding) })
	}
}

func testUpgradeSingleClearSnapshotNormalizationRace(t *testing.T, sharding bool) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	def := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "db"}, Spec: appsv1.ComponentDefinitionSpec{ServiceVersion: "1.0.0", Runtime: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:1"}}}}}
	ver := &appsv1.ComponentVersion{ObjectMeta: metav1.ObjectMeta{Name: "db-ver", Generation: 1, Labels: map[string]string{"db": "db"}}, Spec: appsv1.ComponentVersionSpec{CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{{CompDefs: []string{"db"}, Releases: []string{"v2"}}}, Releases: []appsv1.ComponentVersionRelease{{Name: "v2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:2"}}}}, Status: appsv1.ComponentVersionStatus{ObservedGeneration: 1, Phase: appsv1.AvailablePhase}}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1}, Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", ComponentDef: "db", ServiceVersion: "1.0.0", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}}}, Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
	if sharding {
		cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Shards: 1, Template: cluster.Spec.ComponentSpecs[0]}}
		cluster.Spec.ComponentSpecs = nil
	}
	spec := func() *appsv1.ClusterComponentSpec {
		if sharding {
			return &cluster.Spec.Shardings[0].Template
		}
		return &cluster.Spec.ComponentSpecs[0]
	}
	request := func(name, version string) *opsv1alpha1.OpsRequest {
		return &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Spec: opsv1alpha1.OpsRequestSpec{ClusterName: "demo", Type: opsv1alpha1.UpgradeType, Force: true, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To(version)}}}}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
	}
	first, second := request("first", "2.0.0"), request("second", "")
	runningComp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"}, Spec: appsv1.ComponentSpec{CompDef: "db", ServiceVersion: "1.0.0", Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cluster, first, second).WithObjects(def, ver, cluster, first, second, runningComp).Build()
	reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
	resource := func(ops *opsv1alpha1.OpsRequest) *operations.OpsResource {
		return &operations.OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)}
	}
	reload := func(ops *opsv1alpha1.OpsRequest) {
		t.Helper()
		if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
			t.Fatal(err)
		}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
			t.Fatal(err)
		}
	}
	do := func(ops *opsv1alpha1.OpsRequest) {
		t.Helper()
		result, err := operations.GetOpsManager().Do(reqCtx, cli, resource(ops))
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			ops.Status.Phase, ops.Status.ClusterGeneration = opsv1alpha1.OpsRunningPhase, cluster.Generation
			if err := cli.Status().Update(ctx, ops); err != nil {
				t.Fatal(err)
			}
		}
		reload(ops)
	}
	// Produce a partial pair through the first real Upgrade entry and Action.
	do(first)
	do(first)
	tpl := spec().Instances[0]
	if tpl.CompDef != "" || tpl.ServiceVersion != "2.0.0" {
		t.Fatalf("first Action pair = %#v", tpl)
	}
	// Second Pending entry saves the partial pair and transitions to Creating.
	do(second)
	if second.Status.Phase != opsv1alpha1.OpsCreatingPhase {
		t.Fatalf("second phase = %s", second.Status.Phase)
	}
	original := second.Status.LastConfiguration.Components["db"].InstanceTemplates[0]
	if original.CompDef != "" || original.ServiceVersion != "2.0.0" {
		t.Fatalf("snapshot = %#v", original)
	}
	normalize := func() {
		t.Helper()
		if _, err := (&clusterNormalizationTransformer{}).resolveDefinitions4ComponentWithObj(&clusterTransformContext{Context: ctx, Client: cli, Cluster: cluster}, spec(), runningComp); err != nil {
			t.Fatal(err)
		}
		if err := cli.Update(ctx, cluster); err != nil {
			t.Fatal(err)
		}
		reload(second)
	}
	// The existing Cluster owner fills the omitted definition before second Action.
	normalize()
	t.Logf("pair before second Action = %#v", spec().Instances[0])
	do(second)
	tpl = spec().Instances[0]
	if tpl.CompDef != "db" || tpl.ServiceVersion != "" {
		t.Fatalf("second Action must preserve current omitted definition: %#v", tpl)
	}
	normalize()
	t.Logf("pair after normal backfill = %#v", spec().Instances[0])
	if _, err := operations.GetOpsManager().Reconcile(reqCtx, cli, resource(second)); err != nil {
		t.Fatal(err)
	}
	reload(second)
	if got := second.Status.LastConfiguration.Components["db"].InstanceTemplates[0]; got.CompDef != "" || got.ServiceVersion != "2.0.0" {
		t.Fatalf("original snapshot changed: %#v", got)
	}
	if second.Status.Phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("aborted after only normal owner backfill; no subsequent writer replaced the Action target; completion=%s", second.Status.CompletionTimestamp)
	}
}
