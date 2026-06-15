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
			UpToDate:           true,
			Role:               "leader",
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
