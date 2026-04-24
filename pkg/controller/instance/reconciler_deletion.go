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
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewDeletionReconciler(reader client.Reader) kubebuilderx.Reconciler {
	return &deletionReconciler{
		reader: reader,
	}
}

// deletionReconciler handles object and its secondary resources' deletion
type deletionReconciler struct {
	reader client.Reader
}

var _ kubebuilderx.Reconciler = &deletionReconciler{}

func (r *deletionReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *deletionReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst, _ := tree.GetRoot().(*workloads.Instance)
	if err := r.deleteUnreferencedSharedAssistantObjects(tree, inst); err != nil {
		return kubebuilderx.Continue, err
	}

	pvcRetentionPolicy := inst.Spec.PersistentVolumeClaimRetentionPolicy
	retainPVC := pvcRetentionPolicy != nil && pvcRetentionPolicy.WhenDeleted == appsv1.RetainPersistentVolumeClaimRetentionPolicyType
	if ptr.Deref(inst.Spec.ScaledDown, false) {
		retainPVC = pvcRetentionPolicy != nil && pvcRetentionPolicy.WhenScaled == appsv1.RetainPersistentVolumeClaimRetentionPolicyType
	}

	// delete secondary objects first
	if has, err := r.deleteSecondaryObjects(tree, inst, retainPVC); has {
		return kubebuilderx.Continue, err
	}

	// delete root object
	tree.DeleteRoot()
	return kubebuilderx.Continue, nil
}

func (r *deletionReconciler) deleteSecondaryObjects(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, retainPVC bool) (bool, error) {
	// secondary objects to be deleted
	secondaryObjects := maps.Clone(tree.GetSecondaryObjects())
	for _, assistantObj := range inst.Spec.InstanceAssistantObjects {
		obj, name, err := assistantObjectKey(assistantObj)
		if err != nil {
			return true, err
		}
		if obj == nil {
			continue
		}
		if skipAssistantObjectSecondaryDeletion(inst, obj) {
			delete(secondaryObjects, *name)
		}
	}
	if retainPVC {
		// exclude PVCs from them and clear their owner references
		pvcList := tree.List(&corev1.PersistentVolumeClaim{})
		for _, pvcObj := range pvcList {
			pvc, ok := pvcObj.(*corev1.PersistentVolumeClaim)
			if !ok {
				continue
			}
			// Clear owner references to prevent garbage collection when Instance is deleted
			ownerRefs := pvc.GetOwnerReferences()
			if len(ownerRefs) > 0 {
				// Filter out owner references that belong to this Instance
				filteredRefs := make([]metav1.OwnerReference, 0, len(ownerRefs))
				for _, ref := range ownerRefs {
					if ref.UID != inst.UID {
						filteredRefs = append(filteredRefs, ref)
					}
				}
				if len(filteredRefs) != len(ownerRefs) {
					pvc.SetOwnerReferences(filteredRefs)
					if err := tree.Update(pvc); err != nil {
						return true, err
					}
				}
			}
			name, err := model.GetGVKName(pvc)
			if err != nil {
				return true, err
			}
			delete(secondaryObjects, *name)
		}
	}
	// delete them
	for _, obj := range secondaryObjects {
		if err := tree.Delete(obj); err != nil {
			return true, err
		}
	}
	return len(secondaryObjects) > 0, nil
}

func (r *deletionReconciler) deleteUnreferencedSharedAssistantObjects(tree *kubebuilderx.ObjectTree, inst *workloads.Instance) error {
	instList := &workloads.InstanceList{}
	listOpts := []client.ListOption{
		client.InNamespace(inst.Namespace),
		client.MatchingLabels{
			constant.AppManagedByLabelKey:   constant.AppName,
			constant.AppInstanceLabelKey:    inst.Labels[constant.AppInstanceLabelKey],
			constant.KBAppComponentLabelKey: inst.Labels[constant.KBAppComponentLabelKey],
		},
	}
	if err := r.reader.List(tree.Context, instList, listOpts...); err != nil {
		return err
	}
	for _, assistantObj := range inst.Spec.InstanceAssistantObjects {
		obj, name, err1 := assistantObjectKey(assistantObj)
		if err1 != nil {
			return err1
		}
		if obj == nil || isOrdinalAssistantObject(obj) {
			continue
		}
		existing, err3 := tree.Get(obj)
		if err3 != nil {
			return err3
		}
		if existing == nil {
			continue
		}
		referenced, err2 := sharedAssistantObjectReferencedByOthers(inst, *name, instList.Items)
		if err2 != nil {
			return err2
		}
		if referenced {
			continue
		}
		if !isSharedAssistantObject(existing, inst) {
			continue
		}
		if err4 := tree.Delete(existing); err4 != nil {
			return err4
		}
	}
	return nil
}

func sharedAssistantObjectReferencedByOthers(inst *workloads.Instance, objKey model.GVKNObjKey, instances []workloads.Instance) (bool, error) {
	for i := range instances {
		other := &instances[i]
		if other.Name == inst.Name || other.Namespace != inst.Namespace || !other.DeletionTimestamp.IsZero() {
			continue
		}
		for _, assistantObj := range other.Spec.InstanceAssistantObjects {
			otherObj, otherKey, err := assistantObjectKey(assistantObj)
			if err != nil {
				return false, err
			}
			if otherObj == nil || isOrdinalAssistantObject(otherObj) {
				continue
			}
			if *otherKey == objKey {
				return true, nil
			}
		}
	}
	return false, nil
}

func skipAssistantObjectSecondaryDeletion(inst *workloads.Instance, obj client.Object) bool {
	if !isOrdinalAssistantObject(obj) {
		return true
	}
	return !isCurrentInstanceOrdinalAssistantObject(inst, obj)
}

func assistantObjectKey(assistantObj workloads.InstanceAssistantObject) (client.Object, *model.GVKNObjKey, error) {
	obj, ok := instanceAssistantObject(assistantObj)
	if !ok {
		return nil, nil, nil
	}
	name, err := model.GetGVKName(obj)
	if err != nil {
		return nil, nil, err
	}
	return obj, name, nil
}
