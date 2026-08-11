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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestRollingTargetStatusGenerationFence(t *testing.T) {
	tests := []struct {
		name              string
		clusterGeneration int64
		opsGeneration     int64
		observed          int64
		upToDate          bool
		want              bool
	}{
		{name: "current", clusterGeneration: 8, opsGeneration: 8, observed: 8, upToDate: true, want: true},
		{name: "older than operation", clusterGeneration: 8, opsGeneration: 8, observed: 7, upToDate: true},
		{name: "cluster changed again", clusterGeneration: 9, opsGeneration: 8, observed: 8, upToDate: true},
		{name: "not up to date", clusterGeneration: 8, opsGeneration: 8, observed: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rollingTargetStatusIsCurrent(tt.clusterGeneration, tt.opsGeneration, tt.observed, tt.upToDate); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollingRevisionProgress(t *testing.T) {
	const (
		component = "mysql"
		pod0      = "cluster-mysql-0"
		pod1      = "cluster-mysql-1"
	)
	baseWorkload := func() *defaultWorkload {
		return &defaultWorkload{
			exists:             true,
			statusObserved:     true,
			minReadySeconds:    10,
			desiredReplicas:    2,
			currentReplicas:    2,
			currentRevisionMap: map[string]string{pod0: "new", pod1: "new"},
			updateRevisionMap:  map[string]string{pod0: "new", pod1: "new"},
			notReadySet:        sets.New[string](),
			notAvailableSet:    sets.New[string](),
			failedSet:          sets.New[string](),
			instanceNames:      sets.New(pod0, pod1),
		}
	}
	newResources := func() (*OpsResource, *progressResource, *opsv1alpha1.OpsRequestComponentStatus) {
		ops := &opsv1alpha1.OpsRequest{}
		ops.Status.Phase = opsv1alpha1.OpsRunningPhase
		return &OpsResource{
				OpsRequest: ops,
				Cluster: &appsv1.Cluster{Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
					component: {},
				}}},
				Recorder: record.NewFakeRecorder(20),
			}, &progressResource{
				opsMessageKey:     "upgrade",
				clusterComponent:  &appsv1.ClusterComponentSpec{Name: component, Replicas: 2},
				fullComponentName: component,
			}, &opsv1alpha1.OpsRequestComponentStatus{}
	}

	tests := []struct {
		name          string
		mutate        func(*defaultWorkload, *progressResource)
		wantExpected  int32
		wantCompleted int32
		wantPod0      opsv1alpha1.ProgressStatus
	}{
		{name: "all revisions ready and available", wantExpected: 2, wantCompleted: 2, wantPod0: opsv1alpha1.SucceedProgressStatus},
		{name: "revision mismatch", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.currentRevisionMap[pod0] = "old"
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "not ready", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.notReadySet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "not available", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.notAvailableSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "availability is ignored without minReadySeconds", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.minReadySeconds = 0
			w.notAvailableSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 2, wantPod0: opsv1alpha1.SucceedProgressStatus},
		{name: "old revision failure is ignored", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.currentRevisionMap[pod0] = "old"
			w.failedSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "target revision failure", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.failedSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 2, wantPod0: opsv1alpha1.FailedProgressStatus},
		{name: "target revision missing", mutate: func(w *defaultWorkload, pg *progressResource) {
			delete(w.updateRevisionMap, pod0)
			pg.updatedPodSet = map[string]string{pod0: "template"}
		}, wantExpected: 1, wantCompleted: 0, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "unobserved InstanceSet", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.statusObserved = false
		}, wantExpected: 2, wantCompleted: 0},
		{name: "missing InstanceSet", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.exists = false
		}, wantExpected: 2, wantCompleted: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workload := baseWorkload()
			opsRes, pgRes, compStatus := newResources()
			if tt.mutate != nil {
				tt.mutate(workload, pgRes)
			}
			expected, completed := handleRollingProgressByRevisionWithWorkload(opsRes, workload, pgRes, compStatus)
			if expected != tt.wantExpected || completed != tt.wantCompleted {
				t.Fatalf("progress %d/%d, want %d/%d", completed, expected, tt.wantCompleted, tt.wantExpected)
			}
			if tt.wantPod0 != "" {
				detail := findStatusProgressDetail(compStatus.ProgressDetails, getProgressObjectKey(constant.PodKind, pod0))
				if detail == nil || detail.Status != tt.wantPod0 {
					t.Fatalf("pod0 detail = %#v, want status %s", detail, tt.wantPod0)
				}
			}
		})
	}
}

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
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&opsv1alpha1.OpsRequest{}).WithObjects(cluster, ops).Build()
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "a"}, {ComponentName: "b"}})
		handler := func(_ intctrlutil.RequestCtx, _ client.Client, _ *OpsResource, _ *progressResource,
			_ *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
			return 1, 0, nil
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
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&opsv1alpha1.OpsRequest{}).WithObjects(cluster, ops).Build()
		helper := newComponentOpsHelper([]opsv1alpha1.UpgradeComponent{{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"},
		}})
		handler := func(_ intctrlutil.RequestCtx, _ client.Client, _ *OpsResource, _ *progressResource,
			_ *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
			return 1, 0, nil
		}
		phase, _, err := helper.reconcileRollingActionWithComponentOps(intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
			&OpsResource{Cluster: cluster, OpsRequest: ops}, "upgrade", handler)
		if err != nil || phase != opsv1alpha1.OpsFailedPhase {
			t.Fatalf("phase=%s err=%v, want Failed", phase, err)
		}
	})
}
