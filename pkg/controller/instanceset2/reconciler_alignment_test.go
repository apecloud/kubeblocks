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

package instanceset2

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
)

func TestRestorePVCInitialStepCompletedForInstance(t *testing.T) {
	makeTemplate := func(restoreAnnot bool) *instancetemplate.InstanceTemplateExt {
		vct := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}
		if restoreAnnot {
			vct.Annotations = map[string]string{
				constant.RestoreSourceKindAnnotationKey: "Backup",
			}
		}
		return &instancetemplate.InstanceTemplateExt{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{vct},
		}
	}

	makeInst := func(annotations map[string]string) *workloads.Instance {
		return &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-0",
				Namespace:   "default",
				Annotations: annotations,
			},
		}
	}

	t.Run("nil template returns false", func(t *testing.T) {
		inst := makeInst(nil)
		if restorePVCInitialStepCompletedForInstance(inst, nil) {
			t.Error("expected false for nil template")
		}
	})

	t.Run("no restore VCT returns false", func(t *testing.T) {
		inst := makeInst(map[string]string{
			constant.RestorePVCInitialStepCompletedAnnotationKey: "true",
		})
		template := makeTemplate(false)
		if restorePVCInitialStepCompletedForInstance(inst, template) {
			t.Error("expected false when no restore VCT")
		}
	})

	t.Run("restore VCT without annotation returns false", func(t *testing.T) {
		inst := makeInst(nil)
		template := makeTemplate(true)
		if restorePVCInitialStepCompletedForInstance(inst, template) {
			t.Error("expected false without annotation")
		}
	})

	t.Run("restore VCT with annotation returns true", func(t *testing.T) {
		inst := makeInst(map[string]string{
			constant.RestorePVCInitialStepCompletedAnnotationKey: "true",
		})
		template := makeTemplate(true)
		if !restorePVCInitialStepCompletedForInstance(inst, template) {
			t.Error("expected true with annotation and restore VCT")
		}
	})

	t.Run("wrong annotation value returns false", func(t *testing.T) {
		inst := makeInst(map[string]string{
			constant.RestorePVCInitialStepCompletedAnnotationKey: "false",
		})
		template := makeTemplate(true)
		if restorePVCInitialStepCompletedForInstance(inst, template) {
			t.Error("expected false with wrong annotation value")
		}
	})
}
