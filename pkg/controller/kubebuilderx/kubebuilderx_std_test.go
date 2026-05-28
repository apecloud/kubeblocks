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

package kubebuilderx

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

// --- ObjectTree ---

func TestObjectTree_DeepCopy_NilRoot(t *testing.T) {
	tree := NewObjectTree()
	copy, err := tree.DeepCopy()
	require.NoError(t, err)
	assert.Nil(t, copy.GetRoot())
}

func TestObjectTree_DeepCopy_WithRoot(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "ns"}})
	tree.Context = context.Background()
	copy, err := tree.DeepCopy()
	require.NoError(t, err)
	assert.Equal(t, "root", copy.GetRoot().GetName())
}

func TestObjectTree_GetWithOption(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "ns"}})
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "ns"}}
	err := tree.AddWithOption(cm, WithSubResource("status"))
	require.NoError(t, err)

	obj, opts, err := tree.GetWithOption(cm)
	require.NoError(t, err)
	assert.NotNil(t, obj)
	assert.Equal(t, "status", opts.SubResource)
}

func TestObjectTree_GetWithOption_NotFound(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "ns"}})
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "missing", Namespace: "ns"}}
	obj, _, err := tree.GetWithOption(cm)
	require.NoError(t, err)
	assert.Nil(t, obj)
}

func TestObjectTree_DeleteSecondaryObjects(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "ns"}})
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "ns"}}
	require.NoError(t, tree.Add(cm))
	assert.Len(t, tree.GetSecondaryObjects(), 1)
	tree.DeleteSecondaryObjects()
	assert.Len(t, tree.GetSecondaryObjects(), 0)
}

func TestObjectTree_AddWithOption(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "ns"}})
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "ns"}}
	err := tree.AddWithOption(cm, SkipToReconcile(true))
	require.NoError(t, err)
	_, opts, err := tree.GetWithOption(cm)
	require.NoError(t, err)
	assert.True(t, opts.SkipToReconcile)
}

func TestObjectTree_List_NoMatch(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "ns"}})
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "ns"}}
	require.NoError(t, tree.Add(cm))
	result := tree.List(&corev1.Secret{})
	assert.Len(t, result, 0)
}

func TestObjectTree_SetFinalizer(t *testing.T) {
	tree := NewObjectTree()
	tree.SetFinalizer("my-finalizer")
	assert.Equal(t, "my-finalizer", tree.GetFinalizer())
}

// --- ObjectOption types ---

func TestWithSubResource_ApplyToObject(t *testing.T) {
	opts := ObjectOptions{}
	WithSubResource("status").ApplyToObject(&opts)
	assert.Equal(t, "status", opts.SubResource)
}

func TestSkipToReconcile_ApplyToObject(t *testing.T) {
	opts := ObjectOptions{}
	SkipToReconcile(true).ApplyToObject(&opts)
	assert.True(t, opts.SkipToReconcile)
}

// --- CheckResult and Result ---

func TestCheckResult_Constants(t *testing.T) {
	assert.True(t, ConditionSatisfied.Satisfied)
	assert.False(t, ConditionUnsatisfied.Satisfied)
}

func TestResult_Constants(t *testing.T) {
	assert.Equal(t, cntn, Continue.Next)
	assert.Equal(t, cmmt, Commit.Next)
}

func TestRetryAfter(t *testing.T) {
	r := RetryAfter(5 * time.Second)
	assert.Equal(t, rtry, r.Next)
	assert.Equal(t, 5*time.Second, r.RetryAfter)
}

func TestConditionUnsatisfiedWithError(t *testing.T) {
	r := ConditionUnsatisfiedWithError(fmt.Errorf("fail"))
	assert.False(t, r.Satisfied)
	assert.NotNil(t, r.Err)
}

// --- transformContext getters ---

func TestTransformContext_GetClient(t *testing.T) {
	tc := &transformContext{ctx: context.Background()}
	assert.Nil(t, tc.GetClient())
}

func TestTransformContext_GetRecorder(t *testing.T) {
	rec := record.NewFakeRecorder(10)
	tc := &transformContext{recorder: rec}
	assert.Equal(t, rec, tc.GetRecorder())
}

func TestTransformContext_GetLogger(t *testing.T) {
	tc := &transformContext{logger: logr.Discard()}
	assert.NotNil(t, tc.GetLogger())
}

// --- PlanBuilder ---

func TestPlanBuilder_AddTransformer(t *testing.T) {
	pb := &PlanBuilder{}
	result := pb.AddTransformer()
	assert.Equal(t, pb, result)
}

func TestPlanBuilder_Init(t *testing.T) {
	pb := &PlanBuilder{}
	assert.NoError(t, pb.Init())
}

// --- getTypeName ---

func TestGetTypeName(t *testing.T) {
	cm := &corev1.ConfigMap{}
	assert.Equal(t, "ConfigMap", getTypeName(cm))

	svc := corev1.Service{}
	assert.Equal(t, "Service", getTypeName(svc))
}

// --- placement ---

func TestPlacement_Nil(t *testing.T) {
	result := placement(nil)
	assert.Empty(t, result)
}

func TestPlacement_NoAnnotations(t *testing.T) {
	obj := &corev1.ConfigMap{}
	result := placement(obj)
	assert.Empty(t, result)
}

func TestPlacement_WithAnnotation(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "ctx1"},
	}}
	result := placement(obj)
	assert.Equal(t, "ctx1", result)
}

// --- assign ---

func TestAssign_NonInstance(t *testing.T) {
	ctx := context.Background()
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
	result := assign(ctx, obj)
	assert.Equal(t, obj, result)
}

func TestAssign_Instance(t *testing.T) {
	ctx := multicluster.IntoContext(context.Background(), "ctx1,ctx2")
	inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "inst-1", Namespace: "ns"}}
	result := assign(ctx, inst)
	assert.NotNil(t, result)
}

// --- clientOption ---

func TestClientOption_NilClientOpt(t *testing.T) {
	v := &model.ObjectVertex{}
	opt := clientOption(v)
	assert.NotNil(t, opt)
}

func TestClientOption_ValidMulticluster(t *testing.T) {
	v := &model.ObjectVertex{ClientOpt: multicluster.InDataContext()}
	opt := clientOption(v)
	assert.NotNil(t, opt)
}

func TestClientOption_PanicsOnUnknown(t *testing.T) {
	v := &model.ObjectVertex{ClientOpt: "not-a-client-option"}
	assert.Panics(t, func() {
		clientOption(v)
	})
}

// --- intoContext ---

func TestIntoContext_Std(t *testing.T) {
	ctx := intoContext(context.Background(), "ctx1")
	p, err := multicluster.FromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ctx1", p)
}

// --- Plan Execute ---

func TestPlan_Execute_EmptyVertices(t *testing.T) {
	plan := &Plan{}
	assert.NoError(t, plan.Execute())
}

// --- ErrDeepCopyFailed ---

func TestErrDeepCopyFailed(t *testing.T) {
	assert.NotNil(t, ErrDeepCopyFailed)
}

// --- emitFailureEvent ---

func TestEmitFailureEvent_NilErr(t *testing.T) {
	c := &controller{}
	c.emitFailureEvent()
}

func TestEmitFailureEvent_NilTree(t *testing.T) {
	c := &controller{err: fmt.Errorf("fail")}
	c.emitFailureEvent()
}

func TestEmitFailureEvent_NilRecorder(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root"}})
	c := &controller{err: fmt.Errorf("fail"), tree: tree}
	c.emitFailureEvent()
}

func TestEmitFailureEvent_NilRoot(t *testing.T) {
	tree := NewObjectTree()
	tree.EventRecorder = record.NewFakeRecorder(10)
	c := &controller{err: fmt.Errorf("fail"), tree: tree}
	c.emitFailureEvent()
}

func TestEmitFailureEvent_WithRecorderAndRoot(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root"}})
	tree.EventRecorder = record.NewFakeRecorder(10)
	c := &controller{err: fmt.Errorf("reconcile error"), tree: tree}
	c.emitFailureEvent()
}

// --- emitEvent (plan_builder) ---

func TestEmitEvent_NilCurrentTree(t *testing.T) {
	pb := &PlanBuilder{}
	pb.emitEvent(&corev1.ConfigMap{}, "reason", model.CREATE)
}

// --- emitFailureEvent conflict ---

func TestEmitFailureEvent_Conflict(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root"}})
	tree.EventRecorder = record.NewFakeRecorder(10)
	conflictErr := apierrors.NewConflict(schema.GroupResource{Group: "", Resource: "configmaps"}, "root", fmt.Errorf("conflict"))
	c := &controller{err: conflictErr, tree: tree}
	c.emitFailureEvent()
	// Should not emit event for conflict errors
}

// --- NewController ---

func TestNewController(t *testing.T) {
	c := NewController(context.Background(), nil, ctrl.Request{}, nil, logr.Discard())
	assert.NotNil(t, c)
}

// --- Controller Do ---

func TestController_Do_WithError(t *testing.T) {
	c := &controller{err: fmt.Errorf("previous error"), res: Continue}
	reconciler := &mockReconciler{}
	result := c.Do(reconciler)
	assert.Equal(t, c, result)
}

func TestController_Do_CommitSkipsReconciler(t *testing.T) {
	c := &controller{res: Commit}
	reconciler := &mockReconciler{}
	result := c.Do(reconciler)
	assert.Equal(t, c, result)
}

func TestController_Do_RetrySkipsReconciler(t *testing.T) {
	c := &controller{res: RetryAfter(time.Second)}
	reconciler := &mockReconciler{}
	result := c.Do(reconciler)
	assert.Equal(t, c, result)
}

func TestController_Do_UnexpectedAction(t *testing.T) {
	c := &controller{res: Result{Next: "unknown"}}
	reconciler := &mockReconciler{}
	result := c.Do(reconciler).(*controller)
	assert.Error(t, result.err)
}

func TestController_Do_UnsatisfiedPreCondition(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root"}})
	c := &controller{res: Continue, tree: tree}
	reconciler := &mockReconciler{preCondResult: ConditionUnsatisfied}
	result := c.Do(reconciler).(*controller)
	assert.NoError(t, result.err)
}

func TestController_Do_PreConditionError(t *testing.T) {
	tree := NewObjectTree()
	tree.SetRoot(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "root"}})
	c := &controller{res: Continue, tree: tree}
	reconciler := &mockReconciler{preCondResult: ConditionUnsatisfiedWithError(fmt.Errorf("fail"))}
	result := c.Do(reconciler).(*controller)
	assert.Error(t, result.err)
}

// --- Controller Prepare ---

func TestController_Prepare_NilTree(t *testing.T) {
	c := &controller{ctx: context.Background()}
	loader := &mockTreeLoader{tree: nil}
	result := c.Prepare(loader).(*controller)
	assert.Error(t, result.err)
}

func TestController_Prepare_LoaderError(t *testing.T) {
	c := &controller{ctx: context.Background()}
	loader := &mockTreeLoader{err: fmt.Errorf("load error")}
	result := c.Prepare(loader).(*controller)
	assert.Error(t, result.err)
}

// --- Controller Commit ---

func TestController_Commit_WithError(t *testing.T) {
	c := &controller{err: fmt.Errorf("fail")}
	_, err := c.Commit()
	assert.Error(t, err)
}

func TestController_Commit_NilRoot(t *testing.T) {
	tree := NewObjectTree()
	c := &controller{oldTree: tree}
	result, err := c.Commit()
	assert.NoError(t, err)
	assert.False(t, result.Requeue)
}

type mockReconciler struct {
	preCondResult *CheckResult
	reconcileRes  Result
	reconcileErr  error
}

func (m *mockReconciler) PreCondition(_ *ObjectTree) *CheckResult {
	if m.preCondResult != nil {
		return m.preCondResult
	}
	return ConditionSatisfied
}

func (m *mockReconciler) Reconcile(_ *ObjectTree) (Result, error) {
	return m.reconcileRes, m.reconcileErr
}

type mockTreeLoader struct {
	tree *ObjectTree
	err  error
}

func (m *mockTreeLoader) Load(_ context.Context, _ client.Reader, _ ctrl.Request, _ record.EventRecorder, _ logr.Logger) (*ObjectTree, error) {
	return m.tree, m.err
}

// --- inDataContext4C / inDataContext4G ---

func TestInDataContext4C(t *testing.T) {
	opt := inDataContext4C()
	assert.NotNil(t, opt)
}

func TestInDataContext4G(t *testing.T) {
	opt := inDataContext4G()
	assert.NotNil(t, opt)
}
