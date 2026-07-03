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

func TestGetWorkloadBuildsUnprovisionedSetFromReplicasStatus(t *testing.T) {
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
	if !workload.HasProvisioningStatusSource() {
		t.Fatal("expected ITS-backed workload to have a provisioning status source")
	}
	unprovisioned := workload.GetUnprovisionedInstanceNameSet()
	if !unprovisioned.Has(pod1) {
		t.Fatalf("expected %s in unprovisioned set, got %v", pod1, unprovisioned.UnsortedList())
	}
	if unprovisioned.Has(pod0) {
		t.Fatalf("did not expect %s in unprovisioned set, got %v", pod0, unprovisioned.UnsortedList())
	}
}

func TestHandleScaleOutProgressWaitsForProvisioning(t *testing.T) {
	const (
		compName = "kafka"
		podName  = "test-cluster-kafka-1"
	)

	compDefWithMemberJoin := &appsv1.ComponentDefinition{
		Spec: appsv1.ComponentDefinitionSpec{
			LifecycleActions: &appsv1.ComponentLifecycleActions{
				MemberJoin: &appsv1.Action{},
			},
		},
	}
	compDefWithDataLoad := &appsv1.ComponentDefinition{
		Spec: appsv1.ComponentDefinitionSpec{
			LifecycleActions: &appsv1.ComponentLifecycleActions{
				DataLoad: &appsv1.Action{},
			},
		},
	}
	compDefWithoutActions := &appsv1.ComponentDefinition{}

	newFixtures := func(unprovisioned sets.Set[string], sourced bool, compDef *appsv1.ComponentDefinition) (*OpsResource, *progressResource, Workload, *opsv1alpha1.OpsRequestComponentStatus) {
		workload := &defaultWorkload{
			currentRevisionMap:        map[string]string{podName: "rev-a"},
			notReadySet:               sets.New[string](),
			notAvailableSet:           sets.New[string](),
			failedSet:                 sets.New[string](),
			instanceNames:             sets.New(podName),
			unprovisionedSet:          unprovisioned,
			provisioningStatusSourced: sourced,
		}
		opsRes := &OpsResource{
			Recorder: record.NewFakeRecorder(16),
			OpsRequest: &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "test-scale-out", Namespace: "default"},
			},
		}
		pgRes := &progressResource{
			clusterComponent:  &appsv1.ClusterComponentSpec{Name: compName},
			componentDef:      compDef,
			fullComponentName: compName,
			createdPodSet:     map[string]string{podName: ""},
			compOps:           opsv1alpha1.ComponentOps{ComponentName: compName},
		}
		return opsRes, pgRes, workload, &opsv1alpha1.OpsRequestComponentStatus{}
	}

	assertProgress := func(t *testing.T, opsRes *OpsResource, pgRes *progressResource, workload Workload,
		compStatus *opsv1alpha1.OpsRequestComponentStatus, wantCompleted int32, wantStatus opsv1alpha1.ProgressStatus) {
		t.Helper()
		completed, err := handleScaleOutProgressWithWorkload(opsRes, pgRes, workload, compStatus)
		if err != nil {
			t.Fatalf("handle scale-out progress: %v", err)
		}
		if completed != wantCompleted {
			t.Fatalf("expected %d completed, got %d", wantCompleted, completed)
		}
		if len(compStatus.ProgressDetails) != 1 {
			t.Fatalf("expected 1 progress detail, got %d", len(compStatus.ProgressDetails))
		}
		if compStatus.ProgressDetails[0].Status != wantStatus {
			t.Fatalf("expected %s progress, got %s", wantStatus, compStatus.ProgressDetails[0].Status)
		}
	}

	// case a: a created and ready pod whose memberJoin has not completed
	// (MemberJoined=false) must not be counted as completed; it stays in Processing.
	t.Run("member join open", func(t *testing.T) {
		opsRes, pgRes, workload, compStatus := newFixtures(sets.New(podName), true, compDefWithMemberJoin)
		assertProgress(t, opsRes, pgRes, workload, compStatus, 0, opsv1alpha1.ProcessingProgressStatus)
	})

	// case b: same with dataLoad still open (DataLoaded=false, MemberJoined nil).
	t.Run("data load open", func(t *testing.T) {
		opsRes, pgRes, workload, compStatus := newFixtures(sets.New(podName), true, compDefWithDataLoad)
		assertProgress(t, opsRes, pgRes, workload, compStatus, 0, opsv1alpha1.ProcessingProgressStatus)
	})

	// case c: both records nil (closed or not applicable) — the replica counts as completed.
	t.Run("provisioning record closed", func(t *testing.T) {
		opsRes, pgRes, workload, compStatus := newFixtures(sets.New[string](), true, compDefWithMemberJoin)
		assertProgress(t, opsRes, pgRes, workload, compStatus, 1, opsv1alpha1.SucceedProgressStatus)
	})

	// case d: fallback-unknown — a workload view without a provisioning record source
	// must not silently pass when the component defines provisioning lifecycle actions.
	t.Run("fallback unknown with provisioning actions", func(t *testing.T) {
		opsRes, pgRes, workload, compStatus := newFixtures(sets.New[string](), false, compDefWithMemberJoin)
		assertProgress(t, opsRes, pgRes, workload, compStatus, 0, opsv1alpha1.ProcessingProgressStatus)
	})
	t.Run("fallback unknown without provisioning actions", func(t *testing.T) {
		opsRes, pgRes, workload, compStatus := newFixtures(sets.New[string](), false, compDefWithoutActions)
		assertProgress(t, opsRes, pgRes, workload, compStatus, 1, opsv1alpha1.SucceedProgressStatus)
	})
}

func TestComponentOpsCompletedSkipSemantics(t *testing.T) {
	const replicas = int32(2)
	cases := []struct {
		name           string
		skip           bool
		phase          appsv1.ComponentPhase
		expectCount    int32
		completedCount int32
		want           bool
	}{
		// the skip flag only skips the component terminal-phase gate: with all
		// operation-owned objects completed, a non-terminal component phase must
		// not block completion.
		{"skip=true allows completion despite non-terminal component phase", true, appsv1.UpdatingComponentPhase, 1, 1, true},
		{"skip=false keeps waiting on non-terminal component phase", false, appsv1.UpdatingComponentPhase, 1, 1, false},
		// the operation-owned objects gate is never skippable.
		{"skip=true never overrides incomplete owned objects", true, appsv1.RunningComponentPhase, 1, 0, false},
		{"skip=false with terminal phase and completed objects completes", false, appsv1.RunningComponentPhase, 1, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := componentOpsCompleted(c.skip, c.phase, replicas, c.expectCount, c.completedCount)
			if got != c.want {
				t.Fatalf("componentOpsCompleted(skip=%v, phase=%s, %d/%d) = %v, want %v",
					c.skip, c.phase, c.completedCount, c.expectCount, got, c.want)
			}
		})
	}
}
