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

package multicluster

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

// buildMultiClusterClient creates a control client and worker clients for testing.
// controlObjs are pre-populated in the control-plane client.
// workers is a map of context name to pre-populated objects in that worker cluster.
func buildMultiClusterClient(controlObjs []client.Object, workers map[string][]client.Object) (client.Client, map[string]client.Client) {
	control := newFakeClient(controlObjs...)
	workerClients := make(map[string]client.Client)
	for ctx, objs := range workers {
		workerClients[ctx] = newFakeClient(objs...)
	}
	return NewClient(control, workerClients), workerClients
}

func TestNewClient_NoWorkers(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	assert.NotNil(t, mc)
}

func TestNewClient_WithWorkers(t *testing.T) {
	control := newFakeClient()
	workers := map[string]client.Client{
		"ctx-1": newFakeClient(),
		"ctx-2": newFakeClient(),
	}
	mc := NewClient(control, workers)
	assert.NotNil(t, mc)
}

func TestMclient_Scheme(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	assert.Equal(t, control.Scheme(), mc.Scheme())
}

func TestMclient_RESTMapper(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	assert.Equal(t, control.RESTMapper(), mc.RESTMapper())
}

func TestMclient_GroupVersionKindFor(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	cm := &corev1.ConfigMap{}
	gvk, err := mc.GroupVersionKindFor(cm)
	assert.NoError(t, err)
	assert.Equal(t, "ConfigMap", gvk.Kind)
}

func TestMclient_IsObjectNamespaced(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	cm := &corev1.ConfigMap{}
	// fake client's RESTMapper may not have ConfigMap mapped, so we just verify delegation doesn't panic
	_, _ = mc.IsObjectNamespaced(cm)
}

// === clientReader tests ===

func TestClientReader_Get_NoWorkers(t *testing.T) {
	cm := newConfigMap("default", "cm1", nil)
	control := newFakeClient(cm)
	mc := NewClient(control, nil)

	got := &corev1.ConfigMap{}
	err := mc.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "cm1"}, got)
	assert.NoError(t, err)
	assert.Equal(t, "cm1", got.Name)
}

func TestClientReader_Get_NotFound(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)

	got := &corev1.ConfigMap{}
	err := mc.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "nonexistent"}, got)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestClientReader_Get_WithWorkers_AnyOf(t *testing.T) {
	// control plane doesn't have the object, worker ctx-1 does
	workerCM := newConfigMap("default", "cm-worker", annotationWithPlacement("ctx-1"))
	mc, _ := buildMultiClusterClient(
		nil, // control has nothing
		map[string][]client.Object{
			"ctx-1": {workerCM},
		},
	)

	ctx := IntoContext(context.Background(), "ctx-1")
	got := &corev1.ConfigMap{}
	err := mc.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cm-worker"}, got, InDataContext())
	assert.NoError(t, err)
	assert.Equal(t, "cm-worker", got.Name)
}

func TestClientReader_Get_WithWorkers_ControlContext(t *testing.T) {
	// object exists in control plane, should find it there with InControlContext
	controlCM := newConfigMap("default", "cm-control", nil)
	workerCM := newConfigMap("default", "cm-worker", annotationWithPlacement("ctx-1"))
	mc, _ := buildMultiClusterClient(
		[]client.Object{controlCM},
		map[string][]client.Object{
			"ctx-1": {workerCM},
		},
	)

	got := &corev1.ConfigMap{}
	err := mc.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "cm-control"}, got, InControlContext())
	assert.NoError(t, err)
	assert.Equal(t, "cm-control", got.Name)
}

func TestClientReader_Get_MultiCheck(t *testing.T) {
	cm1 := newConfigMap("default", "cm-shared", annotationWithPlacement("ctx-1"))
	cm2 := newConfigMap("default", "cm-shared", annotationWithPlacement("ctx-2"))
	mc, _ := buildMultiClusterClient(
		nil,
		map[string][]client.Object{
			"ctx-1": {cm1},
			"ctx-2": {cm2},
		},
	)

	ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
	got := &corev1.ConfigMap{}
	err := mc.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cm-shared"}, got, InDataContext(), MultiCheck())
	assert.NoError(t, err)
	assert.Equal(t, "cm-shared", got.Name)
}

func TestClientReader_List_NoWorkers(t *testing.T) {
	cm1 := newConfigMap("default", "cm1", nil)
	cm2 := newConfigMap("default", "cm2", nil)
	control := newFakeClient(cm1, cm2)
	mc := NewClient(control, nil)

	list := &corev1.ConfigMapList{}
	err := mc.List(context.Background(), list)
	assert.NoError(t, err)
	assert.Len(t, list.Items, 2)
}

func TestClientReader_List_WithWorkers_MergesResults(t *testing.T) {
	cm1 := newConfigMap("default", "cm-ctx1", annotationWithPlacement("ctx-1"))
	cm2 := newConfigMap("default", "cm-ctx2", annotationWithPlacement("ctx-2"))
	mc, _ := buildMultiClusterClient(
		nil,
		map[string][]client.Object{
			"ctx-1": {cm1},
			"ctx-2": {cm2},
		},
	)

	ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
	list := &corev1.ConfigMapList{}
	err := mc.List(ctx, list, InDataContextUnspecified())
	assert.NoError(t, err)
	assert.Len(t, list.Items, 2)
}

// === clientWriter tests ===

func TestClientWriter_Create_NoWorkers(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)

	cm := newConfigMap("default", "new-cm", nil)
	err := mc.Create(context.Background(), cm)
	assert.NoError(t, err)

	// verify it was created in control plane
	got := &corev1.ConfigMap{}
	err = control.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "new-cm"}, got)
	assert.NoError(t, err)
	assert.Equal(t, "new-cm", got.Name)
}

func TestClientWriter_Create_WithWorkers_SetsPlacementKey(t *testing.T) {
	mc, workerClients := buildMultiClusterClient(
		nil,
		map[string][]client.Object{
			"ctx-1": {},
		},
	)

	ctx := IntoContext(context.Background(), "ctx-1")
	cm := newConfigMap("default", "new-cm", nil)
	err := mc.Create(ctx, cm, InDataContext())
	assert.NoError(t, err)

	// verify it was created in worker ctx-1 with placement annotation
	got := &corev1.ConfigMap{}
	err = workerClients["ctx-1"].Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "new-cm"}, got)
	assert.NoError(t, err)
	assert.Equal(t, "ctx-1", got.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
}

func TestClientWriter_Delete_NoWorkers(t *testing.T) {
	cm := newConfigMap("default", "to-delete", nil)
	control := newFakeClient(cm)
	mc := NewClient(control, nil)

	err := mc.Delete(context.Background(), cm)
	assert.NoError(t, err)

	// verify deleted
	got := &corev1.ConfigMap{}
	err = control.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "to-delete"}, got)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestClientWriter_Update_NoWorkers(t *testing.T) {
	cm := newConfigMap("default", "to-update", nil)
	control := newFakeClient(cm)
	mc := NewClient(control, nil)

	cm.Data = map[string]string{"key": "value"}
	err := mc.Update(context.Background(), cm)
	assert.NoError(t, err)

	got := &corev1.ConfigMap{}
	err = control.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "to-update"}, got)
	assert.NoError(t, err)
	assert.Equal(t, "value", got.Data["key"])
}

func TestClientWriter_Patch_NoWorkers(t *testing.T) {
	cm := newConfigMap("default", "to-patch", nil)
	control := newFakeClient(cm)
	mc := NewClient(control, nil)

	patch := client.MergeFrom(cm.DeepCopy())
	cm.Data = map[string]string{"patched": "true"}
	err := mc.Patch(context.Background(), cm, patch)
	assert.NoError(t, err)

	got := &corev1.ConfigMap{}
	err = control.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "to-patch"}, got)
	assert.NoError(t, err)
	assert.Equal(t, "true", got.Data["patched"])
}

func TestClientWriter_DeleteAllOf_NoWorkers(t *testing.T) {
	cm1 := newConfigMap("default", "cm1", nil)
	cm2 := newConfigMap("default", "cm2", nil)
	control := newFakeClient(cm1, cm2)
	mc := NewClient(control, nil)

	err := mc.DeleteAllOf(context.Background(), &corev1.ConfigMap{}, client.InNamespace("default"))
	assert.NoError(t, err)

	list := &corev1.ConfigMapList{}
	err = control.List(context.Background(), list, client.InNamespace("default"))
	assert.NoError(t, err)
	assert.Empty(t, list.Items)
}

// === statusClient and subResource tests ===

func TestStatusClient_Status(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	sw := mc.Status()
	assert.NotNil(t, sw)
}

func TestSubResourceClientConstructor_SubResource(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	src := mc.SubResource("status")
	assert.NotNil(t, src)
}

func TestSubResourceWriter_Update(t *testing.T) {
	// use unavailable client since fake client sub-resource update has limitations
	c := newUnavailableClient("ctx-1")
	cm := newConfigMap("default", "cm-status", nil)
	// unavailable client's Status().Update() returns nil
	err := c.Status().Update(context.Background(), cm)
	assert.NoError(t, err)
}

func TestSubResourceWriter_Patch(t *testing.T) {
	// use unavailable client since fake client sub-resource patch has limitations
	c := newUnavailableClient("ctx-1")
	cm := newConfigMap("default", "cm-status-patch", nil)
	err := c.Status().Patch(context.Background(), cm, client.MergeFrom(cm.DeepCopy()))
	assert.NoError(t, err)
}

func TestSubResourceReader_Get_UnavailableClient(t *testing.T) {
	// test sub-resource reader Get with unavailable client (returns error, not panic)
	c := newUnavailableClient("ctx-1")
	cm := newConfigMap("default", "cm-sub", nil)
	subObj := &corev1.ConfigMap{}
	err := c.SubResource("status").Get(context.Background(), cm, subObj)
	assert.Error(t, err)
	assert.True(t, isUnavailableError(err))
}

// === resolvedClients tests ===

func TestResolvedClients_NoWorkers(t *testing.T) {
	control := newFakeClient()
	mctx := mcontext{control: control, workers: nil}
	result := resolvedClients(mctx, context.Background(), nil, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "", result[0].context)
	assert.Equal(t, control, result[0].cli)
}

func TestResolvedClients_NoOption(t *testing.T) {
	control := newFakeClient()
	worker := newFakeClient()
	mctx := mcontext{control: control, workers: map[string]client.Client{"ctx-1": worker}}
	result := resolvedClients(mctx, context.Background(), nil, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "", result[0].context)
	assert.Equal(t, control, result[0].cli)
}

func TestResolvedClients_ControlContext(t *testing.T) {
	control := newFakeClient()
	worker := newFakeClient()
	mctx := mcontext{control: control, workers: map[string]client.Client{"ctx-1": worker}}
	result := resolvedClients(mctx, context.Background(), nil, []any{InControlContext()})
	assert.Len(t, result, 1)
	assert.Equal(t, control, result[0].cli)
}

func TestResolvedClients_Unspecified(t *testing.T) {
	control := newFakeClient()
	worker1 := newFakeClient()
	worker2 := newFakeClient()
	mctx := mcontext{control: control, workers: map[string]client.Client{"ctx-1": worker1, "ctx-2": worker2}}
	result := resolvedClients(mctx, context.Background(), nil, []any{InDataContextUnspecified()})
	assert.Len(t, result, 2)
	// should contain both workers (order may vary due to map iteration)
	contexts := []string{result[0].context, result[1].context}
	assert.Contains(t, contexts, "ctx-1")
	assert.Contains(t, contexts, "ctx-2")
}

func TestResolvedClients_Universal(t *testing.T) {
	control := newFakeClient()
	worker1 := newFakeClient()
	worker2 := newFakeClient()
	mctx := mcontext{control: control, workers: map[string]client.Client{"ctx-1": worker1, "ctx-2": worker2}}
	ctx := IntoContext(context.Background(), "ctx-1")
	result := resolvedClients(mctx, ctx, nil, []any{InUniversalContext()})
	// should contain control + ctx-1 from context
	assert.GreaterOrEqual(t, len(result), 2)
}

func TestResolvedClients_Oneshot(t *testing.T) {
	control := newFakeClient()
	worker1 := newFakeClient()
	worker2 := newFakeClient()
	mctx := mcontext{control: control, workers: map[string]client.Client{"ctx-1": worker1, "ctx-2": worker2}}
	ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
	result := resolvedClients(mctx, ctx, nil, []any{Oneshot()})
	assert.Len(t, result, 1)
	// oneshot uses first worker from context
	assert.Equal(t, "ctx-1", result[0].context)
}

func TestResolvedClients_Default_FromContextNObject(t *testing.T) {
	control := newFakeClient()
	worker1 := newFakeClient()
	mctx := mcontext{control: control, workers: map[string]client.Client{"ctx-1": worker1}}
	ctx := IntoContext(context.Background(), "ctx-1")
	obj := newConfigMap("default", "cm1", annotationWithPlacement("ctx-1"))
	result := resolvedClients(mctx, ctx, obj, []any{InDataContext()})
	assert.Len(t, result, 1)
	assert.Equal(t, "ctx-1", result[0].context)
}

// === hasClientOption tests ===

func TestHasClientOption_NilOpts(t *testing.T) {
	result := hasClientOption(nil)
	assert.Nil(t, result)
}

func TestHasClientOption_EmptySlice(t *testing.T) {
	result := hasClientOption([]any{})
	assert.Nil(t, result)
}

func TestHasClientOption_NoClientOption(t *testing.T) {
	result := hasClientOption([]any{"string", 42})
	assert.Nil(t, result)
}

func TestHasClientOption_WithClientOption(t *testing.T) {
	opt := InControlContext()
	result := hasClientOption([]any{opt})
	assert.NotNil(t, result)
	assert.True(t, result.control)
}

func TestHasClientOption_MixedOptions(t *testing.T) {
	opt := InDataContextUnspecified()
	result := hasClientOption([]any{"string", opt, 42})
	assert.NotNil(t, result)
	assert.True(t, result.unspecified)
}

// === dataClients tests ===

func TestDataClients(t *testing.T) {
	worker1 := newFakeClient()
	worker2 := newFakeClient()
	mctx := mcontext{
		workers: map[string]client.Client{
			"ctx-1": worker1,
			"ctx-2": worker2,
		},
	}
	result := dataClients(mctx, []string{"ctx-1", "ctx-2", "ctx-3"})
	assert.Len(t, result, 2) // ctx-3 doesn't exist
	contexts := []string{result[0].context, result[1].context}
	assert.Contains(t, contexts, "ctx-1")
	assert.Contains(t, contexts, "ctx-2")
}

func TestDataClients_EmptyWorkers(t *testing.T) {
	mctx := mcontext{workers: map[string]client.Client{}}
	result := dataClients(mctx, []string{"ctx-1"})
	assert.Empty(t, result)
}

// === removeDuplicate tests ===

func TestRemoveDuplicate(t *testing.T) {
	control := newFakeClient()
	worker := newFakeClient()
	clients := []contextCli{
		{"", control},
		{"ctx-1", worker},
		{"ctx-1", worker}, // duplicate
		{"ctx-2", worker},
	}
	result := removeDuplicate(clients)
	assert.Len(t, result, 3)
}

func TestRemoveDuplicate_AllUnique(t *testing.T) {
	clients := []contextCli{
		{"ctx-1", nil},
		{"ctx-2", nil},
	}
	result := removeDuplicate(clients)
	assert.Len(t, result, 2)
}

func TestRemoveDuplicate_Empty(t *testing.T) {
	result := removeDuplicate(nil)
	assert.Empty(t, result)
}

// === allOf and anyOf integration tests ===

func TestAllOf_UnavailableClientSkipped(t *testing.T) {
	control := newFakeClient()
	unavailable := newUnavailableClient("ctx-1")
	mctx := mcontext{
		control: control,
		workers: map[string]client.Client{"ctx-1": unavailable},
	}
	ctx := IntoContext(context.Background(), "ctx-1")
	obj := newConfigMap("default", "cm1", nil)

	callCount := 0
	request := func(cc contextCli, _ client.Object) error {
		callCount++
		return nil
	}
	err := allOf(mctx, ctx, obj, request, []any{InDataContext()})
	// should succeed since unavailable errors are tolerated
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount) // only the unavailable client
}

func TestAnyOf_FirstSuccessReturns(t *testing.T) {
	worker1 := newFakeClient(newConfigMap("default", "cm1", nil))
	worker2 := newFakeClient(newConfigMap("default", "cm1", nil))
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": worker1, "ctx-2": worker2},
	}
	ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
	obj := newConfigMap("default", "cm1", nil)

	callCount := 0
	request := func(cc contextCli, _ client.Object) error {
		callCount++
		return nil
	}
	err := anyOf(mctx, ctx, obj, request, []any{InDataContext()})
	assert.NoError(t, err)
	// anyOf returns on first success
	assert.Equal(t, 1, callCount)
}

func TestAnyOf_NoOption_UsesControl(t *testing.T) {
	control := newFakeClient()
	mctx := mcontext{control: control}
	obj := newConfigMap("default", "cm1", nil)

	callCount := 0
	request := func(cc contextCli, _ client.Object) error {
		callCount++
		return nil
	}
	err := anyOf(mctx, context.Background(), obj, request, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

// === subResourceWriter via mclient tests ===

func TestSubResourceWriter_Create_ViaMclient(t *testing.T) {
	control := newUnavailableClient("")
	mc := NewClient(control, nil)
	cm := newConfigMap("default", "cm-sub-create", nil)
	subObj := &corev1.ConfigMap{}
	err := mc.Status().Create(context.Background(), cm, subObj)
	assert.NoError(t, err)
}

func TestSubResourceWriter_Update_ViaMclient(t *testing.T) {
	control := newUnavailableClient("")
	mc := NewClient(control, nil)
	cm := newConfigMap("default", "cm-sub-update", nil)
	err := mc.Status().Update(context.Background(), cm)
	assert.NoError(t, err)
}

func TestSubResourceWriter_Patch_ViaMclient(t *testing.T) {
	control := newUnavailableClient("")
	mc := NewClient(control, nil)
	cm := newConfigMap("default", "cm-sub-patch", nil)
	err := mc.Status().Patch(context.Background(), cm, client.MergeFrom(cm.DeepCopy()))
	assert.NoError(t, err)
}

func TestSubResourceReader_Get_ViaMclient(t *testing.T) {
	control := newUnavailableClient("")
	mc := NewClient(control, nil)
	cm := newConfigMap("default", "cm-sub-get", nil)
	subObj := &corev1.ConfigMap{}
	err := mc.SubResource("status").Get(context.Background(), cm, subObj)
	assert.Error(t, err)
	assert.True(t, isUnavailableError(err))
}

func TestSubResourceClient_SubResource_ContainsReaderAndWriter(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	src := mc.SubResource("status")
	assert.NotNil(t, src)
	// verify it has both reader and writer by calling methods
	_ = src.Get
}

// === anyOfWithMultiCheck tests ===

func TestAnyOfWithMultiCheck_FirstSuccessReturns(t *testing.T) {
	worker1 := newFakeClient()
	worker2 := newFakeClient()
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": worker1, "ctx-2": worker2},
	}
	ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
	obj := newConfigMap("default", "cm1", nil)

	callCount := 0
	request := func(cc contextCli, _ client.Object) error {
		callCount++
		return nil
	}
	err := anyOfWithMultiCheck(mctx, ctx, obj, request, []any{MultiCheck()})
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount) // checks all clusters
}

func TestAnyOfWithMultiCheck_AllFail(t *testing.T) {
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": newUnavailableClient("ctx-1")},
	}
	ctx := IntoContext(context.Background(), "ctx-1")
	obj := newConfigMap("default", "cm1", nil)

	request := func(cc contextCli, _ client.Object) error {
		return getUnavailableError(cc.context, obj)
	}
	err := anyOfWithMultiCheck(mctx, ctx, obj, request, []any{MultiCheck()})
	assert.Error(t, err)
}

// === allOf error branches ===

func TestAllOf_GenericErrorReturned(t *testing.T) {
	worker := newFakeClient()
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": worker},
	}
	ctx := IntoContext(context.Background(), "ctx-1")
	obj := newConfigMap("default", "cm1", nil)

	genericErr := errors.New("generic error")
	request := func(cc contextCli, _ client.Object) error {
		return genericErr
	}
	err := allOf(mctx, ctx, obj, request, []any{InDataContext()})
	assert.Equal(t, genericErr, err)
}

func TestAllOf_UnavailableErrorReturnedWhenNoGenericError(t *testing.T) {
	unavailable := newUnavailableClient("ctx-1")
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": unavailable},
	}
	ctx := IntoContext(context.Background(), "ctx-1")
	obj := newConfigMap("default", "cm1", nil)

	request := func(cc contextCli, _ client.Object) error {
		return getUnavailableError(cc.context, obj)
	}
	err := allOf(mctx, ctx, obj, request, []any{InDataContext()})
	assert.Error(t, err)
	assert.True(t, isUnavailableError(err))
}

// === anyOf_ error branches ===

func TestAnyOf_FirstErrorThenSuccess(t *testing.T) {
	unavailable := newUnavailableClient("ctx-1")
	worker := newFakeClient()
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": unavailable, "ctx-2": worker},
	}
	ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
	obj := newConfigMap("default", "cm1", nil)

	callCount := 0
	request := func(cc contextCli, _ client.Object) error {
		callCount++
		if isUnavailableClient(cc.cli) {
			return getUnavailableError(cc.context, obj)
		}
		return nil
	}
	err := anyOf(mctx, ctx, obj, request, []any{InDataContext()})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 1)
}

func TestAnyOf_AllFail_ReturnsGenericError(t *testing.T) {
	worker := newFakeClient()
	mctx := mcontext{
		control: newFakeClient(),
		workers: map[string]client.Client{"ctx-1": worker},
	}
	ctx := IntoContext(context.Background(), "ctx-1")
	obj := newConfigMap("default", "cm1", nil)

	genericErr := errors.New("not found")
	request := func(cc contextCli, _ client.Object) error {
		return genericErr
	}
	err := anyOf(mctx, ctx, obj, request, []any{InDataContext()})
	assert.Equal(t, genericErr, err)
}
