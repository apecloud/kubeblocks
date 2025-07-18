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
	"reflect"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
		_, err := r.createOrUpdate(tree, inst, obj)
		if err != nil {
			return kubebuilderx.Continue, err
		}
	}
	return kubebuilderx.Continue, nil
}

func (r *assistantObjectReconciler) createOrUpdate(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, assistantObj workloads.InstanceAssistantObject) (bool, error) {
	obj := r.instanceAssistantObject(assistantObj)
	robj, err := tree.Get(obj)
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	}
	if err != nil || robj == nil {
		labels := obj.GetLabels()
		maps.Copy(labels, getMatchLabels(inst.Name))
		obj.SetLabels(labels)
		if err := controllerutil.SetControllerReference(inst, obj, model.GetScheme()); err != nil {
			return false, err
		}
		return true, tree.Add(obj)
	}
	if merged := r.copyAndMerge(assistantObj, robj, obj); merged != nil {
		return true, tree.Update(merged)
	}
	return false, nil
}

func (r *assistantObjectReconciler) instanceAssistantObject(obj workloads.InstanceAssistantObject) client.Object {
	if obj.ConfigMap != nil {
		return obj.ConfigMap
	}
	if obj.Secret != nil {
		return obj.Secret
	}
	if obj.ServiceAccount != nil {
		return obj.ServiceAccount
	}
	if obj.Role != nil {
		return obj.Role
	}
	return obj.RoleBinding
}

func (r *assistantObjectReconciler) copyAndMerge(obj workloads.InstanceAssistantObject, oldObj, newObj client.Object) client.Object {
	cm := func() client.Object {
		return copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*corev1.ConfigMap).Data, n.(*corev1.ConfigMap).Data)
			},
			func(o, n client.Object) {
				o.(*corev1.ConfigMap).Data = n.(*corev1.ConfigMap).Data
			})
	}
	secret := func() client.Object {
		return copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*corev1.Secret).Data, n.(*corev1.Secret).Data)
			},
			func(o, n client.Object) {
				o.(*corev1.Secret).Data = n.(*corev1.Secret).Data
			})
	}
	sa := func() client.Object {
		return copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*corev1.ServiceAccount).Secrets, n.(*corev1.ServiceAccount).Secrets)
			},
			func(o, n client.Object) {
				o.(*corev1.ServiceAccount).Secrets = n.(*corev1.ServiceAccount).Secrets
			})
	}
	role := func() client.Object {
		return copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*rbacv1.Role).Rules, n.(*rbacv1.Role).Rules)
			},
			func(o, n client.Object) {
				o.(*rbacv1.Role).Rules = n.(*rbacv1.Role).Rules
			})
	}
	roleBinding := func() client.Object {
		return copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				o1 := o.(*rbacv1.RoleBinding)
				n1 := n.(*rbacv1.RoleBinding)
				return reflect.DeepEqual(o1.Subjects, n1.Subjects) && reflect.DeepEqual(o1.RoleRef, n1.RoleRef)
			},
			func(o, n client.Object) {
				o1 := o.(*rbacv1.RoleBinding)
				n1 := n.(*rbacv1.RoleBinding)
				o1.Subjects = n1.Subjects
				o1.RoleRef = n1.RoleRef
			})
	}
	if obj.ConfigMap != nil {
		return cm()
	}
	if obj.Secret != nil {
		return secret()
	}
	if obj.ServiceAccount != nil {
		return sa()
	}
	if obj.Role != nil {
		return role()
	}
	return roleBinding()
}

func copyAndMergeAssistantObject(oldObj, newObj client.Object, equal func(o, n client.Object) bool, set func(o, n client.Object)) client.Object {
	if reflect.DeepEqual(oldObj.GetLabels(), newObj.GetLabels()) &&
		reflect.DeepEqual(oldObj.GetAnnotations(), newObj.GetAnnotations()) &&
		equal(oldObj, newObj) {
		return nil
	}
	objCopy := oldObj.DeepCopyObject().(client.Object)
	objCopy.SetLabels(newObj.GetLabels())
	objCopy.SetAnnotations(newObj.GetAnnotations())
	set(objCopy, newObj)
	return objCopy
}
