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

package rollout

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutPlanBuilderContextKey struct{}

func TestRolloutTransformContextAccessors(t *testing.T) {
	ctx := context.WithValue(context.Background(), rolloutPlanBuilderContextKey{}, "value")
	reader := fake.NewClientBuilder().WithScheme(testRolloutPlanBuilderScheme(t)).Build()
	recorder := record.NewFakeRecorder(1)
	logger := logr.Discard()
	transCtx := &rolloutTransformContext{
		Context:       ctx,
		Client:        reader,
		EventRecorder: recorder,
		Logger:        logger,
	}

	if transCtx.GetContext() != ctx {
		t.Fatalf("unexpected context accessor result")
	}
	if transCtx.GetClient() != reader {
		t.Fatalf("unexpected client accessor result")
	}
	if transCtx.GetRecorder() != recorder {
		t.Fatalf("unexpected recorder accessor result")
	}
	if transCtx.GetLogger() != logger {
		t.Fatalf("unexpected logger accessor result")
	}
}

func TestRolloutPlanBuilderDefaultWalkFuncRejectsInvalidVertex(t *testing.T) {
	builder := newTestRolloutPlanBuilder(t)

	if err := builder.defaultWalkFunc("not-an-object-vertex"); err == nil || !strings.Contains(err.Error(), "wrong vertex type") {
		t.Fatalf("expected wrong vertex type error, got %v", err)
	}

	cm := testConfigMap("default", "missing-action", nil)
	if err := builder.defaultWalkFunc(&model.ObjectVertex{Obj: cm}); err == nil || !strings.Contains(err.Error(), "vertex action can't be nil") {
		t.Fatalf("expected nil action error, got %v", err)
	}
}

func TestRolloutPlanBuilderReconcileCreateObject(t *testing.T) {
	builder := newTestRolloutPlanBuilder(t)
	cm := testConfigMap("default", "create-me", map[string]string{"before": "true"})
	vertex := &model.ObjectVertex{
		Obj:    cm,
		Action: model.ActionCreatePtr(),
	}

	if err := builder.defaultWalkFunc(vertex); err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := builder.cli.Get(context.Background(), client.ObjectKeyFromObject(cm), got); err != nil {
		t.Fatalf("expected created configmap, got %v", err)
	}
	if got.Data["before"] != "true" {
		t.Fatalf("unexpected created configmap data: %#v", got.Data)
	}

	existingVertex := &model.ObjectVertex{
		Obj:    testConfigMap("default", "create-me", map[string]string{"before": "true"}),
		Action: model.ActionCreatePtr(),
	}
	if err := builder.reconcileCreateObject(context.Background(), existingVertex); err != nil {
		t.Fatalf("expected already exists create to be ignored, got %v", err)
	}
}

func TestRolloutPlanBuilderReconcilePatchObject(t *testing.T) {
	orig := testConfigMap("default", "patch-me", map[string]string{"version": "old"})
	builder := newTestRolloutPlanBuilder(t, orig)
	patched := orig.DeepCopy()
	patched.Data["version"] = "new"
	patched.Data["added"] = "true"

	if err := builder.defaultWalkFunc(&model.ObjectVertex{
		OriObj: orig.DeepCopy(),
		Obj:    patched,
		Action: model.ActionPatchPtr(),
	}); err != nil {
		t.Fatalf("expected patch to succeed, got %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := builder.cli.Get(context.Background(), client.ObjectKeyFromObject(orig), got); err != nil {
		t.Fatalf("expected patched configmap, got %v", err)
	}
	if got.Data["version"] != "new" || got.Data["added"] != "true" {
		t.Fatalf("unexpected patched configmap data: %#v", got.Data)
	}

	missingOrig := testConfigMap("default", "patch-missing", map[string]string{"version": "old"})
	missingPatched := missingOrig.DeepCopy()
	missingPatched.Data["version"] = "new"
	if err := builder.reconcilePatchObject(context.Background(), &model.ObjectVertex{
		OriObj: missingOrig,
		Obj:    missingPatched,
	}); err != nil {
		t.Fatalf("expected missing object patch to be ignored, got %v", err)
	}
}

func TestRolloutPlanBuilderReconcileDeleteObject(t *testing.T) {
	cm := testConfigMap("default", "delete-me", nil)
	cm.Finalizers = []string{constant.RolloutFinalizerName}
	builder := newTestRolloutPlanBuilder(t, cm)
	vertex := &model.ObjectVertex{
		Obj:               cm.DeepCopy(),
		Action:            model.ActionDeletePtr(),
		PropagationPolicy: client.PropagationPolicy(metav1.DeletePropagationForeground),
	}

	if err := builder.defaultWalkFunc(vertex); err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	got := &corev1.ConfigMap{}
	err := builder.cli.Get(context.Background(), client.ObjectKeyFromObject(cm), got)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected deleted configmap to be not found, got %v", err)
	}

	if err := builder.reconcileDeleteObject(context.Background(), vertex); err != nil {
		t.Fatalf("expected missing object delete to be ignored, got %v", err)
	}
}

func newTestRolloutPlanBuilder(t *testing.T, objs ...client.Object) *rolloutPlanBuilder {
	t.Helper()

	cli := fake.NewClientBuilder().
		WithScheme(testRolloutPlanBuilderScheme(t)).
		WithObjects(objs...).
		Build()
	return &rolloutPlanBuilder{
		cli: cli,
		transCtx: &rolloutTransformContext{
			Context: context.Background(),
			Logger:  logr.Discard(),
		},
	}
}

func testRolloutPlanBuilderScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	return scheme
}

func testConfigMap(namespace, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: data,
	}
}
