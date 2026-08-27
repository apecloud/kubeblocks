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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestRollingTargetsState(t *testing.T) {
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

	completed, failed := helper.rollingTargetsState(opsRes, appsv1.RunningComponentPhase)
	if completed || failed {
		t.Fatalf("stale status completed=%v failed=%v, want processing", completed, failed)
	}

	status := cluster.Status.Components["mysql"]
	status.ObservedGeneration = cluster.Generation
	status.UpToDate = false
	status.Phase = appsv1.FailedComponentPhase
	cluster.Status.Components["mysql"] = status
	completed, failed = helper.rollingTargetsState(opsRes, appsv1.RunningComponentPhase)
	if completed || failed {
		t.Fatalf("non-current failure completed=%v failed=%v, want processing", completed, failed)
	}

	status.UpToDate = true
	cluster.Status.Components["mysql"] = status
	completed, failed = helper.rollingTargetsState(opsRes, appsv1.RunningComponentPhase)
	if completed || !failed {
		t.Fatalf("current failure completed=%v failed=%v, want failed", completed, failed)
	}

	status.Phase = appsv1.RunningComponentPhase
	cluster.Status.Components["mysql"] = status
	completed, failed = helper.rollingTargetsState(opsRes, appsv1.RunningComponentPhase)
	if !completed || failed {
		t.Fatalf("current running status completed=%v failed=%v, want success", completed, failed)
	}
	completed, failed = helper.rollingTargetsState(opsRes, appsv1.StoppedComponentPhase)
	if completed || failed {
		t.Fatalf("running status completed=%v failed=%v, want stop processing", completed, failed)
	}
}

func TestComponentStopFieldsUnchanged(t *testing.T) {
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{
			{Name: "mysql", Stop: pointer.Bool(true)},
			{Name: "proxy"},
		},
		Shardings: []appsv1.ClusterSharding{{Name: "shard", Template: appsv1.ClusterComponentSpec{Stop: pointer.Bool(true)}}},
	}}
	if !newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "mysql"}}).
		componentStopFieldsUnchanged(cluster, true) {
		t.Fatal("matching stop target was rejected")
	}
	if newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "proxy"}}).
		componentStopFieldsUnchanged(cluster, true) {
		t.Fatal("overwritten stop target was accepted")
	}
	if !newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "proxy"}}).
		componentStopFieldsUnchanged(cluster, false) {
		t.Fatal("matching start target was rejected")
	}
	if newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "missing"}}).
		componentStopFieldsUnchanged(cluster, false) {
		t.Fatal("missing target was accepted")
	}
}

func TestInstanceStatusProgress(t *testing.T) {
	const instanceName = "cluster-mysql-0"
	opsRes := &OpsResource{
		Cluster:    &appsv1.Cluster{},
		OpsRequest: &opsv1alpha1.OpsRequest{},
		Recorder:   record.NewFakeRecorder(10),
	}
	pgRes := &progressResource{
		opsMessageKey:     "upgrade",
		fullComponentName: "mysql",
		clusterComponent:  &appsv1.ClusterComponentSpec{Name: "mysql", Replicas: 1},
	}
	compStatus := &opsv1alpha1.OpsRequestComponentStatus{}
	workload := rollingWorkload{
		exists:          true,
		desiredReplicas: 1,
		instances: []workloads.InstanceStatus{{
			PodName:         instanceName,
			DesiredState:    workloads.InstanceDesiredStateActive,
			CurrentState:    workloads.InstanceCurrentStatePresent,
			UpToDate:        true,
			Ready:           true,
			Available:       false,
			CurrentRevision: "revision-1",
			UpdateRevision:  "revision-1",
		}},
	}

	expected, completed := handleActiveInstanceStatusProgress(opsRes, pgRes, compStatus, workload)
	if expected != 1 || completed != 0 {
		t.Fatalf("progress=%d/%d, want 0/1 until Available", completed, expected)
	}
	workload.instances[0].Available = true
	expected, completed = handleActiveInstanceStatusProgress(opsRes, pgRes, compStatus, workload)
	if expected != 1 || completed != 1 {
		t.Fatalf("progress=%d/%d, want 1/1", completed, expected)
	}
	workload.instances[0].Failed = true
	expected, completed = handleActiveInstanceStatusProgress(opsRes, pgRes, compStatus, workload)
	if expected != 1 || completed != 1 || compStatus.ProgressDetails[0].Status != opsv1alpha1.FailedProgressStatus {
		t.Fatalf("failed progress=%d/%d details=%v", completed, expected, compStatus.ProgressDetails)
	}

	workload.instances[0].CurrentState = workloads.InstanceCurrentStateAbsent
	expected, completed = handleStoppedInstanceStatusProgress(opsRes, pgRes, compStatus, workload)
	if expected != 1 || completed != 1 {
		t.Fatalf("stop progress=%d/%d, want 1/1", completed, expected)
	}
}
