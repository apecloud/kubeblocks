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

package instanceset2

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func shouldCloneInstanceAssistantObjects(its *workloads.InstanceSet) bool {
	return multicluster.Enabled4Object(its)
}

func loadInstanceAssistantObjects(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return nil
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	if shouldCloneInstanceAssistantObjects(its) {
		for _, objRef := range its.Spec.InstanceAssistantObjects {
			obj, err := loadInstanceAssistantObject(ctx, reader, objRef)
			if err != nil {
				return err
			}
			if obj != nil {
				if err = tree.AddWithOption(obj, kubebuilderx.SkipToReconcile(true)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadInstanceAssistantObject(ctx context.Context, reader client.Reader, objRef corev1.ObjectReference) (client.Object, error) {
	obj, err := objectReferenceToObject(objRef)
	if err != nil {
		return nil, err
	}
	if err = reader.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return obj, nil
}

func cloneInstanceAssistantObjects(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) ([]workloads.InstanceAssistantObject, error) {
	objs := make([]workloads.InstanceAssistantObject, 0)
	for _, objRef := range its.Spec.InstanceAssistantObjects {
		obj, err := cloneInstanceAssistantObject(tree, objRef)
		if err != nil {
			return nil, err
		}
		if obj != nil {
			objs = append(objs, instanceAssistantObject(obj))
		}
	}
	return objs, nil
}

func cloneInstanceAssistantObject(tree *kubebuilderx.ObjectTree, objRef corev1.ObjectReference) (client.Object, error) {
	obj, err := objectReferenceToObject(objRef)
	if err != nil {
		return nil, err
	}
	return tree.Get(obj)
}

func objectReferenceToObject(objRef corev1.ObjectReference) (client.Object, error) {
	meta := metav1.ObjectMeta{
		Namespace: objRef.Namespace,
		Name:      objRef.Name,
	}
	switch objRef.Kind {
	case objectKind(&corev1.Service{}):
		return &corev1.Service{ObjectMeta: meta}, nil
	case objectKind(&corev1.ConfigMap{}):
		return &corev1.ConfigMap{ObjectMeta: meta}, nil
	case objectKind(&corev1.Secret{}):
		return &corev1.Secret{ObjectMeta: meta}, nil
	case objectKind(&corev1.ServiceAccount{}):
		return &corev1.ServiceAccount{ObjectMeta: meta}, nil
	case objectKind(&rbacv1.Role{}):
		return &rbacv1.Role{ObjectMeta: meta}, nil
	case objectKind(&rbacv1.RoleBinding{}):
		return &rbacv1.RoleBinding{ObjectMeta: meta}, nil
	default:
		return nil, fmt.Errorf("unknown assistant object: %s", objRef.String())
	}
}

func objectKind(obj client.Object) string {
	gvk, _ := apiutil.GVKForObject(obj, model.GetScheme())
	return gvk.Kind
}

func instanceAssistantObject(obj client.Object) workloads.InstanceAssistantObject {
	objectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Namespace:   obj.GetNamespace(),
			Name:        obj.GetName(),
			Labels:      obj.GetLabels(),
			Annotations: obj.GetAnnotations(),
		}
	}
	if service, ok := obj.(*corev1.Service); ok {
		spec := service.Spec.DeepCopy()
		spec.ClusterIP = ""
		spec.ClusterIPs = nil
		return workloads.InstanceAssistantObject{
			Service: &corev1.Service{
				ObjectMeta: objectMeta(),
				Spec:       *spec,
			},
		}
	}
	if cm, ok := obj.(*corev1.ConfigMap); ok {
		return workloads.InstanceAssistantObject{
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: objectMeta(),
				Data:       cm.Data,
			},
		}
	}
	if secret, ok := obj.(*corev1.Secret); ok {
		return workloads.InstanceAssistantObject{
			Secret: &corev1.Secret{
				ObjectMeta: objectMeta(),
				Data:       secret.Data,
			},
		}
	}
	if sa, ok := obj.(*corev1.ServiceAccount); ok {
		return workloads.InstanceAssistantObject{
			ServiceAccount: &corev1.ServiceAccount{
				ObjectMeta: objectMeta(),
				Secrets:    sa.Secrets,
			},
		}
	}
	if role, ok := obj.(*rbacv1.Role); ok {
		return workloads.InstanceAssistantObject{
			Role: &rbacv1.Role{
				ObjectMeta: objectMeta(),
				Rules:      role.Rules,
			},
		}
	}
	return workloads.InstanceAssistantObject{
		RoleBinding: &rbacv1.RoleBinding{
			ObjectMeta: objectMeta(),
			Subjects:   obj.(*rbacv1.RoleBinding).Subjects,
			RoleRef:    obj.(*rbacv1.RoleBinding).RoleRef,
		},
	}
}
