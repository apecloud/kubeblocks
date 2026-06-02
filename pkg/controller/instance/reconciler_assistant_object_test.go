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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
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

func ordinalConfigMap(name string) *corev1.ConfigMap {
	cm := sharedConfigMap(name, "v1")
	cm.Annotations = map[string]string{
		constant.KBAppMultiClusterObjectProvisionPolicyKey: constant.KBAppMultiClusterObjectProvisionOrdinal,
	}
	return cm
}
