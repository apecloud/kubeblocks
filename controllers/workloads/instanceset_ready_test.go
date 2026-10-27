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

package workloads

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestInstanceSetReadyOnResume(t *testing.T) {
	t.Run("Pods", func(t *testing.T) {
		cli := legacyStopRecoveryFixture(t, 2, 0)
		legacyStopRecoveryReady(t, cli)
		before := ageInstanceSetReadyCondition(t, cli, legacyStopRecoverySet(t, cli))
		legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
		legacyStopRecoveryReconcile(t, cli, 6)
		stopped := legacyStopRecoverySet(t, cli)
		if stopped.Status.Replicas != 0 {
			t.Fatal("fixture did not drain runtime")
		}
		assertInstanceSetNoRuntime(t, stopped)
		legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
		legacyStopRecoveryReconcile(t, cli, 1)
		assertInstanceSetNotReady(t, legacyStopRecoverySet(t, cli))
		legacyStopRecoveryReady(t, cli)
		assertInstanceSetFreshReady(t, legacyStopRecoverySet(t, cli), before)
	})
	t.Run("Instances", func(t *testing.T) {
		cli := newSetStopClient(t, setStopFixture())
		reconcileSetStop(t, cli, 5)
		bindSetStopClaims(t, cli)
		markSetStopPodsReady(t, cli)
		reconcileSetStop(t, cli, 5)
		before := ageInstanceSetReadyCondition(t, cli, readSetStop(t, cli))
		mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
		reconcileSetStop(t, cli, 10)
		stopped := readSetStop(t, cli)
		if stopped.Status.Replicas != 0 {
			t.Fatal("fixture did not drain runtime")
		}
		assertInstanceSetNoRuntime(t, stopped)
		mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
		if _, err := reconcileSetStopParent(cli); err != nil {
			t.Fatal(err)
		}
		assertInstanceSetNotReady(t, readSetStop(t, cli))
		reconcileSetStop(t, cli, 3)
		markSetStopPodsReady(t, cli)
		reconcileSetStop(t, cli, 3)
		assertInstanceSetFreshReady(t, readSetStop(t, cli), before)
	})
}

func TestInstanceSetReadyWithZeroReplicas(t *testing.T) {
	t.Run("Pods", func(t *testing.T) {
		cli := legacyStopRecoveryFixture(t, 0, 0)
		assertInstanceSetNoRuntime(t, legacyStopRecoverySet(t, cli))
	})
	t.Run("Instances", func(t *testing.T) {
		its := setStopFixture()
		its.Spec.Replicas = ptr.To[int32](0)
		cli := newSetStopClient(t, its)
		reconcileSetStop(t, cli, 5)
		assertInstanceSetNoRuntime(t, readSetStop(t, cli))
	})
}

func TestInstanceSetReadyDoesNotWaitForMinReadySeconds(t *testing.T) {
	t.Run("Pods", func(t *testing.T) {
		cli := legacyStopRecoveryFixture(t, 2, 3600)
		legacyStopRecoveryReady(t, cli)
		assertInstanceSetReadyBeforeAvailable(t, legacyStopRecoverySet(t, cli))
	})
	t.Run("Instances", func(t *testing.T) {
		its := setStopFixture()
		its.Spec.MinReadySeconds = 86400
		cli := newSetStopClient(t, its)
		reconcileSetStop(t, cli, 5)
		bindSetStopClaims(t, cli)
		markSetStopPodsReady(t, cli)
		reconcileSetStop(t, cli, 5)
		assertInstanceSetReadyBeforeAvailable(t, readSetStop(t, cli))
	})
}

func assertInstanceSetReadyBeforeAvailable(t *testing.T, its *workloads.InstanceSet) {
	t.Helper()
	if !meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady)) ||
		!meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceAvailable)) {
		t.Fatalf("expected Ready before MinReadySeconds made runtime Available: %#v", its.Status.Conditions)
	}
}

func TestInstanceSetReadyDescribesPresentRuntimeHealth(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(readinessPathName(instanceAPI), func(t *testing.T) {
			cli, read, reconcile, _ := instanceSetReadinessFixture(t, instanceAPI)
			var member client.Object
			if instanceAPI {
				member = &listSetStopChildren(t, cli)[0]
			} else {
				member = &legacyStopRecoveryPods(t, cli)[0]
			}
			member.SetFinalizers(nil)
			if err := cli.Update(context.Background(), member); err != nil {
				t.Fatal(err)
			}
			if err := cli.Delete(context.Background(), member); err != nil {
				t.Fatal(err)
			}
			reconcile()
			its := read()
			if its.Status.Replicas != 1 || its.Status.ReadyReplicas != 1 || !meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady)) || its.IsInstancesReady() {
				t.Fatalf("runtime health must remain separate from desired replica convergence: %#v", its.Status)
			}
		})
	}
}

func ageInstanceSetReadyCondition(t *testing.T, cli client.Client, its *workloads.InstanceSet) metav1.Time {
	t.Helper()
	ready := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady))
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatal("fixture is not ready")
	}
	ready.LastTransitionTime = metav1.NewTime(time.Now().Add(-2 * time.Minute))
	if err := cli.Status().Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	return ready.LastTransitionTime
}

func assertInstanceSetFreshReady(t *testing.T, its *workloads.InstanceSet, before metav1.Time) {
	t.Helper()
	ready := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady))
	if ready == nil || ready.Status != metav1.ConditionTrue || !ready.LastTransitionTime.After(before.Time) {
		t.Fatalf("new runtime inherited the old ready transition: before=%v after=%#v", before, ready)
	}
}

func assertInstanceSetNoRuntime(t *testing.T, its *workloads.InstanceSet) {
	t.Helper()
	if its.Status.Replicas != 0 || its.Status.ReadyReplicas != 0 || its.Status.AvailableReplicas != 0 {
		t.Fatalf("fixture still has runtime: %#v", its.Status)
	}
	assertInstanceSetNotReady(t, its)
	if !meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceAvailable)) {
		t.Fatalf("empty runtime is Available: %#v", its.Status.Conditions)
	}
}

func TestInstanceSetReadyPreservesHealthyTransition(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(readinessPathName(instanceAPI), func(t *testing.T) {
			_, read, reconcile, change := instanceSetReadinessFixture(t, instanceAPI)
			before := meta.FindStatusCondition(read().Status.Conditions, string(workloads.InstanceReady))
			if before == nil || before.Status != metav1.ConditionTrue {
				t.Fatal("fixture is not ready")
			}
			change(func(its *workloads.InstanceSet) { its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement })
			reconcile()
			reconcile()
			after := meta.FindStatusCondition(read().Status.Conditions, string(workloads.InstanceReady))
			if after == nil || after.Status != metav1.ConditionTrue || !after.LastTransitionTime.Equal(&before.LastTransitionTime) {
				t.Fatalf("unchanged healthy runtime lost its ready transition: before=%#v after=%#v", before, after)
			}
		})
	}
}

func instanceSetReadinessFixture(t *testing.T, instanceAPI bool) (client.Client, func() *workloads.InstanceSet, func(), func(func(*workloads.InstanceSet))) {
	t.Helper()
	if !instanceAPI {
		cli := legacyStopRecoveryFixture(t, 2, 0)
		legacyStopRecoveryReady(t, cli)
		return cli, func() *workloads.InstanceSet { return legacyStopRecoverySet(t, cli) },
			func() { legacyStopRecoveryReconcile(t, cli, 1) },
			func(change func(*workloads.InstanceSet)) { legacyStopRecoveryChange(t, cli, change) }
	}
	cli := newSetStopClient(t, setStopFixture())
	reconcileSetStop(t, cli, 5)
	bindSetStopClaims(t, cli)
	markSetStopPodsReady(t, cli)
	reconcileSetStop(t, cli, 5)
	return cli, func() *workloads.InstanceSet { return readSetStop(t, cli) },
		func() {
			if _, err := reconcileSetStopParent(cli); err != nil {
				t.Fatal(err)
			}
		}, func(change func(*workloads.InstanceSet)) { mutateSetStop(t, cli, change) }
}

func readinessPathName(instanceAPI bool) string {
	if instanceAPI {
		return "Instances"
	}
	return "Pods"
}

func assertInstanceSetNotReady(t *testing.T, its *workloads.InstanceSet) {
	t.Helper()
	ready := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady))
	if ready == nil || ready.Status != metav1.ConditionFalse {
		t.Fatalf("generation=%d observed=%d replicas=%d ready=%d retained stale Ready: %#v", its.Generation, its.Status.ObservedGeneration, its.Status.Replicas, its.Status.ReadyReplicas, ready)
	}
}
