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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestGetWorkloadBuildsNotJoinedSetFromReplicasStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatalf("add scheme: %v", err)
		}
	}

	const (
		namespace   = "default"
		clusterName = "test-cluster"
		compName    = "kafka"
	)
	pod0 := clusterName + "-" + compName + "-0"
	pod1 := clusterName + "-" + compName + "-1"
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, compName),
			Annotations: map[string]string{
				"apps.kubeblocks.io/replicas-status": `{"replicas":2,"status":[` +
					`{"name":"` + pod0 + `","generation":"2","creationTimestamp":"0001-01-01T00:00:00Z","provisioned":true,"memberJoined":true},` +
					`{"name":"` + pod1 + `","generation":"3","creationTimestamp":"2026-07-03T02:04:32Z","memberJoined":false}]}`,
			},
		},
		Status: workloads.InstanceSetStatus{
			CurrentRevisions: map[string]string{pod0: "rev-a", pod1: "rev-a"},
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its).Build()
	rt := newOpsRuntime(context.Background(), cli, "")
	workload, err := rt.GetWorkload(namespace, clusterName, compName)
	if err != nil {
		t.Fatalf("get workload: %v", err)
	}
	notJoined := workload.GetNotJoinedInstanceNameSet()
	if !notJoined.Has(pod1) {
		t.Fatalf("expected %s in not-joined set, got %v", pod1, notJoined.UnsortedList())
	}
	if notJoined.Has(pod0) {
		t.Fatalf("did not expect %s in not-joined set, got %v", pod0, notJoined.UnsortedList())
	}
}

func TestHandleScaleOutProgressWaitsForMemberJoin(t *testing.T) {
	const (
		compName = "kafka"
		podName  = "test-cluster-kafka-1"
	)

	newFixtures := func(notJoined sets.Set[string]) (*OpsResource, *progressResource, Workload, *opsv1alpha1.OpsRequestComponentStatus) {
		workload := &defaultWorkload{
			currentRevisionMap: map[string]string{podName: "rev-a"},
			notReadySet:        sets.New[string](),
			notAvailableSet:    sets.New[string](),
			failedSet:          sets.New[string](),
			instanceNames:      sets.New(podName),
			notJoinedSet:       notJoined,
		}
		opsRes := &OpsResource{
			Recorder: record.NewFakeRecorder(16),
			OpsRequest: &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "test-scale-out", Namespace: "default"},
			},
		}
		pgRes := &progressResource{
			clusterComponent:  &appsv1.ClusterComponentSpec{Name: compName},
			componentDef:      &appsv1.ComponentDefinition{},
			fullComponentName: compName,
			createdPodSet:     map[string]string{podName: ""},
			compOps:           opsv1alpha1.ComponentOps{ComponentName: compName},
		}
		return opsRes, pgRes, workload, &opsv1alpha1.OpsRequestComponentStatus{}
	}

	// a created and ready pod whose memberJoin/dataLoad has not completed must not
	// be counted as completed; it stays in Processing.
	opsRes, pgRes, workload, compStatus := newFixtures(sets.New(podName))
	completed, err := handleScaleOutProgressWithWorkload(opsRes, pgRes, workload, compStatus)
	if err != nil {
		t.Fatalf("handle scale-out progress: %v", err)
	}
	if completed != 0 {
		t.Fatalf("expected 0 completed while member join is pending, got %d", completed)
	}
	if len(compStatus.ProgressDetails) != 1 {
		t.Fatalf("expected 1 progress detail, got %d", len(compStatus.ProgressDetails))
	}
	if compStatus.ProgressDetails[0].Status != opsv1alpha1.ProcessingProgressStatus {
		t.Fatalf("expected Processing progress while member join is pending, got %s", compStatus.ProgressDetails[0].Status)
	}

	// control: once the replica has joined, the same pod counts as completed.
	opsRes, pgRes, workload, compStatus = newFixtures(sets.New[string]())
	completed, err = handleScaleOutProgressWithWorkload(opsRes, pgRes, workload, compStatus)
	if err != nil {
		t.Fatalf("handle scale-out progress: %v", err)
	}
	if completed != 1 {
		t.Fatalf("expected 1 completed after member join, got %d", completed)
	}
	if len(compStatus.ProgressDetails) != 1 || compStatus.ProgressDetails[0].Status != opsv1alpha1.SucceedProgressStatus {
		t.Fatalf("expected Succeed progress after member join, got %+v", compStatus.ProgressDetails)
	}
}
