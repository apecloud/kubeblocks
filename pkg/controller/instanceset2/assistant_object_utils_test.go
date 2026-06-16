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
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestObjectReferenceToObject(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want client.Object
	}{
		{name: "configmap", kind: "ConfigMap", want: &corev1.ConfigMap{}},
		{name: "secret", kind: "Secret", want: &corev1.Secret{}},
		{name: "service account", kind: "ServiceAccount", want: &corev1.ServiceAccount{}},
		{name: "service", kind: "Service", want: &corev1.Service{}},
		{name: "role", kind: "Role", want: &rbacv1.Role{}},
		{name: "role binding", kind: "RoleBinding", want: &rbacv1.RoleBinding{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := objectReferenceToObject(corev1.ObjectReference{
				Kind:      tt.kind,
				Namespace: "default",
				Name:      "obj",
			})
			if err != nil {
				t.Fatalf("objectReferenceToObject() error = %v", err)
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Fatalf("objectReferenceToObject() type = %T, want %T", got, tt.want)
			}
			if got.GetNamespace() != "default" || got.GetName() != "obj" {
				t.Fatalf("unexpected object key: %s/%s", got.GetNamespace(), got.GetName())
			}
		})
	}
}

func TestObjectReferenceToObjectUnknownKind(t *testing.T) {
	_, err := objectReferenceToObject(corev1.ObjectReference{Kind: "Unsupported", Name: "obj"})
	if err == nil {
		t.Fatal("expected unknown kind error")
	}
}

func TestLoadInstanceAssistantObject(t *testing.T) {
	ctx := context.Background()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "conf",
		},
		Data: map[string]string{"key": "value"},
	}
	reader := fake.NewClientBuilder().
		WithScheme(model.GetScheme()).
		WithObjects(cm).
		Build()

	got, err := loadInstanceAssistantObject(ctx, reader, corev1.ObjectReference{
		Kind:      "ConfigMap",
		Namespace: "default",
		Name:      "conf",
	})
	if err != nil {
		t.Fatalf("loadInstanceAssistantObject() error = %v", err)
	}
	if !reflect.DeepEqual(got.(*corev1.ConfigMap).Data, cm.Data) {
		t.Fatalf("unexpected loaded configmap data: %#v", got.(*corev1.ConfigMap).Data)
	}

	missing, err := loadInstanceAssistantObject(ctx, reader, corev1.ObjectReference{
		Kind:      "ConfigMap",
		Namespace: "default",
		Name:      "missing",
	})
	if err != nil {
		t.Fatalf("loadInstanceAssistantObject() missing error = %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing object, got %T", missing)
	}
}

func TestLoadInstanceAssistantObjects(t *testing.T) {
	ctx := context.Background()
	reader := fake.NewClientBuilder().
		WithScheme(model.GetScheme()).
		WithObjects(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "conf",
			},
		}).
		Build()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(&workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "mysql",
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "cluster-a",
			},
		},
		Spec: workloads.InstanceSetSpec{
			InstanceAssistantObjects: []corev1.ObjectReference{{
				Kind:      "ConfigMap",
				Namespace: "default",
				Name:      "conf",
			}},
		},
	})

	if err := loadInstanceAssistantObjects(ctx, reader, tree); err != nil {
		t.Fatalf("loadInstanceAssistantObjects() error = %v", err)
	}
	if got := tree.List(&corev1.ConfigMap{}); len(got) != 1 {
		t.Fatalf("loaded configmaps = %d, want 1", len(got))
	}

	emptyTree := kubebuilderx.NewObjectTree()
	if err := loadInstanceAssistantObjects(ctx, reader, emptyTree); err != nil {
		t.Fatalf("loadInstanceAssistantObjects() empty tree error = %v", err)
	}
}

func TestCloneInstanceAssistantObjects(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	if err := tree.Add(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "conf",
			Labels:    map[string]string{"app": "mysql"},
		},
		Data: map[string]string{"key": "value"},
	}); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	objs, err := cloneInstanceAssistantObjects(tree, &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			InstanceAssistantObjects: []corev1.ObjectReference{{
				Kind:      "ConfigMap",
				Namespace: "default",
				Name:      "conf",
			}},
		},
	})
	if err != nil {
		t.Fatalf("cloneInstanceAssistantObjects() error = %v", err)
	}
	if len(objs) != 1 || objs[0].ConfigMap == nil {
		t.Fatalf("unexpected cloned objects: %#v", objs)
	}
	if !reflect.DeepEqual(objs[0].ConfigMap.Data, map[string]string{"key": "value"}) {
		t.Fatalf("unexpected cloned configmap data: %#v", objs[0].ConfigMap.Data)
	}
}

func TestInstanceAssistantObject(t *testing.T) {
	meta := metav1.ObjectMeta{
		Namespace:   "default",
		Name:        "obj",
		Labels:      map[string]string{"app": "mysql"},
		Annotations: map[string]string{"owner": "test"},
	}

	if got := instanceAssistantObject(&corev1.Service{
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		},
	}); got.Service == nil || got.Service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("unexpected service assistant object: %#v", got)
	}
	if got := instanceAssistantObject(&corev1.ConfigMap{
		ObjectMeta: meta,
		Data:       map[string]string{"key": "value"},
	}); got.ConfigMap == nil || got.ConfigMap.Data["key"] != "value" {
		t.Fatalf("unexpected configmap assistant object: %#v", got)
	}
	if got := instanceAssistantObject(&corev1.Secret{
		ObjectMeta: meta,
		Data:       map[string][]byte{"key": []byte("value")},
	}); got.Secret == nil || string(got.Secret.Data["key"]) != "value" {
		t.Fatalf("unexpected secret assistant object: %#v", got)
	}
	if got := instanceAssistantObject(&corev1.ServiceAccount{
		ObjectMeta: meta,
		Secrets:    []corev1.ObjectReference{{Name: "token"}},
	}); got.ServiceAccount == nil || got.ServiceAccount.Secrets[0].Name != "token" {
		t.Fatalf("unexpected service account assistant object: %#v", got)
	}
	if got := instanceAssistantObject(&rbacv1.Role{
		ObjectMeta: meta,
		Rules:      []rbacv1.PolicyRule{{Resources: []string{"pods"}}},
	}); got.Role == nil || got.Role.Rules[0].Resources[0] != "pods" {
		t.Fatalf("unexpected role assistant object: %#v", got)
	}
	if got := instanceAssistantObject(&rbacv1.RoleBinding{
		ObjectMeta: meta,
		Subjects:   []rbacv1.Subject{{Name: "sa"}},
		RoleRef:    rbacv1.RoleRef{Name: "role"},
	}); got.RoleBinding == nil || got.RoleBinding.RoleRef.Name != "role" {
		t.Fatalf("unexpected role binding assistant object: %#v", got)
	}
}
