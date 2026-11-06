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

package parameters

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/unstructured"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestParameterViewReconcileInitializesPlainTextContent(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 1,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}

	if view.Spec.Content.Type != parametersv1alpha1.PlainTextParameterViewContentType {
		t.Fatalf("expected content type %q, got %q", parametersv1alpha1.PlainTextParameterViewContentType, view.Spec.Content.Type)
	}
	if view.Spec.Content.Text != "max_connections=1000\n" {
		t.Fatalf("expected content to be backfilled from generated configmap, got %q", view.Spec.Content.Text)
	}
	if view.Status.FileFormat != parametersv1alpha1.Ini {
		t.Fatalf("expected fileFormat %q, got %q", parametersv1alpha1.Ini, view.Status.FileFormat)
	}
	if view.Status.Base.Revision != fmt.Sprint(componentParameterGeneration) {
		t.Fatalf("expected base revision %q, got %q", fmt.Sprint(componentParameterGeneration), view.Status.Base.Revision)
	}
	if view.Status.Base.ContentHash != hashContent("max_connections=1000\n") {
		t.Fatalf("unexpected base content hash: %q", view.Status.Base.ContentHash)
	}
	if view.Status.Latest.Revision != fmt.Sprint(componentParameterGeneration) {
		t.Fatalf("expected latest revision %q, got %q", fmt.Sprint(componentParameterGeneration), view.Status.Latest.Revision)
	}
	if view.Status.Latest.ContentHash != hashContent("max_connections=1000\n") {
		t.Fatalf("unexpected latest content hash: %q", view.Status.Latest.ContentHash)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewSyncedPhase, view.Status.Phase)
	}
	if view.Status.ObservedGeneration != view.Generation {
		t.Fatalf("expected observedGeneration %d, got %d", view.Generation, view.Status.ObservedGeneration)
	}
}

func TestParameterViewReconcileInitializesMarkerLineContent(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nmax_connections=1000\nsync_binlog=1\nserver_id=1\ncustom_local=on\n# trailing comment\n",
		templateContent: "template-value=1\n",
		dynamicParameters: []string{
			"default.max_connections",
		},
		staticParameters: []string{
			"default.sync_binlog",
		},
		immutableParameters: []string{
			"default.server_id",
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 1,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.MarkerLineParameterViewContentType,
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	expected := "[U] [mysqld]\n[D] max_connections=1000\n[S] sync_binlog=1\n[I] server_id=1\n[U] custom_local=on\n[U] # trailing comment\n"
	if view.Spec.Content.Type != parametersv1alpha1.MarkerLineParameterViewContentType {
		t.Fatalf("expected content type %q, got %q", parametersv1alpha1.MarkerLineParameterViewContentType, view.Spec.Content.Type)
	}
	if view.Spec.Content.Text != expected {
		t.Fatalf("expected rendered marker content %q, got %q", expected, view.Spec.Content.Text)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewSyncedPhase, view.Status.Phase)
	}
}

func TestParameterViewReconcileMarksInvalidWhenReferenceMissing(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, &parametersv1alpha1.ParameterView{
		ObjectMeta: metav1.ObjectMeta{
			Name:       parameterViewName,
			Namespace:  parameterViewNamespace,
			Generation: 1,
		},
		Spec: parametersv1alpha1.ParameterViewSpec{
			ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
			TemplateName: templateName,
			FileName:     fileName,
		},
	})

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	if view.Status.Message == "" {
		t.Fatalf("expected invalid message to be set")
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonReferenceNotFound)
}

func TestParameterViewReconcileMarksInvalidWhenTemplateMissing(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 1,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: "missing-template",
				FileName:     fileName,
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonTemplateNotFound)
}

func TestParameterViewReconcileMarksInvalidWhenFileMissing(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 1,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     "missing.cnf",
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonFileNotFound)
}

func TestParameterViewReconcileRejectsUnsupportedContentType(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 1,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.ParameterViewContentType("Chunked"),
				},
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonUnsupportedContentType)
}

func TestParameterViewReconcileReappliesDraftWhenSourceDriftIsExpressible(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=2000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    "1",
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    "1",
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonApplying)
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "max_connections=1500\n", map[string]*string{
		"default.max_connections": ptr.To("1500"),
	})

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if got := compParam.Spec.Desired.Assignments["default.max_connections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired default.max_connections=1500, got %#v", got)
	}
}

func TestParameterViewReconcileWritesPlainTextToComponentParameter(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonApplying)
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "max_connections=1500\n", map[string]*string{
		"default.max_connections": ptr.To("1500"),
	})

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Assignments == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Assignments["default.max_connections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired default.max_connections=1500, got %#v", got)
	}
}

func TestParameterViewReconcilePersistsRemoval(t *testing.T) {
	const original = "max_connections=1000\nsync_binlog=1\n"
	const edited = "sync_binlog=1\n"
	for _, immutable := range []bool{false, true} {
		t.Run(fmt.Sprintf("immutable=%t", immutable), func(t *testing.T) {
			view := &parametersv1alpha1.ParameterView{
				ObjectMeta: metav1.ObjectMeta{Name: parameterViewName, Namespace: parameterViewNamespace, Generation: 2},
				Spec: parametersv1alpha1.ParameterViewSpec{
					ParameterRef: corev1.LocalObjectReference{Name: componentParameterName}, TemplateName: templateName, FileName: fileName,
					Content: parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: edited},
				},
				Status: parametersv1alpha1.ParameterViewStatus{
					FileFormat: parametersv1alpha1.Ini,
					Base:       parametersv1alpha1.ParameterViewRevision{Revision: fmt.Sprint(componentParameterGeneration), ContentHash: hashContent(original)},
					Latest:     parametersv1alpha1.ParameterViewRevision{Revision: fmt.Sprint(componentParameterGeneration), ContentHash: hashContent(original)},
				},
			}
			opts := parameterViewTestOptions{runtimeContent: original, view: view}
			if immutable {
				opts.immutableParameters = []string{"default.max_connections"}
			}
			reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(opts)...)
			request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(view)}
			if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(view), view); err != nil {
				t.Fatal(err)
			}
			if immutable {
				if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase || !strings.Contains(view.Status.Message, "immutable parameter") {
					t.Fatalf("immutable removal was not rejected: %s: %s", view.Status.Phase, view.Status.Message)
				}
				assertParameterViewConditionReason(t, view, parameterViewReasonSchemaValidationFailed)
				return
			}
			if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
				t.Fatalf("expected Applying after removal, got %s: %s", view.Status.Phase, view.Status.Message)
			}
			cp := &parametersv1alpha1.ComponentParameter{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: parameterViewNamespace, Name: componentParameterName}, cp); err != nil {
				t.Fatal(err)
			}
			expected := []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "default.max_connections"}}
			if cp.Spec.Desired == nil || !reflect.DeepEqual(cp.Spec.Desired.Updates, expected) {
				t.Fatalf("persisted removal = %#v, want %#v", cp.Spec.Desired, expected)
			}
			if _, ok := cp.Spec.Desired.Assignments["default.max_connections"]; ok {
				t.Fatal("removed key remained in assignments")
			}
			if !desiredParametersContain(cp.Spec.Desired, map[string]*string{"default.max_connections": nil}) {
				t.Fatal("persisted removal does not match the submitted draft")
			}
			before := cp.Spec.Desired.DeepCopy()
			if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cp), cp); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, cp.Spec.Desired) {
				t.Fatalf("repeat submission changed desired inputs: %#v", cp.Spec.Desired)
			}
		})
	}
}

func TestParameterViewDesiredSetReplacesStaleUpdates(t *testing.T) {
	cp := &parametersv1alpha1.ComponentParameter{ObjectMeta: metav1.ObjectMeta{Name: componentParameterName, Namespace: parameterViewNamespace}, Spec: parametersv1alpha1.ComponentParameterSpec{
		Desired: &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{"keep": ptr.To("1")},
			Updates: []parametersv1alpha1.ParameterUpdate{
				{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "mode"},
				{Type: parametersv1alpha1.ParameterUpdateSet, Key: "mode", Value: ptr.To("old")},
				{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "unrelated"},
			},
		},
	}}
	r, cli := newParameterViewTestReconciler(t, cp)
	if err := r.patchComponentParameterDesired(context.Background(), cp, map[string]*string{"mode": ptr.To("new")}); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cp), cp); err != nil {
		t.Fatal(err)
	}
	if !desiredParametersContain(cp.Spec.Desired, map[string]*string{"mode": ptr.To("new"), "unrelated": nil, "keep": ptr.To("1")}) {
		t.Fatalf("new set was shadowed or unrelated input lost: %#v", cp.Spec.Desired)
	}
	if len(cp.Spec.Desired.Updates) != 1 || cp.Spec.Desired.Updates[0].Key != "unrelated" {
		t.Fatalf("stale updates remain: %#v", cp.Spec.Desired.Updates)
	}
}

func TestDesiredParametersContainEffectiveUpdates(t *testing.T) {
	for _, tt := range []struct {
		name   string
		inputs *parametersv1alpha1.ParameterInputs
		patch  map[string]*string
		want   bool
	}{
		{
			name:   "remove",
			inputs: &parametersv1alpha1.ParameterInputs{Updates: []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "mode"}}},
			patch:  map[string]*string{"mode": nil}, want: true,
		}, {
			name: "update overrides assignment",
			inputs: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"mode": ptr.To("old")},
				Updates:     []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateSet, Key: "mode", Value: ptr.To("new")}},
			},
			patch: map[string]*string{"mode": ptr.To("old")},
		}, {
			name: "invalid update",
			inputs: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"mode": ptr.To("old")},
				Updates:     []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateSet, Key: "mode"}},
			},
			patch: map[string]*string{"mode": ptr.To("old")},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := desiredParametersContain(tt.inputs, tt.patch); got != tt.want {
				t.Fatalf("contain = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterViewStaleDesiredPatchConflicts(t *testing.T) {
	cp := &parametersv1alpha1.ComponentParameter{ObjectMeta: metav1.ObjectMeta{Name: componentParameterName, Namespace: parameterViewNamespace}}
	r, cli := newParameterViewTestReconciler(t, cp)
	first, stale := cp.DeepCopy(), cp.DeepCopy()
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cp), first); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cp), stale); err != nil {
		t.Fatal(err)
	}
	if err := r.patchComponentParameterDesired(context.Background(), first, map[string]*string{"first": nil}); err != nil {
		t.Fatal(err)
	}
	if err := r.patchComponentParameterDesired(context.Background(), stale, map[string]*string{"second": nil}); !apierrors.IsConflict(err) {
		t.Fatalf("stale patch error = %v, want conflict", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cp), cp); err != nil {
		t.Fatal(err)
	}
	if !desiredParametersContain(cp.Spec.Desired, map[string]*string{"first": nil}) {
		t.Fatalf("first removal was lost: %#v", cp.Spec.Desired)
	}
}

type parameterViewConflictingClient struct{ client.Client }

func (c parameterViewConflictingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*parametersv1alpha1.ComponentParameter); ok {
		return apierrors.NewConflict(schema.GroupResource{Group: parametersv1alpha1.GroupVersion.Group, Resource: "componentparameters"}, obj.GetName(), fmt.Errorf("stale resource version"))
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestParameterViewReconcileRequeuesDesiredPatchConflict(t *testing.T) {
	const original = "max_connections=1000\n"
	view := &parametersv1alpha1.ParameterView{
		ObjectMeta: metav1.ObjectMeta{Name: parameterViewName, Namespace: parameterViewNamespace, Generation: 2},
		Spec: parametersv1alpha1.ParameterViewSpec{
			ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
			TemplateName: templateName, FileName: fileName,
			Content: parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
		},
		Status: parametersv1alpha1.ParameterViewStatus{
			FileFormat: parametersv1alpha1.Ini,
			Base:       parametersv1alpha1.ParameterViewRevision{Revision: fmt.Sprint(componentParameterGeneration), ContentHash: hashContent(original)},
			Latest:     parametersv1alpha1.ParameterViewRevision{Revision: fmt.Sprint(componentParameterGeneration), ContentHash: hashContent(original)},
		},
	}
	r, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(original, view)...)
	r.Client = parameterViewConflictingClient{cli}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(view)})
	if !apierrors.IsConflict(err) {
		t.Fatalf("reconcile error = %v, want retriable conflict", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(view), view); err != nil {
		t.Fatal(err)
	}
	if view.Status.Phase == parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatal("write conflict marked the draft invalid")
	}
}

func TestParameterViewReconcileWritesMarkerLineToComponentParameter(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nmax_connections=1000\nsync_binlog=1\nserver_id=1\ncustom_local=on\n",
		templateContent: "template-value=1\n",
		dynamicParameters: []string{
			"default.max_connections",
		},
		staticParameters: []string{
			"default.sync_binlog",
		},
		immutableParameters: []string{
			"default.server_id",
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.MarkerLineParameterViewContentType,
					Text: "[U] [mysqld]\n[D] max_connections=1500\n[S] sync_binlog=1\n[I] server_id=1\n[U] custom_local=on\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\nserver_id=1\ncustom_local=on\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\nserver_id=1\ncustom_local=on\n"),
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonApplying)

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if got := compParam.Spec.Desired.Assignments["mysqld.max_connections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired mysqld.max_connections=1500, got %#v", got)
	}
}

func TestParameterViewReconcileRejectsWriteInReadOnlyMode(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Mode:         parametersv1alpha1.ParameterViewReadOnlyMode,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonReadOnly)

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired != nil && len(compParam.Spec.Desired.Assignments) != 0 {
		t.Fatalf("expected desired parameters to remain unchanged, got %#v", compParam.Spec.Desired.Assignments)
	}
}

func TestParameterViewReconcileWritesMarkerLineStaticEdits(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nmax_connections=1000\nsync_binlog=1\n",
		templateContent: "template-value=1\n",
		dynamicParameters: []string{
			"default.max_connections",
		},
		staticParameters: []string{
			"default.sync_binlog",
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.MarkerLineParameterViewContentType,
					Text: "[U] [mysqld]\n[D] max_connections=1000\n[S] sync_binlog=2\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\n"),
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonApplying)

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if got := compParam.Spec.Desired.Assignments["mysqld.sync_binlog"]; got == nil || *got != "2" {
		t.Fatalf("expected desired mysqld.sync_binlog=2, got %#v", got)
	}
}

func TestParameterViewReconcileRejectsMarkerLineUnmanagedEdits(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nmax_connections=1000\nsync_binlog=1\n",
		templateContent: "template-value=1\n",
		dynamicParameters: []string{
			"default.max_connections",
		},
		staticParameters: []string{
			"default.sync_binlog",
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.MarkerLineParameterViewContentType,
					Text: "[U] [mysqld changed]\n[D] max_connections=1000\n[S] sync_binlog=1\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\n"),
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonUnsupportedContentChanges)
}

func TestParameterViewReconcileRejectsInvalidMarkerSyntax(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nmax_connections=1000\n",
		templateContent: "template-value=1\n",
		dynamicParameters: []string{
			"default.max_connections",
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.MarkerLineParameterViewContentType,
					Text: "[S] [mysqld]\nmax_connections=1500\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\n"),
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonInvalidMarkerSyntax)
}

func TestParameterViewReconcileKeepsApplyingWhenDesiredAlreadySubmitted(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
		func(cp *parametersv1alpha1.ComponentParameter) {
			cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{
					"default.max_connections": ptr.To("1500"),
				},
			}
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonApplying)
}

func TestParameterViewReconcileMarksLatestSubmissionMergeFailed(t *testing.T) {
	now := metav1.Now()
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Submissions: []parametersv1alpha1.ParameterViewSubmission{{
					Revision: parametersv1alpha1.ParameterViewRevision{
						Revision:    fmt.Sprint(componentParameterGeneration),
						ContentHash: hashContent("max_connections=1500\n"),
					},
					SubmittedAt: &now,
					Assignments: map[string]*string{
						"default.max_connections": ptr.To("1500"),
					},
					Result: parametersv1alpha1.ParameterViewSubmissionResult{
						Phase:     parametersv1alpha1.ParameterViewSubmissionProcessingPhase,
						Reason:    parameterViewSubmissionReasonProcessing,
						Message:   "submission is being processed by ComponentParameter",
						UpdatedAt: &now,
					},
				}},
			},
		},
		func(cp *parametersv1alpha1.ComponentParameter) {
			cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{
					"default.max_connections": ptr.To("1500"),
				},
			}
			cp.Status.Phase = parametersv1alpha1.CMergeFailedPhase
			cp.Status.Message = "schema validation failed: max_connections exceeds upper bound"
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase)
	}
	if len(view.Status.Submissions) == 0 {
		t.Fatalf("expected submission history to be preserved")
	}
	assertParameterViewSubmissionResult(t, view.Status.Submissions[0], parametersv1alpha1.ParameterViewSubmissionFailedPhase, parameterViewSubmissionReasonMergeFailed, "schema validation failed: max_connections exceeds upper bound")
}

func TestParameterViewReconcileMarksLatestSubmissionReconfigureFailed(t *testing.T) {
	now := metav1.Now()
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1500\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1500\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1500\n"),
				},
				Submissions: []parametersv1alpha1.ParameterViewSubmission{{
					Revision: parametersv1alpha1.ParameterViewRevision{
						Revision:    fmt.Sprint(componentParameterGeneration),
						ContentHash: hashContent("max_connections=1500\n"),
					},
					SubmittedAt: &now,
					Assignments: map[string]*string{
						"default.max_connections": ptr.To("1500"),
					},
					Result: parametersv1alpha1.ParameterViewSubmissionResult{
						Phase:     parametersv1alpha1.ParameterViewSubmissionProcessingPhase,
						Reason:    parameterViewSubmissionReasonProcessing,
						Message:   "submission is being processed by ComponentParameter",
						UpdatedAt: &now,
					},
				}},
			},
		},
		func(cp *parametersv1alpha1.ComponentParameter) {
			msg := "rolling restart failed on pod test-0"
			cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{
					"default.max_connections": ptr.To("1500"),
				},
			}
			cp.Status.Phase = parametersv1alpha1.CFailedAndPausePhase
			cp.Status.Message = msg
			cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
				Name:    templateName,
				Phase:   parametersv1alpha1.CFailedAndPausePhase,
				Message: &msg,
				ReconcileDetail: &parametersv1alpha1.ReconcileDetail{
					ErrMessage: msg,
				},
			}}
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewSyncedPhase, view.Status.Phase)
	}
	if len(view.Status.Submissions) == 0 {
		t.Fatalf("expected submission history to be preserved")
	}
	assertParameterViewSubmissionResult(t, view.Status.Submissions[0], parametersv1alpha1.ParameterViewSubmissionFailedPhase, parameterViewSubmissionReasonReconfigureFailed, "rolling restart failed on pod test-0")
}

func TestParameterViewReconcileReturnsReadyAfterRuntimeCatchesUp(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content:      parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "max_connections=1500\n"},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
		func(cp *parametersv1alpha1.ComponentParameter) {
			cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{
					"default.max_connections": ptr.To("1500"),
				},
			}
		},
	)...)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no requeue, got %s", result.RequeueAfter)
	}

	rendered := &corev1.ConfigMap{}
	renderedKey := types.NamespacedName{
		Namespace: parameterViewNamespace,
		Name:      parameterscore.GetComponentCfgName(pvClusterName, pvComponentName, templateName),
	}
	if err := cli.Get(context.Background(), renderedKey, rendered); err != nil {
		t.Fatalf("get rendered configmap failed: %v", err)
	}
	rendered.Data[fileName] = "# normalized\nmax_connections=1500\n"
	if err := cli.Update(context.Background(), rendered); err != nil {
		t.Fatalf("update rendered configmap failed: %v", err)
	}

	result, err = reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no further requeue, got %s", result.RequeueAfter)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewSyncedPhase, view.Status.Phase)
	}
	if view.Spec.Content.Text != "# normalized\nmax_connections=1500\n" {
		t.Fatalf("expected content to refresh from runtime source, got %q", view.Spec.Content.Text)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonResolved)
}

func TestParameterViewReconcileRefreshesLegacyConflictWhenRuntimeSemanticsMatch(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.PlainTextParameterViewContentType,
					Text: "[mysqld]\nmax_connections=1500\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				Phase:              parametersv1alpha1.ParameterViewConflictPhase,
				ObservedGeneration: 2,
				Message:            "draft is based on an outdated revision for mysql-config/my.cnf; continue editing to retry replay or set resetToLatest=true to discard the draft",
				FileFormat:         parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
	)...)

	rendered := &corev1.ConfigMap{}
	renderedKey := types.NamespacedName{
		Namespace: parameterViewNamespace,
		Name:      parameterscore.GetComponentCfgName(pvClusterName, pvComponentName, templateName),
	}
	if err := cli.Get(context.Background(), renderedKey, rendered); err != nil {
		t.Fatalf("get rendered configmap failed: %v", err)
	}
	rendered.Data[fileName] = "[mysqld]\nmax_connections=1500\n"
	if err := cli.Update(context.Background(), rendered); err != nil {
		t.Fatalf("update rendered configmap failed: %v", err)
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no requeue, got %s", result.RequeueAfter)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewSyncedPhase, view.Status.Phase)
	}
	if view.Status.Base.Revision != fmt.Sprint(componentParameterGeneration) {
		t.Fatalf("expected base revision %q, got %q", fmt.Sprint(componentParameterGeneration), view.Status.Base.Revision)
	}
	if view.Status.Base.ContentHash != hashContent("[mysqld]\nmax_connections=1500\n") {
		t.Fatalf("expected base content hash to refresh from runtime source, got %q", view.Status.Base.ContentHash)
	}
	if view.Status.Latest.ContentHash != hashContent("[mysqld]\nmax_connections=1500\n") {
		t.Fatalf("expected latest content hash to refresh from runtime source, got %q", view.Status.Latest.ContentHash)
	}
	if view.Spec.Content.Text != "[mysqld]\nmax_connections=1500\n" {
		t.Fatalf("expected content to refresh from runtime source, got %q", view.Spec.Content.Text)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonResolved)
}

func TestParameterViewReconcileRetriesDraftReplayWhileInConflict(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1100\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 3,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.PlainTextParameterViewContentType,
					Text: "max_connections=1200\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				Phase:              parametersv1alpha1.ParameterViewConflictPhase,
				ObservedGeneration: 2,
				Message:            "draft is based on an outdated revision for mysql-config/my.cnf; continue editing to retry replay or set resetToLatest=true to discard the draft",
				FileFormat:         parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration - 1),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1100\n"),
				},
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q with message %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase, view.Status.Message)
	}
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "max_connections=1200\n", map[string]*string{
		"default.max_connections": ptr.To("1200"),
	})
	assertParameterViewConditionReason(t, view, parameterViewReasonApplying)
	if view.Status.Base.Revision != fmt.Sprint(componentParameterGeneration) {
		t.Fatalf("expected base revision %q, got %q", fmt.Sprint(componentParameterGeneration), view.Status.Base.Revision)
	}
	if view.Status.Base.ContentHash != hashContent("max_connections=1100\n") {
		t.Fatalf("expected base content hash %q, got %q", hashContent("max_connections=1100\n"), view.Status.Base.ContentHash)
	}

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Assignments == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Assignments["default.max_connections"]; got == nil || *got != "1200" {
		t.Fatalf("expected desired default.max_connections=1200, got %#v", got)
	}
}

func TestParameterViewReconcileReturnsReadyAfterMarkerLineRuntimeCatchesUp(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nmax_connections=1000\nsync_binlog=1\n",
		templateContent: "template-value=1\n",
		dynamicParameters: []string{
			"mysqld.max_connections",
		},
		staticParameters: []string{
			"mysqld.sync_binlog",
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.MarkerLineParameterViewContentType,
					Text: "[U] [mysqld]\n[D] max_connections=1500\n[S] sync_binlog=1\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nmax_connections=1000\nsync_binlog=1\n"),
				},
			},
		},
		mutate: []func(*parametersv1alpha1.ComponentParameter){
			func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"mysqld.max_connections": ptr.To("1500"),
					},
				}
			},
		},
	})...)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no requeue, got %s", result.RequeueAfter)
	}

	rendered := &corev1.ConfigMap{}
	renderedKey := types.NamespacedName{
		Namespace: parameterViewNamespace,
		Name:      parameterscore.GetComponentCfgName(pvClusterName, pvComponentName, templateName),
	}
	if err := cli.Get(context.Background(), renderedKey, rendered); err != nil {
		t.Fatalf("get rendered configmap failed: %v", err)
	}
	rendered.Data[fileName] = "[mysqld]\nmax_connections=1500\nsync_binlog=1\n"
	if err := cli.Update(context.Background(), rendered); err != nil {
		t.Fatalf("update rendered configmap failed: %v", err)
	}

	result, err = reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no further requeue, got %s", result.RequeueAfter)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewSyncedPhase, view.Status.Phase)
	}
	expected := "[U] [mysqld]\n[D] max_connections=1500\n[S] sync_binlog=1\n"
	if view.Spec.Content.Text != expected {
		t.Fatalf("expected content to refresh from runtime source, got %q", view.Spec.Content.Text)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonResolved)
}

func TestParameterViewReconcileRejectsUnsupportedPlainTextEdits(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"max_connections=1000\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.PlainTextParameterViewContentType,
					Text: "# edited comment\nmax_connections=1000\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("max_connections=1000\n"),
				},
			},
		},
	)...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonUnsupportedContentChanges)
}

func TestParameterViewReconcileRejectsSchemaInvalidParameterValue(t *testing.T) {
	mysqlCue, err := testdata.GetTestDataFileContent("cue_testdata/mysql.cue")
	if err != nil {
		t.Fatalf("load mysql cue failed: %v", err)
	}
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  "[mysqld]\nbinlog_format=MIXED\n",
		templateContent: "[mysqld]\nbinlog_format=MIXED\n",
		dynamicParameters: []string{
			"default.binlog_format",
		},
		parametersSchema: &parametersv1alpha1.ParametersSchema{
			CUE: string(mysqlCue),
		},
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     fileName,
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.PlainTextParameterViewContentType,
					Text: "[mysqld]\nbinlog_format=BAD\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.Ini,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nbinlog_format=MIXED\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("[mysqld]\nbinlog_format=MIXED\n"),
				},
			},
		},
	})...)

	_, err = reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewInvalidPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewInvalidPhase, view.Status.Phase)
	}
	assertParameterViewConditionReason(t, view, parameterViewReasonSchemaValidationFailed)
}

func TestParameterViewReconcileWritesYAMLPlainTextToDesiredParameters(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        "config.yaml",
		fileFormat:      parametersv1alpha1.YAML,
		runtimeContent:  "maxConnections: \"1000\"\n",
		templateContent: "maxConnections: \"1000\"\n",
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     "config.yaml",
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.PlainTextParameterViewContentType,
					Text: "maxConnections: \"1500\"\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.YAML,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("maxConnections: \"1000\"\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("maxConnections: \"1000\"\n"),
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q with message %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase, view.Status.Message)
	}
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "maxConnections: \"1500\"\n", map[string]*string{
		"maxConnections": ptr.To("1500"),
	})

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Assignments == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Assignments["maxConnections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired maxConnections=1500, got %#v", got)
	}
}

func TestParameterViewReconcileWritesJSONPlainTextToDesiredParameters(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        "config.json",
		fileFormat:      parametersv1alpha1.JSON,
		runtimeContent:  "{\n  \"maxConnections\": \"1000\"\n}\n",
		templateContent: "{\n  \"maxConnections\": \"1000\"\n}\n",
		view: &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName: templateName,
				FileName:     "config.json",
				Content: parametersv1alpha1.ParameterViewContent{
					Type: parametersv1alpha1.PlainTextParameterViewContentType,
					Text: "{\n  \"maxConnections\": \"1500\"\n}\n",
				},
			},
			Status: parametersv1alpha1.ParameterViewStatus{
				FileFormat: parametersv1alpha1.JSON,
				Base: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("{\n  \"maxConnections\": \"1000\"\n}\n"),
				},
				Latest: parametersv1alpha1.ParameterViewRevision{
					Revision:    fmt.Sprint(componentParameterGeneration),
					ContentHash: hashContent("{\n  \"maxConnections\": \"1000\"\n}\n"),
				},
			},
		},
	})...)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	view := &parametersv1alpha1.ParameterView{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: parameterViewName}, view); err != nil {
		t.Fatalf("get view failed: %v", err)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected phase %q, got %q with message %q", parametersv1alpha1.ParameterViewApplyingPhase, view.Status.Phase, view.Status.Message)
	}

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Assignments == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Assignments["maxconnections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired maxconnections=1500, got %#v", got)
	}
}

func assertParameterViewLastSubmission(t *testing.T, view *parametersv1alpha1.ParameterView, revision, content string, parameters map[string]*string) {
	t.Helper()

	if len(view.Status.Submissions) == 0 {
		t.Fatalf("expected at least one submission entry")
	}
	got := view.Status.Submissions[0]
	if got.Revision.Revision != revision {
		t.Fatalf("expected latest submission revision %q, got %q", revision, got.Revision.Revision)
	}
	if got.Revision.ContentHash != hashContent(content) {
		t.Fatalf("expected latest submission content hash %q, got %q", hashContent(content), got.Revision.ContentHash)
	}
	if got.SubmittedAt == nil || got.SubmittedAt.IsZero() {
		t.Fatalf("expected latest submission submittedAt to be set")
	}
	if !equalParameterValueMap(got.Assignments, parameters) {
		t.Fatalf("expected latest submission parameters %#v, got %#v", parameters, got.Assignments)
	}
	assertParameterViewSubmissionResult(t, got, parametersv1alpha1.ParameterViewSubmissionProcessingPhase, parameterViewSubmissionReasonProcessing, "submission is being processed by ComponentParameter")
}

func assertParameterViewSubmissionResult(t *testing.T, submission parametersv1alpha1.ParameterViewSubmission,
	phase parametersv1alpha1.ParameterViewSubmissionPhase, reason, message string) {
	t.Helper()

	if submission.Result.Phase != phase {
		t.Fatalf("expected submission result phase %q, got %q", phase, submission.Result.Phase)
	}
	if submission.Result.Reason != reason {
		t.Fatalf("expected submission result reason %q, got %q", reason, submission.Result.Reason)
	}
	if submission.Result.Message != message {
		t.Fatalf("expected submission result message %q, got %q", message, submission.Result.Message)
	}
	if submission.Result.UpdatedAt == nil || submission.Result.UpdatedAt.IsZero() {
		t.Fatalf("expected submission result updatedAt to be set")
	}
}

func TestCompactSubmissionsKeepsNewestEntriesWithinCap(t *testing.T) {
	submissions := make([]parametersv1alpha1.ParameterViewSubmission, 0, parameterViewSubmissionLimit+2)
	for i := 0; i < parameterViewSubmissionLimit+2; i++ {
		submissions = append(submissions, parametersv1alpha1.ParameterViewSubmission{
			Revision: parametersv1alpha1.ParameterViewRevision{
				Revision:    fmt.Sprintf("%d", i),
				ContentHash: fmt.Sprintf("h-%d", i),
			},
			Assignments: map[string]*string{
				fmt.Sprintf("p-%d", i): ptr.To(fmt.Sprintf("v-%d", i)),
			},
		})
	}

	compacted := compactSubmissions(submissions)
	if len(compacted) != parameterViewSubmissionLimit {
		t.Fatalf("expected compacted submissions to keep %d entries, got %d", parameterViewSubmissionLimit, len(compacted))
	}
	if compacted[0].Revision.Revision != "0" {
		t.Fatalf("expected newest submission revision %q, got %q", "0", compacted[0].Revision.Revision)
	}
	if compacted[len(compacted)-1].Revision.Revision != fmt.Sprintf("%d", parameterViewSubmissionLimit-1) {
		t.Fatalf("expected oldest retained submission revision %q, got %q", fmt.Sprintf("%d", parameterViewSubmissionLimit-1), compacted[len(compacted)-1].Revision.Revision)
	}
}

const (
	parameterViewNamespace       = "default"
	pvClusterName                = "test-cluster"
	pvComponentName              = "mysql"
	pvComponentDefName           = "mysql-def"
	componentParameterName       = "test-component-parameter"
	parameterViewName            = "test-view"
	templateName                 = "mysql-config"
	templateConfigMapName        = "mysql-template"
	fileName                     = "my.cnf"
	componentParameterGeneration = 7
)

func newParameterViewTestReconciler(t *testing.T, objects ...runtime.Object) (*ParameterViewReconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme failed: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme failed: %v", err)
	}
	if err := parametersv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add parameters scheme failed: %v", err)
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&parametersv1alpha1.ParameterView{}).
		WithRuntimeObjects(objects...).
		Build()

	return &ParameterViewReconciler{Client: cli, Scheme: scheme}, cli
}

func newParameterViewTestObjects(runtimeContent string, view *parametersv1alpha1.ParameterView, mutate ...func(*parametersv1alpha1.ComponentParameter)) []runtime.Object {
	return newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		fileName:        fileName,
		fileFormat:      parametersv1alpha1.Ini,
		runtimeContent:  runtimeContent,
		templateContent: "template-value=1\n",
		view:            view,
		mutate:          mutate,
	})
}

type parameterViewTestOptions struct {
	fileName            string
	fileFormat          parametersv1alpha1.CfgFileFormat
	runtimeContent      string
	templateContent     string
	parametersSchema    *parametersv1alpha1.ParametersSchema
	dynamicParameters   []string
	staticParameters    []string
	immutableParameters []string
	view                *parametersv1alpha1.ParameterView
	mutate              []func(*parametersv1alpha1.ComponentParameter)
}

func newParameterViewTestObjectsWithOptions(opts parameterViewTestOptions) []runtime.Object {
	if opts.fileName == "" {
		opts.fileName = fileName
	}
	if opts.fileFormat == "" {
		opts.fileFormat = parametersv1alpha1.Ini
	}
	if opts.templateContent == "" {
		opts.templateContent = "template-value=1\n"
	}
	componentParameter := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       componentParameterName,
			Namespace:  parameterViewNamespace,
			Generation: componentParameterGeneration,
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{
			ClusterName:   pvClusterName,
			ComponentName: pvComponentName,
			ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{
				Name: templateName,
				ConfigSpec: &appsv1.ComponentFileTemplate{
					Name:      templateName,
					Template:  templateConfigMapName,
					Namespace: parameterViewNamespace,
				},
				ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
					opts.fileName: {
						Content: ptr.To(opts.runtimeContent),
					},
				},
			}},
		},
	}
	for _, fn := range opts.mutate {
		if fn != nil {
			fn(componentParameter)
		}
	}

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvClusterName,
			Namespace: parameterViewNamespace,
		},
	}

	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.GenerateClusterComponentName(pvClusterName, pvComponentName),
			Namespace: parameterViewNamespace,
		},
		Spec: appsv1.ComponentSpec{
			CompDef: pvComponentDefName,
		},
	}

	componentDef := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvComponentDefName,
		},
		Spec: appsv1.ComponentDefinitionSpec{
			ServiceVersion: "8.0.0",
			Configs: []appsv1.ComponentFileTemplate{{
				Name:            templateName,
				Template:        templateConfigMapName,
				Namespace:       parameterViewNamespace,
				ExternalManaged: ptr.To(true),
			}},
		},
		Status: appsv1.ComponentDefinitionStatus{
			Phase: appsv1.AvailablePhase,
		},
	}

	parametersDef := &parametersv1alpha1.ParametersDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mysql-params",
		},
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			ComponentDef: pvComponentDefName,
			TemplateName: templateName,
			FileName:     opts.fileName,
			FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
				Format: opts.fileFormat,
			},
			ParametersSchema:    opts.parametersSchema,
			DynamicParameters:   opts.dynamicParameters,
			StaticParameters:    opts.staticParameters,
			ImmutableParameters: opts.immutableParameters,
		},
		Status: parametersv1alpha1.ParametersDefinitionStatus{
			Phase: parametersv1alpha1.PDAvailablePhase,
		},
	}

	template := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateConfigMapName,
			Namespace: parameterViewNamespace,
		},
		Data: map[string]string{
			opts.fileName: opts.templateContent,
		},
	}

	rendered := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      parameterscore.GetComponentCfgName(pvClusterName, pvComponentName, templateName),
			Namespace: parameterViewNamespace,
			Labels: constant.GetCompLabels(pvClusterName, pvComponentName, map[string]string{
				constant.CMConfigurationTemplateNameLabelKey: templateName,
				constant.CMConfigurationTypeLabelKey:         "config",
				constant.CMConfigurationSpecProviderLabelKey: templateName,
			}),
		},
		Data: map[string]string{
			opts.fileName: opts.runtimeContent,
		},
	}

	return []runtime.Object{
		opts.view,
		componentParameter,
		cluster,
		component,
		componentDef,
		parametersDef,
		template,
		rendered,
	}
}

func assertParameterViewConditionReason(t *testing.T, view *parametersv1alpha1.ParameterView, reason string) {
	t.Helper()
	for _, condition := range view.Status.Conditions {
		if condition.Type == parameterViewSyncedCondition {
			if condition.Reason != reason {
				t.Fatalf("expected condition reason %q, got %q", reason, condition.Reason)
			}
			return
		}
	}
	t.Fatalf("ready condition not found")
}

var _ = Describe("ParameterView Controller", func() {
	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	It("resolves helper state for parameter value maps and failure messages", func() {
		source := map[string]*string{
			"keep":   ptr.To("1"),
			"delete": nil,
		}
		equalValues := map[string]*string{"keep": ptr.To("1"), "delete": nil}

		By("comparing desired parameter maps with nil-aware semantics")
		Expect(equalParameterValueMap(source, equalValues)).Should(BeTrue())
		Expect(equalParameterValueMap(source, map[string]*string{"keep": ptr.To("1")})).Should(BeFalse())
		Expect(equalParameterValueMap(source, map[string]*string{"keep": ptr.To("2"), "delete": nil})).Should(BeFalse())
		Expect(equalParameterValueMap(source, map[string]*string{"keep": nil, "delete": nil})).Should(BeFalse())

		By("checking desired parameter containment")
		Expect(desiredParametersContain(nil, source)).Should(BeFalse())
		Expect(desiredParametersContain(&parametersv1alpha1.ParameterInputs{}, source)).Should(BeFalse())
		Expect(desiredParametersContain(&parametersv1alpha1.ParameterInputs{Assignments: source}, nil)).Should(BeFalse())
		Expect(desiredParametersContain(&parametersv1alpha1.ParameterInputs{Assignments: source}, equalValues)).Should(BeTrue())
		Expect(desiredParametersContain(&parametersv1alpha1.ParameterInputs{Assignments: source}, map[string]*string{"keep": ptr.To("2")})).Should(BeFalse())
		Expect(desiredParametersContain(&parametersv1alpha1.ParameterInputs{Assignments: source}, map[string]*string{"missing": ptr.To("1")})).Should(BeFalse())

		By("choosing the most specific config item failure message")
		Expect(configItemFailureMessage(nil, "")).Should(Equal("component parameter reconfigure failed"))
		Expect(configItemFailureMessage(nil, "fallback")).Should(Equal("fallback"))
		Expect(configItemFailureMessage(&parametersv1alpha1.ConfigTemplateItemDetailStatus{
			ReconcileDetail: &parametersv1alpha1.ReconcileDetail{ErrMessage: "err-message", ExecResult: "exec-result"},
		}, "fallback")).Should(Equal("err-message"))
		Expect(configItemFailureMessage(&parametersv1alpha1.ConfigTemplateItemDetailStatus{
			ReconcileDetail: &parametersv1alpha1.ReconcileDetail{ExecResult: "exec-result"},
		}, "fallback")).Should(Equal("exec-result"))
		Expect(configItemFailureMessage(&parametersv1alpha1.ConfigTemplateItemDetailStatus{
			Message: ptr.To("status-message"),
		}, "fallback")).Should(Equal("status-message"))
		Expect(configItemFailureMessage(&parametersv1alpha1.ConfigTemplateItemDetailStatus{
			Message: ptr.To(""),
		}, "fallback")).Should(Equal("fallback"))
	})

	It("writes PlainText content back to ComponentParameter desired parameters", func() {
		_, _, _, _, _ = mockReconcileResource()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())

		viewKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      "parameterview-phase2-write",
		}
		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: viewKey.Namespace,
				Name:      viewKey.Name,
				Labels:    map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cfgKey.Name},
				TemplateName: configSpecName,
				FileName:     testparameters.MysqlConfigFile,
			},
		}
		Expect(k8sClient.Create(testCtx.Ctx, view)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
			g.Expect(obj.Spec.Content.Text).Should(ContainSubstring("gtid_mode=OFF"))
			g.Expect(obj.Labels).Should(HaveKeyWithValue(parameterViewParameterRefLabelKey, cfgKey.Name))
			g.Expect(obj.Labels).Should(HaveKeyWithValue(parameterViewTemplateLabelKey, configSpecName))
			g.Expect(obj.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterName))
			g.Expect(obj.Labels).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, defaultCompName))
		})).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			obj.Spec.Content.Text = strings.Replace(obj.Spec.Content.Text, "gtid_mode=OFF", "gtid_mode=ON", 1)
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewApplyingPhase))
		})).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Spec.Desired).ShouldNot(BeNil())
			g.Expect(cfg.Spec.Desired.Assignments).Should(HaveKeyWithValue("gtid_mode", ptr.To("ON")))
		})).Should(Succeed())
	})

	It("persists PlainText removal and removes the runtime parameter", func() {
		_, _, _, _, _ = mockReconcileResource()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())

		viewKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      "parameterview-remove",
		}
		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: viewKey.Namespace,
				Name:      viewKey.Name,
				Labels:    map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cfgKey.Name},
				TemplateName: configSpecName,
				FileName:     testparameters.MysqlConfigFile,
			},
		}
		Expect(k8sClient.Create(testCtx.Ctx, view)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
			g.Expect(obj.Spec.Content.Text).Should(ContainSubstring("gtid_mode=OFF"))
		})).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			obj.Spec.Content.Text = strings.Replace(obj.Spec.Content.Text, "gtid_mode=OFF\n", "", 1)
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Spec.Desired).ShouldNot(BeNil())
			g.Expect(cfg.Spec.Desired.Assignments).ShouldNot(HaveKey("gtid_mode"))
			g.Expect(cfg.Spec.Desired.Updates).Should(ContainElement(parametersv1alpha1.ParameterUpdate{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "gtid_mode"}))
		})).Should(Succeed())
		runtimeKey := client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: parameterscore.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
		Eventually(testapps.CheckObj(&testCtx, runtimeKey, func(g Gomega, cm *corev1.ConfigMap) {
			config, err := unstructured.LoadConfig(testparameters.MysqlConfigFile, cm.Data[testparameters.MysqlConfigFile], parametersv1alpha1.Ini)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(config.Get("mysqld.gtid_mode")).Should(BeNil())
			g.Expect(config.Get("mysqld.server-id")).Should(Equal("1"))
		})).Should(Succeed())
	})

	It("rejects writes in ReadOnly mode", func() {
		_, _, _, _, _ = mockReconcileResource()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())

		viewKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      "parameterview-phase2-readonly",
		}
		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: viewKey.Namespace,
				Name:      viewKey.Name,
				Labels:    map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cfgKey.Name},
				TemplateName: configSpecName,
				FileName:     testparameters.MysqlConfigFile,
				Mode:         parametersv1alpha1.ParameterViewReadOnlyMode,
			},
		}
		Expect(k8sClient.Create(testCtx.Ctx, view)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
		})).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			obj.Spec.Content.Text = strings.Replace(obj.Spec.Content.Text, "gtid_mode=OFF", "gtid_mode=ON", 1)
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewInvalidPhase))
		})).Should(Succeed())

		Consistently(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			if cfg.Spec.Desired == nil || cfg.Spec.Desired.Assignments == nil {
				return
			}
			g.Expect(cfg.Spec.Desired.Assignments).ShouldNot(HaveKey("mysqld.gtid_mode"))
		})).Should(Succeed())
	})

	It("keeps Applying when desired content is already submitted but runtime is stale", func() {
		_, _, _, _, _ = mockReconcileResource()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())

		viewKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      "parameterview-phase2-pending",
		}
		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: viewKey.Namespace,
				Name:      viewKey.Name,
				Labels:    map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cfgKey.Name},
				TemplateName: configSpecName,
				FileName:     testparameters.MysqlConfigFile,
			},
		}
		Expect(k8sClient.Create(testCtx.Ctx, view)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
		})).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
			if cfg.Spec.Desired == nil {
				cfg.Spec.Desired = &parametersv1alpha1.ParameterInputs{}
			}
			if cfg.Spec.Desired.Assignments == nil {
				cfg.Spec.Desired.Assignments = map[string]*string{}
			}
			cfg.Spec.Desired.Assignments["gtid_mode"] = ptr.To("ON")
		})()).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			obj.Spec.Content.Text = strings.Replace(obj.Spec.Content.Text, "gtid_mode=OFF", "gtid_mode=ON", 1)
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewApplyingPhase))
		})).Should(Succeed())
	})

	It("refreshes to Synced when runtime config catches up", func() {
		_, _, _, _, _ = mockReconcileResource()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
		runtimeKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GetComponentCfgName(clusterName, defaultCompName, configSpecName),
		}
		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, runtimeKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("gtid_mode=OFF"))
		})).Should(Succeed())
		Expect(testapps.GetAndChangeObj(&testCtx, runtimeKey, func(cm *corev1.ConfigMap) {
			if cm.Labels == nil {
				cm.Labels = map[string]string{}
			}
			cm.Labels[constant.AppInstanceLabelKey] = clusterName
			cm.Labels[constant.KBAppComponentLabelKey] = defaultCompName
			cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpecName
		})()).Should(Succeed())

		viewKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      "parameterview-phase4-runtime-ready",
		}
		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: viewKey.Namespace,
				Name:      viewKey.Name,
				Labels:    map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cfgKey.Name},
				TemplateName: configSpecName,
				FileName:     testparameters.MysqlConfigFile,
			},
		}
		Expect(k8sClient.Create(testCtx.Ctx, view)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
			g.Expect(obj.Spec.Content.Text).Should(ContainSubstring("gtid_mode=OFF"))
		})).Should(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			contentHash := hashContent(obj.Spec.Content.Text)
			revision := obj.Status.Latest.Revision
			obj.Status.FileFormat = parametersv1alpha1.Ini
			obj.Status.Base = parametersv1alpha1.ParameterViewRevision{
				Revision:    revision,
				ContentHash: contentHash,
			}
			obj.Status.Latest = parametersv1alpha1.ParameterViewRevision{
				Revision:    revision,
				ContentHash: contentHash,
			}
		})()).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			obj.Spec.Content.Text = strings.Replace(obj.Spec.Content.Text, "gtid_mode=OFF", "gtid_mode=ON", 1)
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewApplyingPhase))
		})).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, runtimeKey, func(cm *corev1.ConfigMap) {
			cm.Data[testparameters.MysqlConfigFile] = strings.Replace(cm.Data[testparameters.MysqlConfigFile], "gtid_mode=OFF", "gtid_mode=ON", 1)
		})()).Should(Succeed())
		Expect(testapps.GetAndChangeObj(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			if obj.Annotations == nil {
				obj.Annotations = map[string]string{}
			}
			obj.Annotations["parameters.kubeblocks.io/test-refresh"] = "1"
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
			g.Expect(obj.Spec.Content.Text).Should(ContainSubstring("gtid_mode=ON"))
		})).Should(Succeed())
	})

	It("refreshes to Synced when runtime config drifts without a draft", func() {
		_, _, comp, _, _ := mockReconcileResource()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
		runtimeKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      parameterscore.GetComponentCfgName(clusterName, defaultCompName, configSpecName),
		}
		Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
			g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, runtimeKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("gtid_mode=OFF"))
			g.Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), comp)).Should(Succeed())
			g.Expect(comp.Spec.Configs).Should(HaveLen(1))
			g.Expect(comp.Spec.Configs[0].ConfigMap).ShouldNot(BeNil())
			cfg := &parametersv1alpha1.ComponentParameter{}
			g.Expect(k8sClient.Get(testCtx.Ctx, cfgKey, cfg)).Should(Succeed())
			item := parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)
			g.Expect(item).ShouldNot(BeNil())
			g.Expect(parameters.IsApplyUpdatedParameters(cm, *item, comp.Generation)).Should(BeTrue())
		})).Should(Succeed())
		Expect(testapps.GetAndChangeObj(&testCtx, runtimeKey, func(cm *corev1.ConfigMap) {
			if cm.Labels == nil {
				cm.Labels = map[string]string{}
			}
			cm.Labels[constant.AppInstanceLabelKey] = clusterName
			cm.Labels[constant.KBAppComponentLabelKey] = defaultCompName
			cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpecName
		})()).Should(Succeed())

		viewKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      "parameterview-phase4-runtime-conflict",
		}
		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: viewKey.Namespace,
				Name:      viewKey.Name,
				Labels:    map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cfgKey.Name},
				TemplateName: configSpecName,
				FileName:     testparameters.MysqlConfigFile,
			},
		}
		Expect(k8sClient.Create(testCtx.Ctx, view)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
			g.Expect(obj.Spec.Content.Text).Should(ContainSubstring("gtid_mode=OFF"))
		})).Should(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, viewKey, func(obj *parametersv1alpha1.ParameterView) {
			contentHash := hashContent(obj.Spec.Content.Text)
			revision := obj.Status.Latest.Revision
			obj.Status.FileFormat = parametersv1alpha1.Ini
			obj.Status.Base = parametersv1alpha1.ParameterViewRevision{
				Revision:    revision,
				ContentHash: contentHash,
			}
			obj.Status.Latest = parametersv1alpha1.ParameterViewRevision{
				Revision:    revision,
				ContentHash: contentHash,
			}
		})()).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, runtimeKey, func(cm *corev1.ConfigMap) {
			cm.Data[testparameters.MysqlConfigFile] = strings.Replace(cm.Data[testparameters.MysqlConfigFile], "gtid_mode=OFF", "gtid_mode=ON", 1)
		})()).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, obj *parametersv1alpha1.ParameterView) {
			g.Expect(obj.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.ParameterViewSyncedPhase))
			g.Expect(obj.Spec.Content.Text).Should(ContainSubstring("gtid_mode=ON"))
		})).Should(Succeed())
	})
})

func TestParameterViewSubmissionReplayPreservesRemovalIdentity(t *testing.T) {
	value := "0"
	desired := map[string]*string{
		"default.sync_binlog":     &value,
		"default.max_connections": nil,
		"default.read_only":       nil,
	}
	status := &parametersv1alpha1.ParameterViewStatus{}
	submit := updateSubmissionStatus("1", "sync_binlog=0\n", desired)
	submit(status)
	originalTime := metav1.NewTime(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC))
	originalResult := status.Submissions[0].Result
	originalResult.UpdatedAt = &originalTime
	status.Submissions[0].SubmittedAt = &originalTime
	status.Submissions[0].Result = originalResult
	updates := status.Submissions[0].Updates
	updates[0], updates[1] = updates[1], updates[0]

	submit(status)
	if len(status.Submissions) != 1 {
		t.Fatalf("reordered removals created duplicate history: %#v", status.Submissions)
	}
	got := status.Submissions[0]
	if !got.SubmittedAt.Equal(&originalTime) || !reflect.DeepEqual(got.Result, originalResult) {
		t.Fatalf("reordered removals changed submission time/result: %#v", got)
	}
	wantUpdates := []parametersv1alpha1.ParameterUpdate{
		{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "default.max_connections"},
		{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "default.read_only"},
	}
	if !reflect.DeepEqual(got.Updates, wantUpdates) {
		t.Fatalf("replay did not retain canonical removals: %#v", got.Updates)
	}

	delete(desired, "default.read_only")
	submit(status)
	if len(status.Submissions) != 2 {
		t.Fatalf("different removal keys did not create a distinct submission: %#v", status.Submissions)
	}
}

func TestParameterViewSubmissionHistoryAPI(t *testing.T) {
	environment := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	config, err := environment.Start()
	if err != nil {
		t.Fatalf("start API server: %v", err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Errorf("stop API server: %v", err)
		}
	})
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cli, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	reconciler := &ParameterViewReconciler{Client: cli, Scheme: scheme}
	ctx := context.Background()
	const original = "max_connections=1000\nsync_binlog=1\n"
	for _, tc := range []struct {
		name    string
		edited  string
		removal bool
	}{
		{name: "mixed-set-remove", edited: "sync_binlog=0\n", removal: true},
		{name: "legacy-set-only", edited: "max_connections=1500\nsync_binlog=1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			namespace := "history-" + tc.name
			if err := cli.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}); err != nil {
				t.Fatal(err)
			}
			view := &parametersv1alpha1.ParameterView{
				ObjectMeta: metav1.ObjectMeta{Name: parameterViewName, Namespace: namespace},
				Spec: parametersv1alpha1.ParameterViewSpec{
					ParameterRef: corev1.LocalObjectReference{Name: componentParameterName},
					TemplateName: templateName,
					FileName:     fileName,
				},
			}
			objects := newParameterViewTestObjects(original, view)
			for _, resource := range objects {
				obj := resource.(client.Object)
				if obj.GetNamespace() != "" {
					obj.SetNamespace(namespace)
				}
				switch obj := obj.(type) {
				case *parametersv1alpha1.ComponentParameter:
					obj.Generation = 0
					obj.Spec.ConfigItemDetails[0].ConfigSpec.Namespace = namespace
				case *appsv1.Cluster:
					obj.Spec.TerminationPolicy = appsv1.Delete
				case *appsv1.Component:
					obj.Spec.CompDef += "-" + tc.name
					obj.Spec.Replicas = 1
					obj.Spec.TerminationPolicy = appsv1.Delete
				case *appsv1.ComponentDefinition:
					obj.Name += "-" + tc.name
					obj.Spec.Configs[0].Namespace = namespace
					obj.Spec.Runtime.Containers = []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}}
				case *parametersv1alpha1.ParametersDefinition:
					obj.Name += "-" + tc.name
					obj.Spec.ComponentDef += "-" + tc.name
				case *corev1.ConfigMap:
					if obj.Name != templateConfigMapName {
						obj.Annotations = map[string]string{constant.ConfigurationRevision: "1"}
					}
				}
				if err := cli.Create(ctx, obj); err != nil {
					t.Fatalf("create %T %s: %v", obj, obj.GetName(), err)
				}
				switch obj := obj.(type) {
				case *appsv1.ComponentDefinition:
					obj.Status.Phase = appsv1.AvailablePhase
					if err := cli.Status().Update(ctx, obj); err != nil {
						t.Fatal(err)
					}
				case *parametersv1alpha1.ParametersDefinition:
					obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
					if err := cli.Status().Update(ctx, obj); err != nil {
						t.Fatal(err)
					}
				}
			}
			request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(view)}
			reconcile := func() {
				t.Helper()
				if _, err := reconciler.Reconcile(ctx, request); err != nil {
					t.Fatalf("reconcile: %v", err)
				}
				if err := cli.Get(ctx, request.NamespacedName, view); err != nil {
					t.Fatal(err)
				}
			}
			reconcile()
			if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase || view.Spec.Content.Text != original {
				t.Fatalf("view did not initialize from effective config: %#v", view)
			}
			view.Spec.Content.Text = tc.edited
			if err := cli.Update(ctx, view); err != nil {
				t.Fatal(err)
			}
			reconcile()
			if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase || len(view.Status.Submissions) != 1 {
				t.Fatalf("expected one pending submission: %#v", view.Status)
			}
			if view.Status.Submissions[0].Result.Phase != parametersv1alpha1.ParameterViewSubmissionProcessingPhase {
				t.Fatalf("expected pending execution result: %#v", view.Status.Submissions[0].Result)
			}
			cp := &parametersv1alpha1.ComponentParameter{}
			if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: componentParameterName}, cp); err != nil {
				t.Fatal(err)
			}
			wantAssignments := map[string]*string{}
			var wantUpdates []parametersv1alpha1.ParameterUpdate
			if tc.removal {
				value := "0"
				wantAssignments["default.sync_binlog"] = &value
				wantUpdates = []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "default.max_connections"}}
			} else {
				value := "1500"
				wantAssignments["default.max_connections"] = &value
			}
			if cp.Spec.Desired == nil || !reflect.DeepEqual(cp.Spec.Desired.Assignments, wantAssignments) || !reflect.DeepEqual(cp.Spec.Desired.Updates, wantUpdates) {
				t.Fatalf("desired operations were not persisted: %#v", cp.Spec.Desired)
			}
			persisted := &apiunstructured.Unstructured{}
			persisted.SetGroupVersionKind(parametersv1alpha1.GroupVersion.WithKind("ParameterView"))
			if err := cli.Get(ctx, request.NamespacedName, persisted); err != nil {
				t.Fatal(err)
			}
			submissions, _, err := apiunstructured.NestedSlice(persisted.Object, "status", "submissions")
			if err != nil || len(submissions) != 1 {
				t.Fatalf("read persisted submissions: %v: %#v", err, submissions)
			}
			submission := submissions[0].(map[string]interface{})
			assignments := submission["assignments"].(map[string]interface{})
			if tc.removal {
				want := []interface{}{map[string]interface{}{"type": "Remove", "key": "default.max_connections"}}
				if !reflect.DeepEqual(submission["updates"], want) {
					t.Errorf("persisted history lost removal operation: updates = %#v, want %#v", submission["updates"], want)
				}
				if _, found := assignments["default.max_connections"]; found {
					t.Errorf("removed key remained in history assignments: %#v", assignments)
				}
			} else if _, found := submission["updates"]; found {
				t.Fatalf("legacy Set-only submission unexpectedly has updates: %#v", submission)
			}
			for key, value := range wantAssignments {
				if assignments[key] != *value {
					t.Errorf("history assignment %s = %#v, want %s", key, assignments[key], *value)
				}
			}
			submittedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339)
			updatedAt := time.Date(2025, 1, 2, 3, 5, 6, 0, time.UTC).Format(time.RFC3339)
			submission["submittedAt"] = submittedAt
			result := submission["result"].(map[string]interface{})
			result["updatedAt"] = updatedAt
			wantResult := runtime.DeepCopyJSONValue(result)
			if err := apiunstructured.SetNestedSlice(persisted.Object, submissions, "status", "submissions"); err != nil {
				t.Fatal(err)
			}
			if err := cli.Status().Update(ctx, persisted); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(ctx, request.NamespacedName, persisted); err != nil {
				t.Fatal(err)
			}
			submissions, _, err = apiunstructured.NestedSlice(persisted.Object, "status", "submissions")
			if err != nil || len(submissions) != 1 {
				t.Fatalf("read seeded history: %v: %#v", err, submissions)
			}
			submission = submissions[0].(map[string]interface{})
			if submission["submittedAt"] != submittedAt || !reflect.DeepEqual(submission["result"], wantResult) {
				t.Fatalf("submission time/result were not persisted before retry: %#v", submission)
			}
			reconcile()
			if err := cli.Get(ctx, request.NamespacedName, persisted); err != nil {
				t.Fatal(err)
			}
			submissions, _, err = apiunstructured.NestedSlice(persisted.Object, "status", "submissions")
			if err != nil || len(submissions) != 1 {
				t.Fatalf("repeated pending reconcile created duplicate history: %v: %#v", err, submissions)
			}
			submission = submissions[0].(map[string]interface{})
			if submission["submittedAt"] != submittedAt || !reflect.DeepEqual(submission["result"], wantResult) {
				t.Errorf("repeat changed original submission time/result: %#v", submission)
			}
		})
	}
}
