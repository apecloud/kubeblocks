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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestRoleLabelClaimsRepairsMissingLabelsAndExclusiveClaims(t *testing.T) {
	roles := []workloads.ReplicaRole{{Name: "primary", IsExclusive: true}, {Name: "secondary"}}
	primaryOld := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"primary","roleDefined":true,"eventVersion":"100"}`,
	}, Labels: map[string]string{constant.RoleLabelKey: "primary"}}}
	primaryNew := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-1", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"primary","roleDefined":true,"eventVersion":"200"}`,
	}}}
	secondary := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-2", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"secondary","roleDefined":true,"eventVersion":"300"}`,
	}}}

	claims, err := RoleLabelClaims(roles, []*corev1.Pod{primaryOld, primaryNew, secondary})
	if err != nil {
		t.Fatal(err)
	}
	if claims[primaryNew.Name] != "primary" || claims[primaryOld.Name] != "" || claims[secondary.Name] != "secondary" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRoleLabelClaimsPreferAuthoritativeObservation(t *testing.T) {
	roles := []workloads.ReplicaRole{{Name: "primary", IsExclusive: true}}
	old := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"primary","roleDefined":true,"eventVersion":"200","authoritativeVersion":1}`,
	}}}
	new := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-1", Annotations: map[string]string{
		constant.RoleObservationAnnotationKey: `{"role":"primary","roleDefined":true,"eventVersion":"100","authoritativeVersion":2}`,
	}}}

	claims, err := RoleLabelClaims(roles, []*corev1.Pod{old, new})
	if err != nil {
		t.Fatal(err)
	}
	if claims[new.Name] != "primary" || claims[old.Name] != "" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}
