/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software; you can redistribute it and/or modify
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
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

func TestStopQueueBarrierBlocksForceAndQueueBySelf(t *testing.T) {
	scheme := queueBarrierScheme(t)
	cluster := queueBarrierCluster()
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	ctx := context.Background()

	clusterOps := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "running-cluster-op", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.RestartType},
	}
	if _, err := enqueueQueueBarrierRequest(ctx, cli, cluster, clusterOps, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("enqueue running cluster op: %v", err)
	}

	stop := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	if recorder, err := enqueueQueueBarrierRequest(ctx, cli, cluster, stop, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("enqueue stop: %v", err)
	} else if !recorder.InQueue {
		t.Fatal("Stop bypassed a running cluster-scoped operation")
	}

	force := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "force", Namespace: cluster.Namespace},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName: cluster.Name,
			Type:        opsv1alpha1.RestartType,
			Force:       true,
		},
	}
	if recorder, err := enqueueQueueBarrierRequest(ctx, cli, cluster, force, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("enqueue force op: %v", err)
	} else if !recorder.InQueue {
		t.Fatal("Force bypassed a queued Stop barrier")
	}
	start := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "start", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StartType},
	}
	if recorder, err := enqueueQueueBarrierRequest(ctx, cli, cluster, start, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("enqueue Start after Stop: %v", err)
	} else if !recorder.InQueue {
		t.Fatal("Start bypassed a queued Stop barrier")
	}

	queue, err := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(queue) != 4 || queue[0].Name != clusterOps.Name || queue[1].Name != stop.Name || queue[2].Name != force.Name || queue[3].Name != start.Name {
		t.Fatalf("queue=%v, want running, Stop, Force, Start", queue)
	}

	// QueueBySelf operations continue to coexist when there is no Stop barrier.
	cluster = queueBarrierCluster()
	cli = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	selfRunning := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "running-volume", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.VolumeExpansionType},
	}
	if _, err := enqueueQueueBarrierRequest(ctx, cli, cluster, selfRunning, OpsBehaviour{QueueBySelf: true}); err != nil {
		t.Fatalf("enqueue running QueueBySelf op: %v", err)
	}
	selfOtherType := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "running-expose", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.ExposeType},
	}
	if recorder, err := enqueueQueueBarrierRequest(ctx, cli, cluster, selfOtherType, OpsBehaviour{QueueBySelf: true}); err != nil {
		t.Fatalf("enqueue independent QueueBySelf op: %v", err)
	} else if recorder.InQueue {
		t.Fatal("QueueBySelf operations stopped coexisting with another type")
	}
	stop = &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop-self", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	if recorder, err := enqueueQueueBarrierRequest(ctx, cli, cluster, stop, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("enqueue Stop behind QueueBySelf op: %v", err)
	} else if !recorder.InQueue {
		t.Fatal("Stop bypassed a running QueueBySelf operation")
	}
	secondStop := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop-self-2", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	if recorder, err := enqueueQueueBarrierRequest(ctx, cli, cluster, secondStop, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("enqueue second Stop: %v", err)
	} else if !recorder.InQueue {
		t.Fatal("second Stop bypassed the first queued Stop")
	}
}

func TestStopQueueBarrierReleasesLaterOpsOnTerminalStates(t *testing.T) {
	for _, phase := range []opsv1alpha1.OpsPhase{
		opsv1alpha1.OpsFailedPhase,
		opsv1alpha1.OpsCancelledPhase,
		opsv1alpha1.OpsAbortedPhase, // includes timeout completion
	} {
		t.Run(string(phase), func(t *testing.T) {
			scheme := queueBarrierScheme(t)
			cluster := queueBarrierCluster()
			stop := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: cluster.Namespace},
				Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
				Status:     opsv1alpha1.OpsRequestStatus{Phase: phase},
			}
			later := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "later", Namespace: cluster.Namespace},
				Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.RestartType, Force: true},
				Status:     opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase},
			}
			opsutil.SetOpsRequestToCluster(cluster, []opsv1alpha1.OpsRecorder{
				{Name: stop.Name, Type: opsv1alpha1.StopType},
				{Name: later.Name, Type: later.Spec.Type, InQueue: true},
			})
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(stop, later).WithObjects(cluster, stop, later).Build()

			if err := DequeueOpsRequestInClusterAnnotation(context.Background(), cli, &OpsResource{Cluster: cluster, OpsRequest: stop}); err != nil {
				t.Fatalf("dequeue Stop in %s: %v", phase, err)
			}
			queue, err := opsutil.GetOpsRequestSliceFromCluster(cluster)
			if err != nil {
				t.Fatalf("read queue after %s Stop: %v", phase, err)
			}
			if len(queue) != 1 || queue[0].Name != later.Name || !queue[0].InQueue {
				t.Fatalf("queue after %s Stop=%v, want later request retained and queued", phase, queue)
			}

			if _, err := enqueueOpsRequestToClusterAnnotation(context.Background(), cli, &OpsResource{Cluster: cluster, OpsRequest: later}, OpsBehaviour{QueueByCluster: true}); err != nil {
				t.Fatalf("release later op after %s Stop: %v", phase, err)
			}
			queue, err = opsutil.GetOpsRequestSliceFromCluster(cluster)
			if err != nil {
				t.Fatalf("read promoted queue: %v", err)
			}
			if len(queue) != 1 || queue[0].Name != later.Name || queue[0].InQueue {
				t.Fatalf("promoted queue after %s Stop=%v, want later running", phase, queue)
			}
		})
	}
}

func TestStopQueueBarrierWaitsForRunningOpsBeforePromotion(t *testing.T) {
	scheme := queueBarrierScheme(t)
	cluster := queueBarrierCluster()
	running := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "running", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.RestartType},
	}
	stop := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	opsutil.SetOpsRequestToCluster(cluster, []opsv1alpha1.OpsRecorder{
		{Name: running.Name, Type: running.Spec.Type},
		{Name: stop.Name, Type: stop.Spec.Type, InQueue: true},
	})
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

	if err := DequeueOpsRequestInClusterAnnotation(context.Background(), cli, &OpsResource{
		Cluster:    cluster,
		OpsRequest: running,
	}); err != nil {
		t.Fatalf("dequeue running operation: %v", err)
	}
	if _, err := enqueueOpsRequestToClusterAnnotation(context.Background(), cli, &OpsResource{
		Cluster:    cluster,
		OpsRequest: stop,
	}, OpsBehaviour{QueueByCluster: true}); err != nil {
		t.Fatalf("promote Stop: %v", err)
	}
	queue, err := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if err != nil {
		t.Fatalf("read promoted Stop queue: %v", err)
	}
	if len(queue) != 1 || queue[0].Name != stop.Name || queue[0].InQueue {
		t.Fatalf("queue=%v, want Stop promoted after earlier operation leaves", queue)
	}
}

func enqueueQueueBarrierRequest(ctx context.Context, cli client.Client, cluster *appsv1.Cluster, ops *opsv1alpha1.OpsRequest, behaviour OpsBehaviour) (*opsv1alpha1.OpsRecorder, error) {
	return enqueueOpsRequestToClusterAnnotation(ctx, cli, &OpsResource{Cluster: cluster, OpsRequest: ops}, behaviour)
}

func queueBarrierScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func queueBarrierCluster() *appsv1.Cluster {
	return &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"}}
}
