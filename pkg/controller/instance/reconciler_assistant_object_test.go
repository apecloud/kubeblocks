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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

const (
	testNamespace = "default"
	testCluster   = "cluster"
	testComponent = "component"
)

func TestSharedAssistantObjectCreateDoesNotSetInstanceOwnership(t *testing.T) {
	inst := newTestInstance("cluster-component-0", sharedConfigMapAssistantObject("cluster-component-env", "v1"))
	tree := newTestTree(inst)

	reconciler := NewAssistantObjectReconciler().(*assistantObjectReconciler)
	if err := reconciler.createOrUpdate(tree, inst, inst.Spec.InstanceAssistantObjects[0]); err != nil {
		t.Fatalf("createOrUpdate() error = %v", err)
	}

	obj, err := tree.Get(sharedConfigMap("cluster-component-env", ""))
	if err != nil {
		t.Fatalf("tree.Get() error = %v", err)
	}
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		t.Fatalf("expected shared ConfigMap to be planned, got %T", obj)
	}
	if len(cm.OwnerReferences) != 0 {
		t.Fatalf("expected shared ConfigMap to have no Instance owner, got %#v", cm.OwnerReferences)
	}
	if _, ok := cm.Labels[constant.KBAppInstanceNameLabelKey]; ok {
		t.Fatalf("expected shared ConfigMap to have no Instance label, got labels %#v", cm.Labels)
	}
	if cm.Labels[constant.AppManagedByLabelKey] != constant.AppName ||
		cm.Labels[constant.AppInstanceLabelKey] != testCluster ||
		cm.Labels[constant.KBAppComponentLabelKey] != testComponent {
		t.Fatalf("expected KB managed component labels, got %#v", cm.Labels)
	}
	if cm.Annotations[assistantObjectAnnotationKey] != "true" {
		t.Fatalf("expected assistant object annotation, got %#v", cm.Annotations)
	}
}

func TestSharedAssistantObjectUpdatesExistingManagedObject(t *testing.T) {
	existing := sharedConfigMap("cluster-component-env", "old")
	existing.Labels = map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    testCluster,
		constant.KBAppComponentLabelKey: testComponent,
	}
	existing.Annotations = map[string]string{
		assistantObjectAnnotationKey: "true",
	}
	inst := newTestInstance("cluster-component-1", sharedConfigMapAssistantObject("cluster-component-env", "new"))
	tree := newTestTree(inst)
	if err := tree.Add(existing); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	reconciler := NewAssistantObjectReconciler().(*assistantObjectReconciler)
	if err := reconciler.createOrUpdate(tree, inst, inst.Spec.InstanceAssistantObjects[0]); err != nil {
		t.Fatalf("createOrUpdate() error = %v", err)
	}

	obj, err := tree.Get(sharedConfigMap("cluster-component-env", ""))
	if err != nil {
		t.Fatalf("tree.Get() error = %v", err)
	}
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		t.Fatalf("expected shared ConfigMap to be planned, got %T", obj)
	}
	if cm.Data["k"] != "new" {
		t.Fatalf("expected shared ConfigMap data to be patched, got %#v", cm.Data)
	}
	if len(cm.OwnerReferences) != 0 {
		t.Fatalf("expected shared ConfigMap to remain ownerless, got %#v", cm.OwnerReferences)
	}
	if cm.Annotations[assistantObjectAnnotationKey] != "true" {
		t.Fatalf("expected shared ConfigMap to be marked, got annotations %#v", cm.Annotations)
	}
}

func TestDeletionReconcilerDeletesUnreferencedManagedSharedAssistantObject(t *testing.T) {
	existing := managedSharedConfigMap("cluster-component-env")
	inst := newTestInstance("cluster-component-0", sharedConfigMapAssistantObject("cluster-component-env", "v1"))
	cli := newFakeClient(t, existing, inst)
	tree := newTestTree(inst)
	if err := tree.Add(existing); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	reconciler := NewDeletionReconciler(cli).(*deletionReconciler)
	if err := reconciler.deleteUnreferencedSharedAssistantObjects(tree, inst); err != nil {
		t.Fatalf("deleteUnreferencedSharedAssistantObjects() error = %v", err)
	}

	obj, err := tree.Get(sharedConfigMap("cluster-component-env", ""))
	if err != nil {
		t.Fatalf("tree.Get() error = %v", err)
	}
	if obj != nil {
		t.Fatalf("expected unreferenced shared ConfigMap to be removed from desired tree")
	}
}

func TestDeletionReconcilerKeepsSharedAssistantObjectReferencedByAnotherInstance(t *testing.T) {
	existing := managedSharedConfigMap("cluster-component-env")
	inst0 := newTestInstance("cluster-component-0", sharedConfigMapAssistantObject("cluster-component-env", "v1"))
	inst1 := newTestInstance("cluster-component-1", sharedConfigMapAssistantObject("cluster-component-env", "v1"))
	cli := newFakeClient(t, existing, inst0, inst1)
	tree := newTestTree(inst0)
	if err := tree.Add(existing); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	reconciler := NewDeletionReconciler(cli).(*deletionReconciler)
	if err := reconciler.deleteUnreferencedSharedAssistantObjects(tree, inst0); err != nil {
		t.Fatalf("deleteUnreferencedSharedAssistantObjects() error = %v", err)
	}

	obj, err := tree.Get(sharedConfigMap("cluster-component-env", ""))
	if err != nil {
		t.Fatalf("tree.Get() error = %v", err)
	}
	if obj == nil {
		t.Fatalf("expected referenced shared ConfigMap to remain in desired tree")
	}
}

func TestDeletionReconcilerDeletesOnlyCurrentOrdinalAssistantObjectAsSecondaryObject(t *testing.T) {
	shared := managedSharedConfigMap("cluster-component-env")
	currentOrdinal := ordinalConfigMap("cluster-component-0")
	otherOrdinal := ordinalConfigMap("cluster-component-1")
	inst := newTestInstance("cluster-component-0",
		sharedConfigMapAssistantObject(shared.Name, "v1"),
		ordinalConfigMapAssistantObject(currentOrdinal.Name),
		ordinalConfigMapAssistantObject(otherOrdinal.Name))
	tree := newTestTree(inst)
	for _, obj := range []client.Object{shared, currentOrdinal, otherOrdinal} {
		if err := tree.Add(obj); err != nil {
			t.Fatalf("tree.Add() error = %v", err)
		}
	}

	reconciler := NewDeletionReconciler(newFakeClient(t, inst)).(*deletionReconciler)
	if _, err := reconciler.deleteSecondaryObjects(tree, inst, false); err != nil {
		t.Fatalf("deleteSecondaryObjects() error = %v", err)
	}

	if obj, err := tree.Get(sharedConfigMap(shared.Name, "")); err != nil || obj == nil {
		t.Fatalf("expected shared ConfigMap to stay in desired tree, obj = %v, err = %v", obj, err)
	}
	if obj, err := tree.Get(ordinalConfigMap(otherOrdinal.Name)); err != nil || obj == nil {
		t.Fatalf("expected other ordinal ConfigMap to stay in desired tree, obj = %v, err = %v", obj, err)
	}
	if obj, err := tree.Get(ordinalConfigMap(currentOrdinal.Name)); err != nil || obj != nil {
		t.Fatalf("expected current ordinal ConfigMap to be deleted from desired tree, obj = %v, err = %v", obj, err)
	}
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func newTestTree(inst *workloads.Instance) *kubebuilderx.ObjectTree {
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	return tree
}

func newTestInstance(name string, assistantObjs ...workloads.InstanceAssistantObject) *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    testCluster,
				constant.KBAppComponentLabelKey: testComponent,
			},
		},
		Spec: workloads.InstanceSpec{
			InstanceAssistantObjects: assistantObjs,
		},
	}
}

func sharedConfigMapAssistantObject(name, value string) workloads.InstanceAssistantObject {
	return workloads.InstanceAssistantObject{
		ConfigMap: sharedConfigMap(name, value),
	}
}

func ordinalConfigMapAssistantObject(name string) workloads.InstanceAssistantObject {
	return workloads.InstanceAssistantObject{
		ConfigMap: ordinalConfigMap(name),
	}
}

func sharedConfigMap(name, value string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Data: map[string]string{
			"k": value,
		},
	}
}

func managedSharedConfigMap(name string) *corev1.ConfigMap {
	cm := sharedConfigMap(name, "v1")
	cm.Labels = map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    testCluster,
		constant.KBAppComponentLabelKey: testComponent,
	}
	cm.Annotations = map[string]string{
		assistantObjectAnnotationKey: "true",
	}
	return cm
}

func TestInstanceAssistantObject(t *testing.T) {
	t.Run("service", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{Service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc1"}}}
		result, ok := instanceAssistantObject(obj)
		assert.True(t, ok)
		assert.Equal(t, "svc1", result.GetName())
	})

	t.Run("configmap", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}}
		result, ok := instanceAssistantObject(obj)
		assert.True(t, ok)
		assert.Equal(t, "cm1", result.GetName())
	})

	t.Run("secret", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{Secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec1"}}}
		result, ok := instanceAssistantObject(obj)
		assert.True(t, ok)
		assert.Equal(t, "sec1", result.GetName())
	})

	t.Run("serviceaccount", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{ServiceAccount: &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa1"}}}
		result, ok := instanceAssistantObject(obj)
		assert.True(t, ok)
		assert.Equal(t, "sa1", result.GetName())
	})

	t.Run("role", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{Role: &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role1"}}}
		result, ok := instanceAssistantObject(obj)
		assert.True(t, ok)
		assert.Equal(t, "role1", result.GetName())
	})

	t.Run("rolebinding", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{RoleBinding: &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb1"}}}
		result, ok := instanceAssistantObject(obj)
		assert.True(t, ok)
		assert.Equal(t, "rb1", result.GetName())
	})

	t.Run("empty", func(t *testing.T) {
		obj := workloads.InstanceAssistantObject{}
		_, ok := instanceAssistantObject(obj)
		assert.False(t, ok)
	})
}

func TestCopyAndMergeAssistantObject_NoChange(t *testing.T) {
	old := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "cm1",
			Labels:      map[string]string{"l": "v"},
			Annotations: map[string]string{"a": "v"},
		},
		Data: map[string]string{"key": "value"},
	}
	new := old.DeepCopy()

	result := copyAndMergeAssistantObject(old, new,
		func(o, n client.Object) bool {
			return o.(*corev1.ConfigMap).Data["key"] == n.(*corev1.ConfigMap).Data["key"]
		},
		func(o, n client.Object) {
			o.(*corev1.ConfigMap).Data = n.(*corev1.ConfigMap).Data
		})
	assert.Nil(t, result) // no change
}

func TestCopyAndMergeAssistantObject_DataChanged(t *testing.T) {
	old := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "cm1",
			Labels:      map[string]string{"l": "v"},
			Annotations: map[string]string{"a": "v"},
		},
		Data: map[string]string{"key": "old"},
	}
	new := old.DeepCopy()
	new.Data["key"] = "new"

	result := copyAndMergeAssistantObject(old, new,
		func(o, n client.Object) bool {
			return o.(*corev1.ConfigMap).Data["key"] == n.(*corev1.ConfigMap).Data["key"]
		},
		func(o, n client.Object) {
			o.(*corev1.ConfigMap).Data = n.(*corev1.ConfigMap).Data
		})
	assert.NotNil(t, result)
	assert.Equal(t, "new", result.(*corev1.ConfigMap).Data["key"])
}

func TestReconcilerCopyAndMerge_AllTypes(t *testing.T) {
	r := &assistantObjectReconciler{}

	t.Run("service", func(t *testing.T) {
		old := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "svc1"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
		}
		new := old.DeepCopy()
		new.Spec.Ports[0].Port = 8080
		obj := workloads.InstanceAssistantObject{Service: &corev1.Service{}}
		result := r.copyAndMerge(obj, old, new)
		assert.NotNil(t, result)
		assert.Equal(t, int32(8080), result.(*corev1.Service).Spec.Ports[0].Port)
	})

	t.Run("configmap", func(t *testing.T) {
		old := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
			Data:       map[string]string{"key": "old"},
		}
		new := old.DeepCopy()
		new.Data["key"] = "new"
		obj := workloads.InstanceAssistantObject{ConfigMap: &corev1.ConfigMap{}}
		result := r.copyAndMerge(obj, old, new)
		assert.NotNil(t, result)
		assert.Equal(t, "new", result.(*corev1.ConfigMap).Data["key"])
	})

	t.Run("secret", func(t *testing.T) {
		old := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sec1"},
			Data:       map[string][]byte{"key": []byte("old")},
		}
		new := old.DeepCopy()
		new.Data["key"] = []byte("new")
		obj := workloads.InstanceAssistantObject{Secret: &corev1.Secret{}}
		result := r.copyAndMerge(obj, old, new)
		assert.NotNil(t, result)
		assert.Equal(t, []byte("new"), result.(*corev1.Secret).Data["key"])
	})

	t.Run("serviceaccount", func(t *testing.T) {
		old := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "sa1"},
		}
		new := old.DeepCopy()
		new.Labels = map[string]string{"new": "label"}
		obj := workloads.InstanceAssistantObject{ServiceAccount: &corev1.ServiceAccount{}}
		result := r.copyAndMerge(obj, old, new)
		assert.NotNil(t, result)
	})

	t.Run("role", func(t *testing.T) {
		old := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "role1"},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get"}}},
		}
		new := old.DeepCopy()
		new.Rules[0].Verbs = []string{"get", "list"}
		obj := workloads.InstanceAssistantObject{Role: &rbacv1.Role{}}
		result := r.copyAndMerge(obj, old, new)
		assert.NotNil(t, result)
		assert.Len(t, result.(*rbacv1.Role).Rules[0].Verbs, 2)
	})

	t.Run("rolebinding", func(t *testing.T) {
		old := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "rb1"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "sa1"}},
			RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "role1"},
		}
		new := old.DeepCopy()
		new.Subjects[0].Name = "sa2"
		obj := workloads.InstanceAssistantObject{RoleBinding: &rbacv1.RoleBinding{}}
		result := r.copyAndMerge(obj, old, new)
		assert.NotNil(t, result)
		assert.Equal(t, "sa2", result.(*rbacv1.RoleBinding).Subjects[0].Name)
	})
}

func TestIsSharedAssistantObject(t *testing.T) {
	inst := newTestInstance("inst-0")

	t.Run("matches", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cm1",
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    testCluster,
					constant.KBAppComponentLabelKey: testComponent,
				},
				Annotations: map[string]string{
					assistantObjectAnnotationKey: "true",
				},
			},
		}
		assert.True(t, isSharedAssistantObject(obj, inst))
	})

	t.Run("no labels", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		assert.False(t, isSharedAssistantObject(obj, inst))
	})

	t.Run("wrong cluster", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cm1",
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    "other-cluster",
					constant.KBAppComponentLabelKey: testComponent,
				},
				Annotations: map[string]string{
					assistantObjectAnnotationKey: "true",
				},
			},
		}
		assert.False(t, isSharedAssistantObject(obj, inst))
	})
}

func TestIsOrdinalAssistantObject(t *testing.T) {
	t.Run("no annotations", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		assert.False(t, isOrdinalAssistantObject(obj))
	})

	t.Run("ordinal annotation", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cm1",
				Annotations: map[string]string{
					constant.KBAppMultiClusterObjectProvisionPolicyKey: constant.KBAppMultiClusterObjectProvisionOrdinal,
				},
			},
		}
		assert.True(t, isOrdinalAssistantObject(obj))
	})
}

func TestIsCurrentInstanceOrdinalAssistantObject(t *testing.T) {
	inst := newTestInstance("cluster-component-0")

	t.Run("matching ordinal", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "env-0"}}
		assert.True(t, isCurrentInstanceOrdinalAssistantObject(inst, obj))
	})

	t.Run("different ordinal", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "env-1"}}
		assert.False(t, isCurrentInstanceOrdinalAssistantObject(inst, obj))
	})
}

func ordinalConfigMap(name string) *corev1.ConfigMap {
	cm := sharedConfigMap(name, "v1")
	cm.Annotations = map[string]string{
		constant.KBAppMultiClusterObjectProvisionPolicyKey: constant.KBAppMultiClusterObjectProvisionOrdinal,
	}
	return cm
}
