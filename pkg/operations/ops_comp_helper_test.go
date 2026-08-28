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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestUpdateRollingActionPhase(t *testing.T) {
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
	phase := helper.updateRollingActionPhase(opsRes, appsv1.RunningComponentPhase)
	if phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("generation before action phase=%s, want Running", phase)
	}
	ops.Status.ClusterGeneration = 8
	phase = helper.updateRollingActionPhase(opsRes, appsv1.RunningComponentPhase)
	if phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("stale status phase=%s, want Running", phase)
	}

	status := cluster.Status.Components["mysql"]
	status.ObservedGeneration = cluster.Generation
	status.UpToDate = false
	status.Phase = appsv1.FailedComponentPhase
	cluster.Status.Components["mysql"] = status
	phase = helper.updateRollingActionPhase(opsRes, appsv1.RunningComponentPhase)
	if phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("non-current failure phase=%s, want Running", phase)
	}

	status.UpToDate = true
	cluster.Status.Components["mysql"] = status
	phase = helper.updateRollingActionPhase(opsRes, appsv1.RunningComponentPhase)
	if phase != opsv1alpha1.OpsFailedPhase {
		t.Fatalf("current failure phase=%s, want Failed", phase)
	}

	status.Phase = appsv1.RunningComponentPhase
	cluster.Status.Components["mysql"] = status
	phase = helper.updateRollingActionPhase(opsRes, appsv1.RunningComponentPhase)
	if phase != opsv1alpha1.OpsSucceedPhase {
		t.Fatalf("current running status phase=%s, want Succeed", phase)
	}
	phase = helper.updateRollingActionPhase(opsRes, appsv1.StoppedComponentPhase)
	if phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("running status phase=%s, want Running while stopping", phase)
	}
}

func TestSyncStartAllRollingProgressDetails(t *testing.T) {
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
		Shardings:      []appsv1.ClusterSharding{{Name: "shard"}, {Name: "gone"}},
	}}
	progressResources := []progressResource{
		{
			opsMessageKey:     "start",
			fullComponentName: "mysql",
			compOps:           opsv1alpha1.ComponentOps{ComponentName: "mysql"},
		},
		{
			opsMessageKey:     "start",
			fullComponentName: "cluster-shard-0",
			compOps:           opsv1alpha1.ComponentOps{ComponentName: "shard"},
		},
		{
			opsMessageKey:     "start",
			fullComponentName: "cluster-shard-1",
			compOps:           opsv1alpha1.ComponentOps{ComponentName: "shard"},
		},
	}
	progressResults := []rollingProgress{
		{details: []opsv1alpha1.ProgressStatusDetail{newDetail("Pod/cluster-mysql-0")}},
		{details: []opsv1alpha1.ProgressStatusDetail{newDetail("Pod/cluster-shard-0-0")}},
		{details: []opsv1alpha1.ProgressStatusDetail{newDetail("Pod/cluster-shard-1-0")}},
	}
	opsRes := &OpsResource{Cluster: cluster, OpsRequest: opsRequest, Recorder: record.NewFakeRecorder(3)}
	helper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{})

	progressSnapshots := helper.rollingProgressSnapshots(cluster, progressResources, progressResults, true)
	syncRollingProgressDetails(opsRes, progressSnapshots)
	detail := opsRequest.Status.Components["mysql"].ProgressDetails[0]
	if detail.Status != opsv1alpha1.SucceedProgressStatus || !detail.EndTime.After(failedTime.Time) ||
		detail.Message != "Successfully start: Pod/cluster-mysql-0 in Component: mysql" {
		t.Fatalf("detail=%+v, want normalized success", detail)
	}
	shardDetails := opsRequest.Status.Components["shard"].ProgressDetails
	if len(shardDetails) != 2 {
		t.Fatalf("sharding details=%+v, want details for current physical components only", shardDetails)
	}
	if shardDetails[0].Message != "Successfully start: Pod/cluster-shard-0-0 in Component: cluster-shard-0" ||
		shardDetails[1].Message != "Successfully start: Pod/cluster-shard-1-0 in Component: cluster-shard-1" {
		t.Fatalf("sharding details=%+v, want physical component names", shardDetails)
	}
	if len(opsRequest.Status.Components["gone"].ProgressDetails) != 0 {
		t.Fatalf("gone sharding details=%+v, want stale details removed", opsRequest.Status.Components["gone"].ProgressDetails)
	}
	if len(opsRes.Recorder.(*record.FakeRecorder).Events) != 3 {
		t.Fatalf("events=%d, want one success event per normalized detail", len(opsRes.Recorder.(*record.FakeRecorder).Events))
	}
	if opsRequest.Status.Components["unrelated"].ProgressDetails[0].Status != opsv1alpha1.ProcessingProgressStatus {
		t.Fatal("unrelated component detail was changed")
	}
}

func TestRunningInstanceProgress(t *testing.T) {
	const instanceName = "cluster-mysql-0"
	opsRes := &OpsResource{Cluster: &appsv1.Cluster{}}
	pgRes := &progressResource{
		opsMessageKey:     "upgrade",
		fullComponentName: "mysql",
		clusterComponent:  &appsv1.ClusterComponentSpec{Name: "mysql", Replicas: 1},
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
	if result.expectedCount != 1 || result.completedCount != 0 {
		t.Fatalf("progress=%d/%d, want 0/1 until UpToDate", result.completedCount, result.expectedCount)
	}
	its.Status.InstanceStatus[0].UpToDate = true
	result = handleRunningInstanceProgress(opsRes, pgRes, its)
	if result.expectedCount != 1 || result.completedCount != 1 {
		t.Fatalf("progress=%d/%d, want 1/1", result.completedCount, result.expectedCount)
	}
	its.Status.InstanceStatus[0].Failed = true
	result = handleRunningInstanceProgress(opsRes, pgRes, its)
	if result.expectedCount != 1 || result.completedCount != 1 || result.details[0].Status != opsv1alpha1.FailedProgressStatus {
		t.Fatalf("failed progress=%d/%d details=%v", result.completedCount, result.expectedCount, result.details)
	}
}
