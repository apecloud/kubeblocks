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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

func init() {
	AddScheme(workloads.AddToScheme)
}

// --- graph_options.go ---

func TestClientOptionApplyTo(t *testing.T) {
	opt := WithClientOption("my-client-opt")
	require.NotNil(t, opt)
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.Equal(t, "my-client-opt", gOpts.clientOpt)
}

func TestWithClientOption(t *testing.T) {
	opt := WithClientOption(42)
	require.NotNil(t, opt)
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.Equal(t, 42, gOpts.clientOpt)
}

func TestPropagationPolicyOptionApplyTo(t *testing.T) {
	policy := client.PropagationPolicy(metav1.DeletePropagationForeground)
	opt := WithPropagationPolicy(policy)
	require.NotNil(t, opt)
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.Equal(t, policy, gOpts.propagationPolicy)
}

func TestWithPropagationPolicy(t *testing.T) {
	policy := client.PropagationPolicy(metav1.DeletePropagationOrphan)
	opt := WithPropagationPolicy(policy)
	require.NotNil(t, opt)
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.Equal(t, policy, gOpts.propagationPolicy)
}

func TestSubResourceOptionApplyTo(t *testing.T) {
	opt := WithSubResource("status")
	require.NotNil(t, opt)
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.Equal(t, "status", gOpts.subResource)
}

func TestWithSubResource(t *testing.T) {
	opt := WithSubResource("scale")
	require.NotNil(t, opt)
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.Equal(t, "scale", gOpts.subResource)
}

func TestReplaceIfExistingOptionApplyTo(t *testing.T) {
	opt := &ReplaceIfExistingOption{}
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.True(t, gOpts.replaceIfExisting)
}

func TestHaveDifferentTypeWithOptionApplyTo(t *testing.T) {
	opt := &HaveDifferentTypeWithOption{}
	gOpts := &GraphOptions{}
	opt.ApplyTo(gOpts)
	assert.True(t, gOpts.haveDifferentTypeWith)
}

func TestMultipleOptionsApplied(t *testing.T) {
	gOpts := &GraphOptions{}
	opts := []GraphOption{
		WithClientOption("my-opt"),
		WithSubResource("status"),
		WithPropagationPolicy(client.PropagationPolicy(metav1.DeletePropagationBackground)),
		&ReplaceIfExistingOption{},
		&HaveDifferentTypeWithOption{},
	}
	for _, opt := range opts {
		opt.ApplyTo(gOpts)
	}
	assert.Equal(t, "my-opt", gOpts.clientOpt)
	assert.Equal(t, "status", gOpts.subResource)
	assert.Equal(t, client.PropagationPolicy(metav1.DeletePropagationBackground), gOpts.propagationPolicy)
	assert.True(t, gOpts.replaceIfExisting)
	assert.True(t, gOpts.haveDifferentTypeWith)
}

// --- transform_types.go -- NewObjectVertex ---

func TestNewObjectVertex_NoOptions(t *testing.T) {
	oldObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "old-cm", Namespace: "ns"}}
	newObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "new-cm", Namespace: "ns"}}
	action := ActionCreatePtr()
	v := NewObjectVertex(oldObj, newObj, action)
	require.NotNil(t, v)
	assert.Equal(t, newObj, v.Obj)
	assert.Equal(t, oldObj, v.OriObj)
	assert.Equal(t, action, v.Action)
	assert.Equal(t, "", v.SubResource)
	assert.Nil(t, v.ClientOpt)
}

func TestNewObjectVertex_WithSubResource(t *testing.T) {
	oldObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}
	newObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}
	v := NewObjectVertex(oldObj, newObj, ActionStatusPtr(), WithSubResource("status"))
	assert.Equal(t, "status", v.SubResource)
}

func TestNewObjectVertex_WithClientOption(t *testing.T) {
	oldObj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc"}}
	newObj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc"}}
	v := NewObjectVertex(oldObj, newObj, ActionUpdatePtr(), WithClientOption("dry-run"))
	assert.Equal(t, "dry-run", v.ClientOpt)
}

func TestNewObjectVertex_MultipleOptions(t *testing.T) {
	oldObj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}
	newObj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}
	v := NewObjectVertex(oldObj, newObj, ActionPatchPtr(),
		WithSubResource("scale"),
		WithClientOption("field-manager"),
	)
	assert.Equal(t, "scale", v.SubResource)
	assert.Equal(t, "field-manager", v.ClientOpt)
}

func TestNewObjectVertex_NilAction(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}
	v := NewObjectVertex(nil, obj, nil)
	assert.Nil(t, v.Action)
	assert.Nil(t, v.OriObj)
	assert.Equal(t, obj, v.Obj)
}

// --- ObjectVertex.String ---

func TestObjectVertexString_NilAction(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "my-cm"}}
	v := &ObjectVertex{Obj: obj}
	assert.Equal(t, "{obj:*v1.ConfigMap, name: my-cm, action: nil}", v.String())
}

func TestObjectVertexString_WithAction(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "my-cm"}}
	v := &ObjectVertex{Obj: obj, Action: ActionDeletePtr()}
	assert.Equal(t, "{obj:*v1.ConfigMap, name: my-cm, action: DELETE}", v.String())
}

// --- GetScheme ---

func TestGetScheme(t *testing.T) {
	s := GetScheme()
	require.NotNil(t, s)
	gvks, _, err := s.ObjectKinds(&corev1.ConfigMap{})
	require.NoError(t, err)
	assert.NotEmpty(t, gvks)
}

// --- GetGVKName ---

func TestGetGVKName_RegisteredObject(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "default"}}
	key, err := GetGVKName(cm)
	require.NoError(t, err)
	require.NotNil(t, key)
	assert.Equal(t, "ConfigMap", key.Kind)
	assert.Equal(t, "cm1", key.Name)
	assert.Equal(t, "default", key.Namespace)
}

func TestGetGVKName_InstanceSet(t *testing.T) {
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "its1", Namespace: "ns2"}}
	key, err := GetGVKName(its)
	require.NoError(t, err)
	assert.Equal(t, "InstanceSet", key.Kind)
	assert.Equal(t, "its1", key.Name)
}

// --- IsReconciliationPaused ---

func TestIsReconciliationPaused_Paused(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{Paused: true},
	}
	assert.True(t, IsReconciliationPaused(its))
}

func TestIsReconciliationPaused_NotPaused(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{Paused: false},
	}
	assert.False(t, IsReconciliationPaused(its))
}

func TestIsReconciliationPaused_NoSpecPausedField(t *testing.T) {
	cm := &corev1.ConfigMap{}
	assert.False(t, IsReconciliationPaused(cm))
}

// --- IsObjectUpdating ---

func TestIsObjectUpdating_Updating(t *testing.T) {
	its := &workloads.InstanceSet{}
	its.Generation = 2
	its.Status.ObservedGeneration = 1
	assert.True(t, IsObjectUpdating(its))
}

func TestIsObjectUpdating_NotUpdating(t *testing.T) {
	its := &workloads.InstanceSet{}
	its.Generation = 3
	its.Status.ObservedGeneration = 3
	assert.False(t, IsObjectUpdating(its))
}

func TestIsObjectUpdating_NoStatusField(t *testing.T) {
	cm := &corev1.ConfigMap{}
	assert.False(t, IsObjectUpdating(cm))
}

// --- IsObjectDeleting ---

func TestIsObjectDeleting_NotDeleting(t *testing.T) {
	cm := &corev1.ConfigMap{}
	assert.False(t, IsObjectDeleting(cm))
}

func TestIsObjectDeleting_Deleting(t *testing.T) {
	now := metav1.Now()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}}
	assert.True(t, IsObjectDeleting(cm))
}

// --- IsObjectStatusUpdating ---

func TestIsObjectStatusUpdating_NotDeletingNotUpdating(t *testing.T) {
	its := &workloads.InstanceSet{}
	its.Generation = 1
	its.Status.ObservedGeneration = 1
	assert.True(t, IsObjectStatusUpdating(its))
}

func TestIsObjectStatusUpdating_Updating(t *testing.T) {
	its := &workloads.InstanceSet{}
	its.Generation = 2
	its.Status.ObservedGeneration = 1
	assert.False(t, IsObjectStatusUpdating(its))
}

// --- IsOwnerOf ---

func TestIsOwnerOf_True(t *testing.T) {
	owner := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "ns", UID: "uid-123"},
	}
	child := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "child",
			Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workloads.kubeblocks.io/v1",
					Kind:       "InstanceSet",
					Name:       "owner",
					UID:        "uid-123",
				},
			},
		},
	}
	assert.True(t, IsOwnerOf(owner, child))
}

func TestIsOwnerOf_False_DifferentName(t *testing.T) {
	owner := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "owner-a", Namespace: "ns", UID: "uid-1"},
	}
	child := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "child",
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "workloads.kubeblocks.io/v1", Kind: "InstanceSet", Name: "owner-b", UID: "uid-2"},
			},
		},
	}
	assert.False(t, IsOwnerOf(owner, child))
}

func TestIsOwnerOf_NoOwnerReferences(t *testing.T) {
	owner := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "owner"}}
	child := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "child"}}
	assert.False(t, IsOwnerOf(owner, child))
}

// --- DefaultLess ---

func TestDefaultLess_BothObjectVertex(t *testing.T) {
	v1 := &ObjectVertex{Obj: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "aaa"}}, Action: ActionCreatePtr()}
	v2 := &ObjectVertex{Obj: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "zzz"}}, Action: ActionCreatePtr()}
	assert.True(t, DefaultLess(v1, v2))
	assert.False(t, DefaultLess(v2, v1))
}

func TestDefaultLess_NonObjectVertex(t *testing.T) {
	type fakeVertex struct{}
	v1 := &ObjectVertex{Obj: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}, Action: ActionCreatePtr()}
	var v2 graph.Vertex = &fakeVertex{}
	assert.False(t, DefaultLess(v1, v2))
	assert.False(t, DefaultLess(v2, v1))
}

// --- FindRootVertex ---

func TestFindRootVertex_EmptyDAG(t *testing.T) {
	dag := graph.NewDAG()
	_, err := FindRootVertex(dag)
	require.Error(t, err)
}

func TestFindRootVertex_WithRoot(t *testing.T) {
	dag := graph.NewDAG()
	root := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root"}}
	dag.AddVertex(&ObjectVertex{Obj: root, Action: ActionCreatePtr()})
	rv, err := FindRootVertex(dag)
	require.NoError(t, err)
	require.NotNil(t, rv)
	assert.Equal(t, root, rv.Obj)
}

// --- ActionPtr helpers ---

func TestActionPtrHelpers(t *testing.T) {
	assert.Equal(t, CREATE, *ActionCreatePtr())
	assert.Equal(t, UPDATE, *ActionUpdatePtr())
	assert.Equal(t, PATCH, *ActionPatchPtr())
	assert.Equal(t, DELETE, *ActionDeletePtr())
	assert.Equal(t, STATUS, *ActionStatusPtr())
}

// --- NewRequeueError ---

func TestNewRequeueError_Std(t *testing.T) {
	err := NewRequeueError(5000000000, "test-reason")
	require.Error(t, err)
	reqErr, ok := err.(RequeueError)
	require.True(t, ok)
	assert.Equal(t, "test-reason", reqErr.Reason())
	assert.Contains(t, err.Error(), "test-reason")
}

// --- GVKNObjKey equality ---

func TestGVKNObjKey_Equality(t *testing.T) {
	cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}}
	cm2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}}
	k1, err := GetGVKName(cm1)
	require.NoError(t, err)
	k2, err := GetGVKName(cm2)
	require.NoError(t, err)
	assert.Equal(t, *k1, *k2)

	cm3 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm-other", Namespace: "ns"}}
	k3, err := GetGVKName(cm3)
	require.NoError(t, err)
	assert.NotEqual(t, *k1, *k3)
}
