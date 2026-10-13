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
	"testing"
	"time"

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
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestUpgradeInstanceTemplateVersions(t *testing.T) {
	for _, tc := range []struct {
		name, version, templateImage   string
		flat, shard, wrongStatus       bool
		unlabeled, alternateDefinition bool
		want                           opsv1alpha1.OpsPhase
		progress                       string
	}{
		{name: "fixed template", version: "1.0.0", templateImage: "db:v1", want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "wrong template image", version: "1.0.0", templateImage: "db:wrong", want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "old running image", version: "1.0.0", templateImage: "db:v1", wrongStatus: true, want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "empty differential inherits default", version: "0.0.0", templateImage: "db:v2", want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "flat fixed template", version: "1.0.0", templateImage: "db:v1", flat: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "flat template without ownership label", version: "1.0.0", templateImage: "db:v1", flat: true, unlabeled: true, want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "shard instance template", version: "1.0.0", templateImage: "db:v1", shard: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "shard wrong image", version: "1.0.0", templateImage: "db:wrong", shard: true, want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "shard definition determines readiness", version: "1.0.0", templateImage: "db:v1", shard: true, alternateDefinition: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{
				corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme,
			} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			def := &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "db"},
				Spec: appsv1.ComponentDefinitionSpec{
					Runtime: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:base"}}},
				},
			}
			ver := &appsv1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Labels: map[string]string{"db": "db"}},
				Spec: appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{CompDefs: []string{"db"}, Releases: []string{"r0", "r1", "r2"}},
					},
					Releases: []appsv1.ComponentVersionRelease{
						{Name: "r0", ServiceVersion: "0.0.0", Images: map[string]string{"db": "db:base"}},
						{Name: "r1", ServiceVersion: "1.0.0", Images: map[string]string{"db": "db:v1"}},
						{Name: "r2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:v2"}},
					},
				},
				Status: appsv1.ComponentVersionStatus{Phase: appsv1.AvailablePhase},
			}
			spec := appsv1.ClusterComponentSpec{
				Name: "db", ComponentDef: "db", ServiceVersion: "2.0.0", Replicas: 2,
				FlatInstanceOrdinal: tc.flat,
				Instances:           []appsv1.InstanceTemplate{{Name: "fixed", CompDef: "db", ServiceVersion: tc.version}},
			}
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns"},
				Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{spec}},
				Status: appsv1.ClusterStatus{
					Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}},
				},
			}
			target, componentName := "db", "db"
			var shard *appsv1.Component
			if tc.shard {
				target, componentName = "shards", "shards-abc"
				defaultSpec := spec
				defaultSpec.Name = "shards"
				defaultSpec.Replicas = 3
				defaultSpec.Instances = nil
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{
					Name: target, Shards: 1, Template: defaultSpec,
					ShardTemplates: []appsv1.ShardTemplate{{
						Name: "special", Shards: ptr.To(int32(1)), Replicas: ptr.To(int32(2)), Instances: spec.Instances,
					}},
				}}
				cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
					target: {Phase: appsv1.RunningComponentPhase},
				}
				labels := constant.GetCompLabels(cluster.Name, componentName)
				labels[constant.KBAppShardingNameLabelKey] = target
				labels[constant.KBAppShardTemplateLabelKey] = "special"
				shard = &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-" + componentName, Namespace: "ns", Labels: labels,
				}}
			}
			ops := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "ns"},
				Spec: opsv1alpha1.OpsRequestSpec{
					Type: opsv1alpha1.UpgradeType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
						Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
							ComponentOps: opsv1alpha1.ComponentOps{ComponentName: target},
						}}},
					},
				},
				Status: opsv1alpha1.OpsRequestStatus{
					Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.NewTime(time.Now()),
				},
			}
			pod := func(name, image string) *corev1.Pod {
				return &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: name, Namespace: "ns", Labels: constant.GetCompLabels("cluster", componentName),
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: image}}},
					Status: corev1.PodStatus{
						Phase:             corev1.PodRunning,
						Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
						ContainerStatuses: []corev1.ContainerStatus{{Name: "db", Image: image, Ready: true}},
					},
				}
			}
			base := pod("cluster-"+componentName+"-0", "db:v2")
			base.Labels[constant.KBAppInstanceTemplateLabelKey] = ""
			fixed := pod("cluster-"+componentName+"-fixed-0", tc.templateImage)
			if tc.flat {
				fixed.Name = "cluster-" + componentName + "-1"
				fixed.Labels[constant.KBAppInstanceTemplateLabelKey] = "fixed"
			}
			if tc.wrongStatus {
				fixed.Status.ContainerStatuses[0].Image = "db:old"
			}
			if tc.unlabeled {
				delete(fixed.Labels, constant.KBAppInstanceTemplateLabelKey)
			}
			objects := []client.Object{def, ver, cluster, ops, base, fixed}
			if tc.alternateDefinition {
				alternate := def.DeepCopy()
				alternate.Name = "alternate"
				def.Spec.Roles = []appsv1.ReplicaRole{{Name: "leader"}}
				ver.Labels[alternate.Name] = alternate.Name
				ver.Spec.CompatibilityRules[0].CompDefs = append(ver.Spec.CompatibilityRules[0].CompDefs, alternate.Name)
				cluster.Spec.Shardings[0].ShardTemplates[0].CompDef = ptr.To(alternate.Name)
				cluster.Spec.Shardings[0].ShardTemplates[0].ServiceVersion = ptr.To("2.0.0")
				objects = append(objects, alternate)
			}
			if shard != nil {
				objects = append(objects, shard)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(objects...).Build()
			phase, _, err := (upgradeOpsHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
				&OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)},
			)
			if err != nil {
				t.Fatal(err)
			}
			if phase != tc.want || ops.Status.Progress != tc.progress {
				t.Fatalf("phase=%s progress=%s, want %s %s", phase, ops.Status.Progress, tc.want, tc.progress)
			}
		})
	}
}
