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

package controllerutil

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestInstanceStatusHelpers(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
		},
		Spec: workloads.InstanceSpec{
			Roles: []workloads.ReplicaRole{{Name: "leader"}},
		},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: 3,
			// Runtime health is independent from desired-state convergence. A dynamic config
			// update or PVC expansion can keep UpToDate false while the Instance stays ready.
			UpToDate: false,
			Role:     "leader",
			Conditions: []metav1.Condition{
				{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
				{Type: string(workloads.InstanceFailure), Status: metav1.ConditionFalse},
			},
		},
	}

	if !IsInstanceReady(inst) {
		t.Fatalf("expected instance to be ready")
	}
	if !IsInstanceReadyWithRole(inst) {
		t.Fatalf("expected instance to be ready with observed role")
	}
	if !IsInstanceAvailable(inst) {
		t.Fatalf("expected instance to be available")
	}
	if IsInstanceFailure(inst) {
		t.Fatalf("expected instance not to be failed")
	}
	inst.Status.Conditions[2].Status = metav1.ConditionTrue
	if !IsInstanceFailure(inst) {
		t.Fatalf("expected current failure observation even when instance is not up-to-date")
	}
	inst.Status.Conditions[2].Status = metav1.ConditionFalse

	inst.Status.Role = ""
	if IsInstanceReadyWithRole(inst) {
		t.Fatalf("role-aware readiness should require observed role when roles are configured")
	}
	inst.Spec.Roles = nil
	if !IsInstanceReadyWithRole(inst) {
		t.Fatalf("role-aware readiness should not require role when roles are not configured")
	}

	inst.Status.ObservedGeneration = 2
	if IsInstanceReady(inst) || IsInstanceAvailable(inst) {
		t.Fatalf("stale status should not be ready or available")
	}
}

func TestIsInstancePending(t *testing.T) {
	now := metav1.NewTime(time.Now())
	makeInst := func(gen, obsGen int64, state workloads.InstanceCurrentState, status metav1.ConditionStatus, reason string, deleting bool) *workloads.Instance {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Generation: gen},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: obsGen,
				CurrentState:       state,
				Conditions:         []metav1.Condition{{Type: string(workloads.InstanceReady), Status: status, Reason: reason}},
			},
		}
		if deleting {
			inst.DeletionTimestamp = &now
		}
		return inst
	}
	tests := []struct {
		name string
		inst *workloads.Instance
		want bool
	}{
		{"present pending pod", makeInst(2, 2, workloads.InstanceCurrentStatePresent, metav1.ConditionFalse, workloads.ReasonPodPending, false), true},
		{"present running not ready pod", makeInst(2, 2, workloads.InstanceCurrentStatePresent, metav1.ConditionFalse, workloads.ReasonNotReady, false), false},
		{"present ready pod", makeInst(2, 2, workloads.InstanceCurrentStatePresent, metav1.ConditionTrue, workloads.ReasonReady, false), false},
		{"absent pod", makeInst(2, 2, workloads.InstanceCurrentStateAbsent, metav1.ConditionFalse, workloads.ReasonPodPending, false), false},
		{"stale observed generation", makeInst(2, 1, workloads.InstanceCurrentStatePresent, metav1.ConditionFalse, workloads.ReasonPodPending, false), false},
		{"terminating instance", makeInst(2, 2, workloads.InstanceCurrentStatePresent, metav1.ConditionFalse, workloads.ReasonPodPending, true), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInstancePending(tt.inst); got != tt.want {
				t.Fatalf("IsInstancePending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstanceTerminatingAndConditionLookup(t *testing.T) {
	now := metav1.NewTime(time.Now())
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
		Status: workloads.InstanceStatus2{
			Conditions: []metav1.Condition{{Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue}},
		},
	}

	if !IsInstanceTerminating(inst) {
		t.Fatalf("expected terminating instance")
	}
	if IsInstanceFailure(inst) {
		t.Fatalf("terminating instance should not be considered failed")
	}
	idx, condition := getInstanceCondition(inst, workloads.InstanceFailure)
	if idx != 0 || condition == nil || condition.Type != string(workloads.InstanceFailure) {
		t.Fatalf("expected failure condition at index 0, got %d %#v", idx, condition)
	}
	idx, condition = getInstanceConditionFromList(nil, string(workloads.InstanceReady))
	if idx != -1 || condition != nil {
		t.Fatalf("expected nil condition list to return no match")
	}
}
