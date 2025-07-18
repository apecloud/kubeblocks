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
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewAssistantObjectReconciler() kubebuilderx.Reconciler {
	return &assistantObjectReconciler{}
}

type assistantObjectReconciler struct{}

var _ kubebuilderx.Reconciler = &assistantObjectReconciler{}

func (r *assistantObjectReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *assistantObjectReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst := tree.GetRoot().(*workloads.Instance)
	for _, obj := range inst.Spec.AssistantObjects {
		if err := r.createOrUpdate(tree, inst, r.instanceAssistantObject(obj)); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	return kubebuilderx.Continue, nil
}

func (r *assistantObjectReconciler) createOrUpdate(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, obj client.Object) error {
	robj, err := tree.Get(obj)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err != nil || robj == nil {
		labels := obj.GetLabels()
		maps.Copy(labels, getMatchLabels(inst.Name))
		obj.SetLabels(labels)
		if err := controllerutil.SetControllerReference(inst, obj, model.GetScheme()); err != nil {
			return err
		}
		return tree.Add(obj)
	}
	r.mergeAssistantObjectMeta(robj, obj)
	return tree.Update(obj)
}

func (r *assistantObjectReconciler) mergeAssistantObjectMeta(oldObj, newObj client.Object) {
	newObj.SetSelfLink(oldObj.GetSelfLink())
	newObj.SetUID(oldObj.GetUID())
	newObj.SetResourceVersion(oldObj.GetResourceVersion())
	newObj.SetGeneration(oldObj.GetGeneration())
	newObj.SetCreationTimestamp(oldObj.GetCreationTimestamp())
	newObj.SetDeletionTimestamp(oldObj.GetDeletionTimestamp())
	newObj.SetDeletionGracePeriodSeconds(oldObj.GetDeletionGracePeriodSeconds())
	newObj.SetOwnerReferences(oldObj.GetOwnerReferences())
	newObj.SetFinalizers(oldObj.GetFinalizers())
	newObj.SetManagedFields(oldObj.GetManagedFields())
	// TODO: merge labels & annotations
}

func (r *assistantObjectReconciler) instanceAssistantObject(obj workloads.InstanceAssistantObject) client.Object {
	if obj.ConfigMap != nil {
		return obj.ConfigMap
	}
	if obj.Secret != nil {
		return obj.Secret
	}
	if obj.Service != nil {
		return obj.Service
	}
	if obj.ServiceAccount != nil {
		return obj.ServiceAccount
	}
	if obj.Role != nil {
		return obj.Role
	}
	return obj.RoleBinding
}
