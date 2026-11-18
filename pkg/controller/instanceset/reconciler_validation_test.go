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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestDirectConfigOverridesValidateInactiveTemplate(t *testing.T) {
	its, tree, _, _ := newDirectConfigFixture(t, "named")
	its.Spec.Instances[0].Replicas = ptr.To[int32](0)
	its.Spec.Instances[0].Configs[0].Volume.VolumeName = "missing"
	if _, err := NewValidationReconciler().Reconcile(tree); err == nil {
		t.Fatal("inactive template's invalid binding was accepted")
	}
}

func TestDirectConfigOverridesRejectPVCVolumeCollision(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "active inherited claim", true: "inactive template claim"}[inactive], func(t *testing.T) {
			its, tree, _, _ := newDirectConfigFixture(t, "named")
			claim := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
				},
			}
			if inactive {
				its.Spec.Instances[0].Replicas = ptr.To[int32](0)
				its.Spec.Instances[0].VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
			} else {
				its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
			}
			if _, err := NewValidationReconciler().Reconcile(tree); err == nil {
				t.Fatal("binding was accepted despite PVC replacing the ConfigMap volume")
			}
			// The existing same-name volume/PVC behavior remains valid without a binding.
			its.Spec.Instances[0].Configs = nil
			runDirectReconciler(t, NewValidationReconciler(), tree)
		})
	}
}
