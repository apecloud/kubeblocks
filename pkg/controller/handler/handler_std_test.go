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

package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func testScheme() runtime.Scheme {
	return *model.GetScheme()
}

func testFinderContext() *FinderContext {
	return &FinderContext{
		Context: context.Background(),
		Scheme:  testScheme(),
	}
}

// --- LabelFinder ---

func TestNewLabelFinder(t *testing.T) {
	finder := NewLabelFinder(&workloads.InstanceSet{}, "managed-by", "kubeblocks", "parent-name")
	require.NotNil(t, finder)
}

func TestLabelFinder_Find_NoLabels(t *testing.T) {
	finder := NewLabelFinder(&workloads.InstanceSet{}, "managed-by", "kubeblocks", "parent-name")
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns"}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestLabelFinder_Find_ManagedByMissing(t *testing.T) {
	finder := NewLabelFinder(&workloads.InstanceSet{}, "managed-by", "kubeblocks", "parent-name")
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		Labels:    map[string]string{"other": "val"},
	}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestLabelFinder_Find_ManagedByWrongValue(t *testing.T) {
	finder := NewLabelFinder(&workloads.InstanceSet{}, "managed-by", "kubeblocks", "parent-name")
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		Labels:    map[string]string{"managed-by": "other"},
	}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestLabelFinder_Find_ParentNameMissing(t *testing.T) {
	finder := NewLabelFinder(&workloads.InstanceSet{}, "managed-by", "kubeblocks", "parent-name")
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		Labels:    map[string]string{"managed-by": "kubeblocks"},
	}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestLabelFinder_Find_Success(t *testing.T) {
	finder := NewLabelFinder(&workloads.InstanceSet{}, "managed-by", "kubeblocks", "parent-name")
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		Labels:    map[string]string{"managed-by": "kubeblocks", "parent-name": "my-its"},
	}}
	result := finder.Find(ctx, obj)
	require.NotNil(t, result)
	assert.Equal(t, "my-its", result.ObjectKey.Name)
	assert.Equal(t, "ns", result.ObjectKey.Namespace)
}

// --- InvolvedObjectFinder ---

func TestInvolvedObjectFinder_Find_NotAnEvent(t *testing.T) {
	finder := NewInvolvedObjectFinder(&corev1.Pod{})
	ctx := testFinderContext()
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestInvolvedObjectFinder_Find_BadAPIVersion(t *testing.T) {
	finder := NewInvolvedObjectFinder(&corev1.Pod{})
	ctx := testFinderContext()
	evt := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "not-a-valid-group-version///",
			Kind:       "Pod",
			Name:       "pod-0",
			Namespace:  "ns",
		},
	}
	result := finder.Find(ctx, evt)
	assert.Nil(t, result)
}

func TestInvolvedObjectFinder_Find_GVKMismatch(t *testing.T) {
	finder := NewInvolvedObjectFinder(&corev1.Pod{})
	ctx := testFinderContext()
	evt := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
			Name:       "sts-0",
			Namespace:  "ns",
		},
	}
	result := finder.Find(ctx, evt)
	assert.Nil(t, result)
}

func TestInvolvedObjectFinder_Find_Success(t *testing.T) {
	finder := NewInvolvedObjectFinder(&corev1.Pod{})
	ctx := testFinderContext()
	evt := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       "pod-0",
			Namespace:  "ns",
		},
	}
	result := finder.Find(ctx, evt)
	require.NotNil(t, result)
	assert.Equal(t, "pod-0", result.ObjectKey.Name)
}

// --- OwnerFinder ---

func TestOwnerFinder_Find_NoOwner(t *testing.T) {
	finder := NewOwnerFinder(&appsv1.StatefulSet{})
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns"}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestOwnerFinder_Find_BadAPIVersion(t *testing.T) {
	finder := NewOwnerFinder(&appsv1.StatefulSet{})
	ctx := testFinderContext()
	isController := true
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "bad///version",
			Kind:       "StatefulSet",
			Name:       "sts-0",
			Controller: &isController,
		}},
	}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestOwnerFinder_Find_GVKMismatch(t *testing.T) {
	finder := NewOwnerFinder(&appsv1.StatefulSet{})
	ctx := testFinderContext()
	isController := true
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "Service",
			Name:       "svc-0",
			Controller: &isController,
		}},
	}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestOwnerFinder_Find_Success(t *testing.T) {
	finder := NewOwnerFinder(&appsv1.StatefulSet{})
	ctx := testFinderContext()
	isController := true
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
			Name:       "sts-0",
			Controller: &isController,
		}},
	}}
	result := finder.Find(ctx, obj)
	require.NotNil(t, result)
	assert.Equal(t, "sts-0", result.ObjectKey.Name)
}

// --- DelegatorFinder ---

func TestDelegatorFinder_Find_NoLabels(t *testing.T) {
	finder := NewDelegatorFinder(&workloads.InstanceSet{}, []string{"app", "component"})
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns"}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestDelegatorFinder_Find_MissingLabel(t *testing.T) {
	finder := NewDelegatorFinder(&workloads.InstanceSet{}, []string{"app", "component"})
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		Labels:    map[string]string{"app": "cluster"},
	}}
	result := finder.Find(ctx, obj)
	assert.Nil(t, result)
}

func TestDelegatorFinder_Find_Success(t *testing.T) {
	finder := NewDelegatorFinder(&workloads.InstanceSet{}, []string{"app", "component"})
	ctx := testFinderContext()
	obj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-0",
		Namespace: "ns",
		Labels:    map[string]string{"app": "cluster", "component": "mysql"},
	}}
	result := finder.Find(ctx, obj)
	require.NotNil(t, result)
	assert.Equal(t, "cluster-mysql", result.ObjectKey.Name)
	assert.Equal(t, "ns", result.ObjectKey.Namespace)
}

// --- getObjectFromKey ---

func TestGetObjectFromKey_UnknownGVK(t *testing.T) {
	ctx := testFinderContext()
	key := &model.GVKNObjKey{}
	key.GroupVersionKind.Kind = "UnknownKind"
	key.GroupVersionKind.Group = "unknown.group"
	key.GroupVersionKind.Version = "v1"
	result := getObjectFromKey(ctx, key)
	assert.Nil(t, result)
}

// --- getGroupVersionKind ---

func TestGetGroupVersionKind_InvalidType(t *testing.T) {
	bf := &baseFinder{objectType: &runtime.Unknown{}}
	scheme := testScheme()
	result := bf.getGroupVersionKind(&scheme)
	assert.Nil(t, result)
}

// --- Builder edge cases ---

func TestBuilder_Build_NoFinders(t *testing.T) {
	ctx := testFinderContext()
	h := NewBuilder(ctx).Build()
	assert.NotNil(t, h)
}
