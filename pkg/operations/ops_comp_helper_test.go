/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestRollingComponentAndShardingTerminalStatus(t *testing.T) {
	const (
		namespace   = "default"
		clusterName = "cluster"
	)
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	t.Run("all component targets must be current", func(t *testing.T) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 7},
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{
				{Name: "a", Replicas: 1}, {Name: "b", Replicas: 1},
			}},
			Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
				"a": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true},
				"b": {Phase: appsv1.FailedComponentPhase, ObservedGeneration: 6, UpToDate: true},
			}},
		}
		ops := &opsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "upgrade"},
			Status:     opsv1alpha1.OpsRequestStatus{ClusterGeneration: 7},
		}
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "a"}, {ComponentName: "b"}})
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&opsv1alpha1.OpsRequest{}).WithObjects(cluster, ops).Build()
		handler := func(_ intctrlutil.RequestCtx, _ client.Client, _ *OpsResource, pg *progressResource,
			_ *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
			return 1, 1, nil
		}
		phase, _, err := helper.reconcileRollingActionWithComponentOps(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, "upgrade", handler)
		if err != nil || phase != opsv1alpha1.OpsRunningPhase {
			t.Fatalf("phase=%s err=%v, want Running", phase, err)
		}
		status := cluster.Status.Components["b"]
		status.ObservedGeneration = 7
		status.Phase = appsv1.StoppedComponentPhase
		cluster.Status.Components["b"] = status
		phase, _, err = helper.reconcileRollingActionWithComponentOps(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, "upgrade", handler)
		if err != nil || phase != opsv1alpha1.OpsSucceedPhase {
			t.Fatalf("phase=%s err=%v, want Succeed", phase, err)
		}
		if ops.Status.Progress != "2/2" {
			t.Fatalf("progress=%s, want 2/2", ops.Status.Progress)
		}
	})

	t.Run("current sharding failure is terminal", func(t *testing.T) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 9},
			Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
				Name: "shard", Shards: 1, Template: appsv1.ClusterComponentSpec{Replicas: 1},
			}}},
			Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{
				"shard": {Phase: appsv1.FailedComponentPhase, ObservedGeneration: 9, UpToDate: true},
			}},
		}
		ops := &opsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "upgrade-shard"},
			Status:     opsv1alpha1.OpsRequestStatus{ClusterGeneration: 9},
		}
		helper := newComponentOpsHelper([]opsv1alpha1.UpgradeComponent{{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"},
		}})
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&opsv1alpha1.OpsRequest{}).WithObjects(cluster, ops).Build()
		handler := func(_ intctrlutil.RequestCtx, _ client.Client, _ *OpsResource, _ *progressResource,
			_ *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
			return 1, 0, nil
		}
		phase, _, err := helper.reconcileRollingActionWithComponentOps(intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
			opsRes, "upgrade", handler)
		if err != nil || phase != opsv1alpha1.OpsFailedPhase {
			t.Fatalf("phase=%s err=%v, want Failed", phase, err)
		}
	})

	t.Run("later generation is accepted after the Cluster converges", func(t *testing.T) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 8},
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql", Annotations: map[string]string{constant.RestartAnnotationKey: "restart-a"},
			}}},
			Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
				"mysql": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true},
			}},
		}
		ops := &opsv1alpha1.OpsRequest{Status: opsv1alpha1.OpsRequestStatus{ClusterGeneration: 8}}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}})
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		cluster.Generation = 9
		cluster.Spec.ComponentSpecs[0].Annotations[constant.RestartAnnotationKey] = "restart-b"
		cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{
			Phase: appsv1.UpdatingComponentPhase, ObservedGeneration: 9,
		}
		completed, failed := helper.rollingTargetsState(opsRes)
		if completed || failed {
			t.Fatalf("updating Cluster completed=%v failed=%v, want processing", completed, failed)
		}
		cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 9, UpToDate: true,
		}
		completed, failed = helper.rollingTargetsState(opsRes)
		if !completed || failed {
			t.Fatalf("converged Cluster completed=%v failed=%v, want success", completed, failed)
		}
	})

	t.Run("later generation waits for its current Cluster status", func(t *testing.T) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 8},
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql", Annotations: map[string]string{constant.RestartAnnotationKey: "restart-a"},
			}}},
			Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
				"mysql": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true},
			}},
		}
		ops := &opsv1alpha1.OpsRequest{Status: opsv1alpha1.OpsRequestStatus{ClusterGeneration: 8}}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}})
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}

		// A later Cluster change invalidates the previously observed target status.
		cluster.Generation = 9
		cluster.Spec.Services = []appsv1.ClusterService{{Service: appsv1.Service{Name: "external"}}}
		cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true,
		}
		completed, failed := helper.rollingTargetsState(opsRes)
		if completed || failed {
			t.Fatalf("stale status completed=%v failed=%v, want processing", completed, failed)
		}

		cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 9, UpToDate: true,
		}
		completed, failed = helper.rollingTargetsState(opsRes)
		if !completed || failed {
			t.Fatalf("compatible generation completed=%v failed=%v, want success", completed, failed)
		}
	})

	t.Run("failed status is ignored until the target is up-to-date", func(t *testing.T) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Generation: 5},
			Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}}},
			Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
				"mysql": {Phase: appsv1.FailedComponentPhase, ObservedGeneration: 5, UpToDate: false},
			}},
		}
		ops := &opsv1alpha1.OpsRequest{Status: opsv1alpha1.OpsRequestStatus{ClusterGeneration: 5}}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}})
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		completed, failed := helper.rollingTargetsState(opsRes)
		if completed || failed {
			t.Fatalf("completed=%v failed=%v, want processing for stale failure", completed, failed)
		}
		status := cluster.Status.Components["mysql"]
		status.UpToDate = true
		cluster.Status.Components["mysql"] = status
		completed, failed = helper.rollingTargetsState(opsRes)
		if completed || !failed {
			t.Fatalf("completed=%v failed=%v, want current Cluster failure", completed, failed)
		}
	})

	t.Run("instance-scoped progress is independent of aggregate component status", func(t *testing.T) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 5},
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql", Replicas: 1,
			}}},
			Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
				"mysql": {Phase: appsv1.FailedComponentPhase, ObservedGeneration: 5, UpToDate: true},
			}},
		}
		ops := &opsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "vertical-scaling"},
			Status:     opsv1alpha1.OpsRequestStatus{ClusterGeneration: 5},
		}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}})
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&opsv1alpha1.OpsRequest{}).WithObjects(cluster, ops).Build()
		participantStatus := opsv1alpha1.SucceedProgressStatus
		handler := func(_ intctrlutil.RequestCtx, _ client.Client, _ *OpsResource, pg *progressResource,
			compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
			const podName = "cluster-mysql-0"
			pg.updatedPodSet = map[string]string{podName: "template-a"}
			compStatus.ProgressDetails = []opsv1alpha1.ProgressStatusDetail{{
				ObjectKey: getProgressObjectKey(constant.PodKind, podName),
				Status:    participantStatus,
			}}
			return 1, 1, nil
		}
		phase, _, err := helper.reconcileRollingActionWithComponentOps(
			intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, "vertical scale", handler)
		if err != nil || phase != opsv1alpha1.OpsSucceedPhase {
			t.Fatalf("phase=%s err=%v, want participant success despite aggregate failure", phase, err)
		}

		participantStatus = opsv1alpha1.FailedProgressStatus
		phase, _, err = helper.reconcileRollingActionWithComponentOps(
			intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, "vertical scale", handler)
		if err != nil || phase != opsv1alpha1.OpsFailedPhase {
			t.Fatalf("phase=%s err=%v, want participant failure", phase, err)
		}
	})
}
