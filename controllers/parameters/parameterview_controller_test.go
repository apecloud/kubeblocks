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
	"testing"

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
)

func TestParameterViewReconcileInitializesPlainTextContent(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"runtime-value=1\n",
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
	if view.Spec.Content.Text != "runtime-value=1\n" {
		t.Fatalf("expected content to be backfilled from generated configmap, got %q", view.Spec.Content.Text)
	}
	if view.Spec.FileFormat != parametersv1alpha1.Ini {
		t.Fatalf("expected fileFormat %q, got %q", parametersv1alpha1.Ini, view.Spec.FileFormat)
	}
	if view.Spec.SourceGeneration != componentParameterGeneration {
		t.Fatalf("expected sourceGeneration %d, got %d", componentParameterGeneration, view.Spec.SourceGeneration)
	}
	if view.Spec.ContentHash != hashContent("runtime-value=1\n") {
		t.Fatalf("unexpected contentHash: %q", view.Spec.ContentHash)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewReadyPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewReadyPhase, view.Status.Phase)
	}
	if view.Status.ObservedGeneration != view.Generation {
		t.Fatalf("expected observedGeneration %d, got %d", view.Generation, view.Status.ObservedGeneration)
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
}

func TestParameterViewReconcileMarksConflictOnSourceDrift(t *testing.T) {
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjects(
		"runtime-value=2\n",
		&parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parameterViewName,
				Namespace:  parameterViewNamespace,
				Generation: 2,
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef:     corev1.LocalObjectReference{Name: componentParameterName},
				TemplateName:     templateName,
				FileName:         fileName,
				FileFormat:       parametersv1alpha1.Ini,
				SourceGeneration: 1,
				ContentHash:      hashContent("runtime-value=1\n"),
				Content:          parametersv1alpha1.ParameterViewContent{Type: parametersv1alpha1.PlainTextParameterViewContentType, Text: "user-edited=1\n"},
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
	if view.Status.Phase != parametersv1alpha1.ParameterViewConflictPhase {
		t.Fatalf("expected phase %q, got %q", parametersv1alpha1.ParameterViewConflictPhase, view.Status.Phase)
	}
	if view.Status.Message == "" {
		t.Fatalf("expected conflict message to be set")
	}
	if view.Spec.Content.Text != "user-edited=1\n" {
		t.Fatalf("expected user content to be preserved on conflict, got %q", view.Spec.Content.Text)
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

func newParameterViewTestObjects(runtimeContent string, view *parametersv1alpha1.ParameterView) []runtime.Object {
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
					fileName: {
						Content: ptr.To(runtimeContent),
					},
				},
			}},
		},
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
			FileName:     fileName,
			FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
			},
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
			fileName: "template-value=1\n",
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
			fileName: runtimeContent,
		},
	}

	return []runtime.Object{
		view,
		componentParameter,
		cluster,
		component,
		componentDef,
		parametersDef,
		template,
		rendered,
	}
}
