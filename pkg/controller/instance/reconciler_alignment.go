/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewAlignmentReconciler() kubebuilderx.Reconciler {
	return &alignmentReconciler{}
}

type alignmentReconciler struct{}

var _ kubebuilderx.Reconciler = &alignmentReconciler{}

func (r *alignmentReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *alignmentReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst := tree.GetRoot().(*workloads.Instance)

	newNameSet := sets.New[string](podName(inst))
	obj, err := tree.Get(podObj(inst))
	if err != nil {
		return kubebuilderx.Continue, err
	}

	oldNameSet := sets.New[string]()
	oldPodList := tree.List(&corev1.Pod{})
	oldPVCList := tree.List(&corev1.PersistentVolumeClaim{})
	for _, object := range oldPodList {
		oldNameSet.Insert(object.GetName())
	}
	deleteNameSet := oldNameSet.Difference(newNameSet)

	if obj == nil {
		// create pod
		newPod, err := buildInstancePod(inst, "")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := tree.Add(newPod); err != nil {
			return kubebuilderx.Continue, err
		}

		// create PVCs
		pvcs, err := buildInstancePVCs(inst)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		for _, pvc := range pvcs {
			switch oldPvc, err := tree.Get(pvc); {
			case err != nil:
				return kubebuilderx.Continue, err
			case oldPvc == nil:
				if err = tree.Add(pvc); err != nil {
					return kubebuilderx.Continue, err
				}
			default:
				pvcObj := copyAndMerge(oldPvc, pvc)
				if pvcObj != nil {
					if err = tree.Update(pvcObj); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}
	}

	// delete useless pod & PVCs
	for _, object := range oldPodList {
		pod, _ := object.(*corev1.Pod)
		if _, ok := deleteNameSet[pod.Name]; !ok {
			continue
		}
		if err = tree.Delete(pod); err != nil {
			return kubebuilderx.Continue, err
		}

		retentionPolicy := inst.Spec.PersistentVolumeClaimRetentionPolicy
		// the default policy is `Delete`
		if retentionPolicy == nil || retentionPolicy.WhenScaled != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
			for _, pobj := range oldPVCList {
				pvc := pobj.(*corev1.PersistentVolumeClaim)
				if pvc.Labels != nil && pvc.Labels[constant.KBAppPodNameLabelKey] == pod.Name {
					if err = tree.Delete(pvc); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}
	}

	return kubebuilderx.Continue, nil
}
