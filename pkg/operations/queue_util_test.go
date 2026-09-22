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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

func TestStopAndVolumeExpansionPreserveQueueOrder(t *testing.T) {
	scheme := queueStopTestScheme(t)
	cluster := queueStopTestCluster()
	ve := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "volume-expansion", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.VolumeExpansionType},
	}
	stop := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	opsutil.SetOpsRequestToCluster(cluster, []opsv1alpha1.OpsRecorder{{
		Name: ve.Name, Type: ve.Spec.Type, QueueBySelf: true,
	}})
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

	recorder, err := enqueueOpsRequestToClusterAnnotation(context.Background(), cli, &OpsResource{Cluster: cluster, OpsRequest: stop}, OpsBehaviour{QueueByCluster: true})
	if err != nil {
		t.Fatalf("enqueue Stop after running VolumeExpansion: %v", err)
	}
	if !recorder.InQueue {
		t.Fatal("Stop bypassed VolumeExpansion")
	}
	queue, err := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(queue) != 2 || queue[0].Name != ve.Name || queue[1].Name != stop.Name {
		t.Fatalf("queue=%v, want VolumeExpansion before Stop", queue)
	}
}

func TestVolumeExpansionWaitsForStopButOtherQueueBySelfOpsRemainUnchanged(t *testing.T) {
	scheme := queueStopTestScheme(t)
	cluster := queueStopTestCluster()
	stop := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	ve := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "volume-expansion", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.VolumeExpansionType},
	}
	opsutil.SetOpsRequestToCluster(cluster, []opsv1alpha1.OpsRecorder{{
		Name: stop.Name, Type: stop.Spec.Type,
	}})
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

	veRecorder, err := enqueueOpsRequestToClusterAnnotation(context.Background(), cli, &OpsResource{Cluster: cluster, OpsRequest: ve}, GetOpsManager().OpsMap[opsv1alpha1.VolumeExpansionType])
	if err != nil {
		t.Fatalf("enqueue VolumeExpansion after Stop: %v", err)
	}
	if !veRecorder.InQueue {
		t.Fatal("VolumeExpansion bypassed Stop")
	}

	other := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "expose", Namespace: cluster.Namespace},
		Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.ExposeType},
	}
	otherRecorder, err := enqueueOpsRequestToClusterAnnotation(context.Background(), cli, &OpsResource{Cluster: cluster, OpsRequest: other}, OpsBehaviour{QueueBySelf: true})
	if err != nil {
		t.Fatalf("enqueue other QueueBySelf operation after Stop: %v", err)
	}
	if otherRecorder.InQueue {
		t.Fatal("Stop changed the queue relationship for other QueueBySelf operations")
	}
}

func TestStopQueueConflictIsScopedToVolumeExpansion(t *testing.T) {
	ve := opsv1alpha1.OpsRecorder{Name: "volume-expansion", Type: opsv1alpha1.VolumeExpansionType}
	other := opsv1alpha1.OpsRecorder{Name: "expose", Type: opsv1alpha1.ExposeType, QueueBySelf: true}
	stop := opsv1alpha1.OpsRecorder{Name: "stop", Type: opsv1alpha1.StopType}

	if !hasStopQueueConflict([]opsv1alpha1.OpsRecorder{ve}, 1, opsv1alpha1.StopType, false) {
		t.Fatal("Stop did not conflict with a preceding VolumeExpansion")
	}
	if hasStopQueueConflict([]opsv1alpha1.OpsRecorder{other}, 1, opsv1alpha1.StopType, false) {
		t.Fatal("Stop unexpectedly conflicted with another QueueBySelf operation")
	}
	if !hasStopQueueConflict([]opsv1alpha1.OpsRecorder{stop}, 1, opsv1alpha1.VolumeExpansionType, true) {
		t.Fatal("VolumeExpansion did not conflict with a preceding Stop")
	}
	if hasStopQueueConflict([]opsv1alpha1.OpsRecorder{stop}, 1, opsv1alpha1.ExposeType, false) {
		t.Fatal("another operation unexpectedly conflicted with Stop")
	}
}

func queueStopTestScheme(t *testing.T) *runtime.Scheme {
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

func queueStopTestCluster() *appsv1.Cluster {
	return &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"}}
}
