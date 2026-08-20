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

package instance

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestRestorePVCInitialStepCompleted(t *testing.T) {
	tests := []struct {
		name     string
		pvc      *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name: "no conditions",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{},
			},
			expected: false,
		},
		{
			name: "populating succeed",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimConditionType(constant.DataProtectionPVCConditionPopulating),
							Status: corev1.ConditionTrue,
							Reason: constant.DataProtectionPVCConditionReasonPopulatingSucceed,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "populating provisioned",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimConditionType(constant.DataProtectionPVCConditionPopulating),
							Status: corev1.ConditionTrue,
							Reason: constant.DataProtectionPVCConditionReasonPopulatingProvision,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "restore condition terminal true blocks",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimConditionType(workloads.InstanceRestore),
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PersistentVolumeClaimConditionType(constant.DataProtectionPVCConditionPopulating),
							Status: corev1.ConditionTrue,
							Reason: constant.DataProtectionPVCConditionReasonPopulatingSucceed,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "populating failed",
			pvc: &corev1.PersistentVolumeClaim{
				Status: corev1.PersistentVolumeClaimStatus{
					Conditions: []corev1.PersistentVolumeClaimCondition{
						{
							Type:   corev1.PersistentVolumeClaimConditionType(constant.DataProtectionPVCConditionPopulating),
							Status: corev1.ConditionFalse,
							Reason: constant.DataProtectionPVCConditionReasonPopulatingFailed,
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := restorePVCInitialStepCompleted(tt.pvc)
			if result != tt.expected {
				t.Errorf("restorePVCInitialStepCompleted() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReconcileRestorePVCAnnotation(t *testing.T) {
	makeInst := func(vctAnnotations map[string]string) *workloads.Instance {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-0",
				Namespace: "default",
			},
			Spec: workloads.InstanceSpec{
				InstanceSetName: "test",
			},
		}
		if vctAnnotations != nil {
			inst.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaimTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "data",
						Annotations: vctAnnotations,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			}
		}
		return inst
	}

	makePVC := func(name string, conditions []corev1.PersistentVolumeClaimCondition) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.KBAppInstanceNameLabelKey: "test-0",
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Conditions: conditions,
			},
		}
	}

	t.Run("no restore VCT - no annotation", func(t *testing.T) {
		inst := makeInst(nil)
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(inst)

		r := &statusReconciler{}
		r.reconcileRestorePVCAnnotation(tree, inst)

		if _, ok := inst.Annotations[constant.RestorePVCInitialStepCompletedAnnotationKey]; ok {
			t.Error("expected no annotation when no restore VCTs")
		}
	})

	t.Run("restore VCT but PVC not found - no annotation", func(t *testing.T) {
		inst := makeInst(map[string]string{
			constant.RestoreSourceKindAnnotationKey: "Backup",
		})
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(inst)

		r := &statusReconciler{}
		r.reconcileRestorePVCAnnotation(tree, inst)

		if _, ok := inst.Annotations[constant.RestorePVCInitialStepCompletedAnnotationKey]; ok {
			t.Error("expected no annotation when PVC not found")
		}
	})

	t.Run("restore VCT with PVC populating succeed - annotation set", func(t *testing.T) {
		inst := makeInst(map[string]string{
			constant.RestoreSourceKindAnnotationKey: "Backup",
		})
		pvc := makePVC("data-test-0", []corev1.PersistentVolumeClaimCondition{
			{
				Type:   corev1.PersistentVolumeClaimConditionType(constant.DataProtectionPVCConditionPopulating),
				Status: corev1.ConditionTrue,
				Reason: constant.DataProtectionPVCConditionReasonPopulatingSucceed,
			},
		})
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(inst)
		if err := tree.Add(pvc); err != nil {
			t.Fatal(err)
		}

		r := &statusReconciler{}
		r.reconcileRestorePVCAnnotation(tree, inst)

		if inst.Annotations[constant.RestorePVCInitialStepCompletedAnnotationKey] != "true" {
			t.Error("expected annotation to be set when PVC populating succeeded")
		}
	})

	t.Run("restore VCT with PVC no conditions - no annotation", func(t *testing.T) {
		inst := makeInst(map[string]string{
			constant.RestoreSourceKindAnnotationKey: "Backup",
		})
		pvc := makePVC("data-test-0", nil)
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(inst)
		if err := tree.Add(pvc); err != nil {
			t.Fatal(err)
		}

		r := &statusReconciler{}
		r.reconcileRestorePVCAnnotation(tree, inst)

		if _, ok := inst.Annotations[constant.RestorePVCInitialStepCompletedAnnotationKey]; ok {
			t.Error("expected no annotation when PVC has no populating condition")
		}
	})

	t.Run("annotation removed when restore completes", func(t *testing.T) {
		inst := makeInst(nil)
		inst.Annotations = map[string]string{
			constant.RestorePVCInitialStepCompletedAnnotationKey: "true",
		}
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(inst)

		r := &statusReconciler{}
		r.reconcileRestorePVCAnnotation(tree, inst)

		if _, ok := inst.Annotations[constant.RestorePVCInitialStepCompletedAnnotationKey]; ok {
			t.Error("expected annotation to be removed when no restore VCTs")
		}
	})
}
