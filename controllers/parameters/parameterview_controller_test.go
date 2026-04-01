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

package parameters

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
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
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "max_connections=1500\n", parametersv1alpha1.ParameterValueMap{
		"default.max_connections": ptr.To("1500"),
	})

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if got := compParam.Spec.Desired.Parameters["default.max_connections"]; got == nil || *got != "1500" {
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
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "max_connections=1500\n", parametersv1alpha1.ParameterValueMap{
		"default.max_connections": ptr.To("1500"),
	})

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Parameters == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Parameters["default.max_connections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired default.max_connections=1500, got %#v", got)
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
	if got := compParam.Spec.Desired.Parameters["mysqld.max_connections"]; got == nil || *got != "1500" {
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
	if compParam.Spec.Desired != nil && len(compParam.Spec.Desired.Parameters) != 0 {
		t.Fatalf("expected desired parameters to remain unchanged, got %#v", compParam.Spec.Desired.Parameters)
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
	if got := compParam.Spec.Desired.Parameters["mysqld.sync_binlog"]; got == nil || *got != "2" {
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
			cp.Spec.Desired = &parametersv1alpha1.ParameterValues{
				Parameters: parametersv1alpha1.ParameterValueMap{
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
					Parameters: parametersv1alpha1.ParameterValueMap{
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
			cp.Spec.Desired = &parametersv1alpha1.ParameterValues{
				Parameters: parametersv1alpha1.ParameterValueMap{
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
					Parameters: parametersv1alpha1.ParameterValueMap{
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
			cp.Spec.Desired = &parametersv1alpha1.ParameterValues{
				Parameters: parametersv1alpha1.ParameterValueMap{
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
			cp.Spec.Desired = &parametersv1alpha1.ParameterValues{
				Parameters: parametersv1alpha1.ParameterValueMap{
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
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "max_connections=1200\n", parametersv1alpha1.ParameterValueMap{
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
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Parameters == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Parameters["default.max_connections"]; got == nil || *got != "1200" {
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
				cp.Spec.Desired = &parametersv1alpha1.ParameterValues{
					Parameters: parametersv1alpha1.ParameterValueMap{
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
	assertParameterViewLastSubmission(t, view, fmt.Sprint(componentParameterGeneration), "maxConnections: \"1500\"\n", parametersv1alpha1.ParameterValueMap{
		"maxConnections": ptr.To("1500"),
	})

	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Parameters == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Parameters["maxConnections"]; got == nil || *got != "1500" {
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
	if compParam.Spec.Desired == nil || compParam.Spec.Desired.Parameters == nil {
		t.Fatalf("expected desired parameters to be written")
	}
	if got := compParam.Spec.Desired.Parameters["maxconnections"]; got == nil || *got != "1500" {
		t.Fatalf("expected desired maxconnections=1500, got %#v", got)
	}
}

func assertParameterViewLastSubmission(t *testing.T, view *parametersv1alpha1.ParameterView, revision, content string, parameters parametersv1alpha1.ParameterValueMap) {
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
	if !equalParameterValueMap(got.Parameters, parameters) {
		t.Fatalf("expected latest submission parameters %#v, got %#v", parameters, got.Parameters)
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
			Parameters: parametersv1alpha1.ParameterValueMap{
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
			g.Expect(cfg.Spec.Desired.Parameters).Should(HaveKeyWithValue("gtid_mode", ptr.To("ON")))
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
			if cfg.Spec.Desired == nil || cfg.Spec.Desired.Parameters == nil {
				return
			}
			g.Expect(cfg.Spec.Desired.Parameters).ShouldNot(HaveKey("mysqld.gtid_mode"))
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
				cfg.Spec.Desired = &parametersv1alpha1.ParameterValues{}
			}
			if cfg.Spec.Desired.Parameters == nil {
				cfg.Spec.Desired.Parameters = parametersv1alpha1.ParameterValueMap{}
			}
			cfg.Spec.Desired.Parameters["gtid_mode"] = ptr.To("ON")
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
