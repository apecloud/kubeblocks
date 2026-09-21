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

package instanceset

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestRepairRoleLabelsUsesAcceptedObservations(t *testing.T) {
	roles := []workloads.ReplicaRole{{Name: "primary", IsExclusive: true}}
	primary := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"primary","roleDefined":true,"eventVersion":"obs:200"}`,
	}, Labels: map[string]string{constant.RoleLabelKey: "primary"}}}
	peer := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-1", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"primary","roleDefined":true,"eventVersion":"obs:100"}`,
	}, Labels: map[string]string{constant.RoleLabelKey: "primary"}}}

	claims, err := RoleLabelClaims(roles, []*corev1.Pod{primary, peer})
	if err != nil {
		t.Fatal(err)
	}
	if err := RepairRoleLabels(nil, []*corev1.Pod{primary, peer}, claims); err != nil {
		t.Fatal(err)
	}
	if primary.Labels[constant.RoleLabelKey] != "primary" {
		t.Fatalf("primary claim was removed: %#v", primary.Labels)
	}
	if peer.Labels[constant.RoleLabelKey] != "" {
		t.Fatalf("stale exclusive claim was retained: %#v", peer.Labels)
	}
}

func TestRepairRoleLabelsRestoresMissingNonExclusiveLabel(t *testing.T) {
	roles := []workloads.ReplicaRole{{Name: "secondary"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"secondary","roleDefined":true,"eventVersion":"obs:100"}`,
	}}}

	claims, err := RoleLabelClaims(roles, []*corev1.Pod{pod})
	if err != nil {
		t.Fatal(err)
	}
	if err := RepairRoleLabels(nil, []*corev1.Pod{pod}, claims); err != nil {
		t.Fatal(err)
	}
	if pod.Labels[constant.RoleLabelKey] != "secondary" {
		t.Fatalf("missing label was not repaired: %#v", pod.Labels)
	}
}

func TestSetInstanceStatusProjectsAndRepairsRoleObservation(t *testing.T) {
	replicas := int32(1)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       workloads.InstanceSetSpec{Replicas: &replicas, Roles: []workloads.ReplicaRole{{Name: "secondary"}}},
		Status:     workloads.InstanceSetStatus{UpdateRevisions: map[string]string{"db-0": ""}},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Namespace: "default", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"secondary","roleDefined":true,"eventVersion":"obs:100"}`,
	}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}}}
	if err := setInstanceStatus(nil, its, []*corev1.Pod{pod}); err != nil {
		t.Fatal(err)
	}
	if pod.Labels[constant.RoleLabelKey] != "secondary" {
		t.Fatalf("role label was not repaired: %#v", pod.Labels)
	}
	if status := its.FindInstanceStatus("db-0"); status == nil || status.Role != "secondary" {
		t.Fatalf("role status was not projected: %#v", status)
	}
}
