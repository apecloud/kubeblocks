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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
	for _, obj := range inst.Spec.InstanceAssistantObjects {
		if err := r.createOrUpdate(tree, inst, obj); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	return kubebuilderx.Continue, nil
}

func (r *assistantObjectReconciler) createOrUpdate(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, assistantObj workloads.InstanceAssistantObject) error {
	obj := r.checkObjectProvisionPolicy(inst, r.instanceAssistantObject(assistantObj))
	if obj == nil {
		return nil // skip the object
	}
	r.withInstAnnotationsNLabels(inst, obj)

	robj, err := tree.Get(obj)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err != nil || robj == nil {
		return r.create(tree, inst, obj)
	}
	return r.update(tree, assistantObj, robj, obj)
}

func (r *assistantObjectReconciler) instanceAssistantObject(obj workloads.InstanceAssistantObject) client.Object {
	if obj.Service != nil {
		return obj.Service
	}
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

func (r *assistantObjectReconciler) checkObjectProvisionPolicy(inst *workloads.Instance, obj client.Object) client.Object {
	var policy string
	if obj.GetAnnotations() != nil {
		policy = obj.GetAnnotations()[constant.KBAppMultiClusterObjectProvisionPolicyKey]
	}
	if policy != "ordinal" { // HACK
		return obj
	}

	ordinal := func() int {
		subs := strings.Split(inst.GetName(), "-")
		o, _ := strconv.Atoi(subs[len(subs)-1])
		return o
	}
	if strings.HasSuffix(obj.GetName(), fmt.Sprintf("-%d", ordinal())) {
		return obj
	}
	return nil
}

func (r *assistantObjectReconciler) withInstAnnotationsNLabels(inst *workloads.Instance, obj client.Object) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[constant.KubeBlocksGenerationKey] = inst.Annotations[constant.KubeBlocksGenerationKey]
	obj.SetAnnotations(annotations)

	labels := obj.GetLabels()
	if labels == nil {
		labels = getMatchLabels(inst.Name)
	} else {
		maps.Copy(labels, getMatchLabels(inst.Name))
	}
	obj.SetLabels(labels)
}

func (r *assistantObjectReconciler) create(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, obj client.Object) error {
	// TODO: shared assistant objects
	// if err := controllerutil.SetControllerReference(inst, obj, model.GetScheme()); err != nil {
	//	return err
	// }
	tree.Logger.Info("create object", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName(), "labels", obj.GetLabels())
	return tree.Add(obj)
}

func (r *assistantObjectReconciler) update(tree *kubebuilderx.ObjectTree, assistantObj workloads.InstanceAssistantObject, robj, obj client.Object) error {
	ng, og := r.generation(obj), r.generation(robj)
	if ng > 0 && og > 0 && ng < og {
		tree.Logger.Info("skip update object", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName(), "labels", obj.GetLabels())
		return nil
	}
	merged := r.copyAndMerge(assistantObj, robj, obj)
	if merged == nil {
		return nil
	}
	tree.Logger.Info("update object", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName(), "labels", obj.GetLabels())
	return tree.Update(merged)
}

func (r *assistantObjectReconciler) generation(obj client.Object) int64 {
	g := int64(-1)
	s := obj.GetAnnotations()[constant.KubeBlocksGenerationKey]
	if len(s) > 0 {
		g, _ = strconv.ParseInt(s, 10, 64)
	}
	return g
}

func (r *assistantObjectReconciler) copyAndMerge(obj workloads.InstanceAssistantObject, oldObj, newObj client.Object) client.Object {
	service := func() client.Object {
		return r.copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				o1 := o.(*corev1.Service)
				n1 := n.(*corev1.Service)
				return reflect.DeepEqual(o1.Spec.Selector, n1.Spec.Selector) &&
					reflect.DeepEqual(o1.Spec.Type, n1.Spec.Type) &&
					reflect.DeepEqual(o1.Spec.PublishNotReadyAddresses, n1.Spec.PublishNotReadyAddresses) &&
					reflect.DeepEqual(o1.Spec.Ports, n1.Spec.Ports)
			},
			func(o, n client.Object) {
				o1 := o.(*corev1.Service)
				n1 := n.(*corev1.Service)
				o1.Spec.Selector = n1.Spec.Selector
				o1.Spec.Type = n1.Spec.Type
				o1.Spec.PublishNotReadyAddresses = n1.Spec.PublishNotReadyAddresses
				o1.Spec.Ports = n1.Spec.Ports
			})
	}
	cm := func() client.Object {
		return r.copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*corev1.ConfigMap).Data, n.(*corev1.ConfigMap).Data)
			},
			func(o, n client.Object) {
				o.(*corev1.ConfigMap).Data = n.(*corev1.ConfigMap).Data
			})
	}
	secret := func() client.Object {
		return r.copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*corev1.Secret).Data, n.(*corev1.Secret).Data)
			},
			func(o, n client.Object) {
				o.(*corev1.Secret).Data = n.(*corev1.Secret).Data
			})
	}
	sa := func() client.Object {
		return r.copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*corev1.ServiceAccount).Secrets, n.(*corev1.ServiceAccount).Secrets)
			},
			func(o, n client.Object) {
				o.(*corev1.ServiceAccount).Secrets = n.(*corev1.ServiceAccount).Secrets
			})
	}
	role := func() client.Object {
		return r.copyAndMergeAssistantObject(oldObj, newObj,
			func(o, n client.Object) bool {
				return reflect.DeepEqual(o.(*rbacv1.Role).Rules, n.(*rbacv1.Role).Rules)
			},
			func(o, n client.Object) {
				o.(*rbacv1.Role).Rules = n.(*rbacv1.Role).Rules
			})
	}
	roleBinding := func() client.Object {
		return r.copyAndMergeAssistantObject(oldObj, newObj,
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
	if obj.Service != nil {
		return service()
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

func (r *assistantObjectReconciler) copyAndMergeAssistantObject(oldObj, newObj client.Object, equal func(o, n client.Object) bool, set func(o, n client.Object)) client.Object {
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
