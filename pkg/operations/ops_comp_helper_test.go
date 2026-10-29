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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("Component action phase", func() {
	It("waits for the current target phase after a recoverable failure", func() {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Generation: 8},
			Spec: appsv1.ClusterSpec{
				ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}},
				Shardings:      []appsv1.ClusterSharding{{Name: "shard"}},
			},
			Status: appsv1.ClusterStatus{
				Components: map[string]appsv1.ClusterComponentStatus{
					"mysql": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true},
				},
				Shardings: map[string]appsv1.ClusterShardingStatus{
					"shard": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true},
				},
			},
		}
		ops := &opsv1alpha1.OpsRequest{Status: opsv1alpha1.OpsRequestStatus{
			ClusterGeneration: 8,
			Components:        map[string]opsv1alpha1.OpsRequestComponentStatus{},
		}}
		opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops}
		helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}, {ComponentName: "shard"}})

		ops.Status.ClusterGeneration = 9
		phase := helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
		Expect(phase).To(Equal(opsv1alpha1.OpsRunningPhase))
		ops.Status.ClusterGeneration = 8
		phase = helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
		Expect(phase).To(Equal(opsv1alpha1.OpsRunningPhase))

		status := cluster.Status.Components["mysql"]
		status.ObservedGeneration = cluster.Generation
		status.UpToDate = false
		status.Phase = appsv1.FailedComponentPhase
		cluster.Status.Components["mysql"] = status
		phase = helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
		Expect(phase).To(Equal(opsv1alpha1.OpsRunningPhase))

		status.UpToDate = true
		cluster.Status.Components["mysql"] = status
		phase = helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
		Expect(phase).To(Equal(opsv1alpha1.OpsRunningPhase))

		status.Phase = appsv1.RunningComponentPhase
		cluster.Status.Components["mysql"] = status
		phase = helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
		Expect(phase).To(Equal(opsv1alpha1.OpsSucceedPhase))
		phase = helper.componentActionPhase(opsRes, appsv1.StoppedComponentPhase)
		Expect(phase).To(Equal(opsv1alpha1.OpsRunningPhase))
	})
})

func TestSyncCurrentProgressDetails(t *testing.T) {
	newDetail := func(objectKey string) opsv1alpha1.ProgressStatusDetail {
		return opsv1alpha1.ProgressStatusDetail{
			ObjectKey: objectKey,
			Status:    opsv1alpha1.ProcessingProgressStatus,
			Message:   "Start to start",
		}
	}
	failedTime := metav1.NewTime(time.Now().Add(-time.Hour))
	opsRequest := &opsv1alpha1.OpsRequest{Status: opsv1alpha1.OpsRequestStatus{Components: map[string]opsv1alpha1.OpsRequestComponentStatus{
		"mysql": {ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
			ObjectKey: "Pod/cluster-mysql-0",
			Status:    opsv1alpha1.FailedProgressStatus,
			EndTime:   failedTime,
		}}},
		"shard": {ProgressDetails: []opsv1alpha1.ProgressStatusDetail{
			newDetail("Pod/cluster-shard-0-0"),
			newDetail("Pod/cluster-shard-1-0"),
			newDetail("Pod/cluster-shard-deleted-0"),
		}},
		"gone": {ProgressDetails: []opsv1alpha1.ProgressStatusDetail{
			newDetail("Pod/cluster-gone-0-0"),
		}},
		"unrelated": {ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
			ObjectKey: "Pod/proxy-0",
			Status:    opsv1alpha1.ProcessingProgressStatus,
		}}},
	}}}
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}},
		Shardings:      []appsv1.ClusterSharding{{Name: "shard"}},
	}}
	opsRes := &OpsResource{Cluster: cluster, OpsRequest: opsRequest, Recorder: record.NewFakeRecorder(3)}
	helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}, {ComponentName: "shard"}, {ComponentName: "gone"}})
	current := helper.emptyInstanceProgress(opsRes)
	current["mysql"] = []opsv1alpha1.ProgressStatusDetail{newDetail("Pod/cluster-mysql-0")}
	current["shard"] = []opsv1alpha1.ProgressStatusDetail{
		newDetail("Pod/cluster-shard-1-0"),
		newDetail("Pod/cluster-shard-0-0"),
	}
	syncCurrentProgressDetails(opsRes, current)
	detail := opsRequest.Status.Components["mysql"].ProgressDetails[0]
	if detail.Status != opsv1alpha1.ProcessingProgressStatus || !detail.EndTime.IsZero() || detail.StartTime.IsZero() {
		t.Fatalf("detail=%+v, want the recovered current observation with its terminal time cleared", detail)
	}
	shardDetails := opsRequest.Status.Components["shard"].ProgressDetails
	if len(shardDetails) != 2 {
		t.Fatalf("sharding details=%+v, want details for current physical components only", shardDetails)
	}
	if shardDetails[0].ObjectKey != "Pod/cluster-shard-0-0" || shardDetails[1].ObjectKey != "Pod/cluster-shard-1-0" {
		t.Fatalf("sharding details=%+v, want stable current-row ordering", shardDetails)
	}
	if len(opsRequest.Status.Components["gone"].ProgressDetails) != 0 {
		t.Fatalf("gone sharding details=%+v, want stale details removed", opsRequest.Status.Components["gone"].ProgressDetails)
	}
	if len(opsRes.Recorder.(*record.FakeRecorder).Events) != 1 {
		t.Fatalf("events=%d, want only the changed failed-to-processing event", len(opsRes.Recorder.(*record.FakeRecorder).Events))
	}
	if opsRequest.Status.Components["unrelated"].ProgressDetails[0].Status != opsv1alpha1.ProcessingProgressStatus {
		t.Fatal("unrelated component detail was changed")
	}
}

func TestRunningInstanceProgress(t *testing.T) {
	const instanceName = "cluster-mysql-0"
	opsRes := &OpsResource{Cluster: &appsv1.Cluster{}}
	pgRes := &instanceProgressResource{
		opsMessageKey:     "upgrade",
		fullComponentName: "mysql",
		clusterComponent:  &appsv1.ClusterComponentSpec{Name: "mysql", Replicas: 3},
	}
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{Replicas: pointer.Int32(1)},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
			PodName:      instanceName,
			DesiredState: workloads.InstanceDesiredStateActive,
			CurrentState: workloads.InstanceCurrentStatePresent,
			UpToDate:     false,
			Ready:        true,
			Available:    true,
		}}},
	}

	result := handleRunningInstanceProgress(opsRes, pgRes, its)
	if result.expectedCount != 1 || result.completedCount != 0 || !result.observationsComplete {
		t.Fatalf("progress=%d/%d, want 0/1 until UpToDate", result.completedCount, result.expectedCount)
	}
	its.Status.InstanceStatus[0].UpToDate = true
	its.Generation = 2
	its.Status.ObservedGeneration = 1
	result = handleRunningInstanceProgress(opsRes, pgRes, its)
	if result.observationsComplete {
		t.Fatal("stale InstanceSet status must keep the observation incomplete")
	}
	its.Status.ObservedGeneration = its.Generation
	result = handleRunningInstanceProgress(opsRes, pgRes, its)
	if result.expectedCount != 1 || result.completedCount != 1 || result.succeededCount != 1 {
		t.Fatalf("progress=%d/%d, want 1/1", result.completedCount, result.expectedCount)
	}
	its.Status.InstanceStatus[0].Failed = true
	result = handleRunningInstanceProgress(opsRes, pgRes, its)
	if result.expectedCount != 1 || result.completedCount != 1 || result.succeededCount != 0 || result.details[0].Status != opsv1alpha1.FailedProgressStatus {
		t.Fatalf("failed progress=%d/%d details=%v", result.completedCount, result.expectedCount, result.details)
	}
}
