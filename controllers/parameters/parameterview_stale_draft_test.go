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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
)

func TestParameterViewReconcilePreservesStaleDraftAfterConcurrentEdit(t *testing.T) {
	const (
		originalContent = "[mysqld]\nmax_connections=1000\nsync_binlog=1\n"
		draftContent    = "[mysqld]\nmax_connections=1500\nsync_binlog=1\n"
		latestContent   = "[mysqld]\nmax_connections=1000\nsync_binlog=2\n"
		otherViewName   = "other-view"
		latestRevision  = "8"
		parametersCUE   = "#Parameters: {mysqld: {\nmax_connections: int & >=1\nsync_binlog: int & >=0\n}}\nconfiguration: #Parameters\n"
	)
	parametersSchema, err := openapi.GenerateOpenAPISchema(parametersCUE, "Parameters")
	if err != nil {
		t.Fatalf("generate parameters schema failed: %v", err)
	}
	initialView := &parametersv1alpha1.ParameterView{
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
	}
	reconciler, cli := newParameterViewTestReconciler(t, newParameterViewTestObjectsWithOptions(parameterViewTestOptions{
		runtimeContent:  originalContent,
		templateContent: originalContent,
		dynamicParameters: []string{
			"mysqld.max_connections",
			"mysqld.sync_binlog",
		},
		parametersSchema: &parametersv1alpha1.ParametersSchema{
			CUE:          parametersCUE,
			SchemaInJSON: parametersSchema,
		},
		view: initialView,
	})...)
	ctx := context.Background()
	getView := func(name string) *parametersv1alpha1.ParameterView {
		t.Helper()
		view := &parametersv1alpha1.ParameterView{}
		if err := cli.Get(ctx, types.NamespacedName{Namespace: parameterViewNamespace, Name: name}, view); err != nil {
			t.Fatalf("get view %s failed: %v", name, err)
		}
		return view
	}
	reconcileView := func(name string) {
		t.Helper()
		if _, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: parameterViewNamespace, Name: name},
		}); err != nil {
			t.Fatalf("reconcile view %s failed: %v", name, err)
		}
	}
	otherView := initialView.DeepCopy()
	otherView.Name = otherViewName
	otherView.ResourceVersion = ""
	if err := cli.Create(ctx, otherView); err != nil {
		t.Fatalf("create concurrent view failed: %v", err)
	}
	for _, name := range []string{parameterViewName, otherViewName} {
		reconcileView(name)
		view := getView(name)
		if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase || view.Spec.Content.Text != originalContent {
			t.Fatalf("expected view %s initialized from original content, got %#v", name, view)
		}
	}
	view := getView(parameterViewName)
	base := view.Status.Base
	view.Spec.Content.Text = draftContent
	view.Generation++
	if err := cli.Update(ctx, view); err != nil {
		t.Fatalf("edit stale draft failed: %v", err)
	}
	draft := view.Spec.Content
	otherView = getView(otherViewName)
	otherView.Spec.Content.Text = latestContent
	otherView.Generation++
	if err := cli.Update(ctx, otherView); err != nil {
		t.Fatalf("edit concurrent view failed: %v", err)
	}
	reconcileView(otherViewName)
	otherView = getView(otherViewName)
	if otherView.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected concurrent edit to be submitted, got phase %q with message %q", otherView.Status.Phase, otherView.Status.Message)
	}
	compParamKey := types.NamespacedName{Namespace: parameterViewNamespace, Name: componentParameterName}
	compParam := &parametersv1alpha1.ComponentParameter{}
	if err := cli.Get(ctx, compParamKey, compParam); err != nil {
		t.Fatalf("get component parameter failed: %v", err)
	}
	if compParam.Spec.Desired == nil || len(compParam.Spec.Desired.Assignments) != 1 {
		t.Fatalf("expected only the concurrent sync_binlog assignment, got %#v", compParam.Spec.Desired)
	}
	if got := compParam.Spec.Desired.Assignments["mysqld.sync_binlog"]; got == nil || *got != "2" {
		t.Fatalf("expected concurrent edit to submit sync_binlog=2, got %#v", got)
	}
	desired := compParam.DeepCopy().Spec.Desired
	rendered := &corev1.ConfigMap{}
	renderedKey := types.NamespacedName{
		Namespace: parameterViewNamespace,
		Name:      parameterscore.GetComponentCfgName(pvClusterName, pvComponentName, templateName),
	}
	if err := cli.Get(ctx, renderedKey, rendered); err != nil {
		t.Fatalf("get rendered configmap failed: %v", err)
	}
	rendered.Data[fileName] = latestContent
	rendered.Annotations = map[string]string{constant.ConfigurationRevision: latestRevision}
	if err := cli.Update(ctx, rendered); err != nil {
		t.Fatalf("update rendered configmap failed: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		reconcileView(parameterViewName)
		view = getView(parameterViewName)
		if err := cli.Get(ctx, compParamKey, compParam); err != nil {
			t.Fatalf("get component parameter after stale reconciliation failed: %v", err)
		}
		if !reflect.DeepEqual(compParam.Spec.Desired, desired) {
			t.Errorf("reconcile %d: stale draft changed concurrent desired parameters: want %#v, got %#v", attempt, desired.Assignments, compParam.Spec.Desired.Assignments)
		}
		if got := compParam.Spec.Desired.Assignments["mysqld.sync_binlog"]; got == nil {
			t.Errorf("reconcile %d: concurrent sync_binlog=2 assignment was removed", attempt)
		} else if *got != "2" {
			t.Errorf("reconcile %d: stale draft overwrote concurrent sync_binlog=2 with %q", attempt, *got)
		}
		if view.Status.ObservedGeneration != view.Generation {
			t.Errorf("reconcile %d: expected observed generation %d, got %d", attempt, view.Generation, view.Status.ObservedGeneration)
		}
		if view.Status.Phase != parametersv1alpha1.ParameterViewConflictPhase {
			t.Errorf("reconcile %d: expected Conflict for stale draft, got %q", attempt, view.Status.Phase)
		}
		if view.Spec.Content != draft || view.Status.Base != base {
			t.Errorf("reconcile %d: expected draft and base to remain unchanged, got content %#v and base %#v", attempt, view.Spec.Content, view.Status.Base)
		}
		if view.Status.Latest.Revision != latestRevision || view.Status.Latest.ContentHash != hashContent(latestContent) {
			t.Errorf("reconcile %d: expected latest to track concurrent runtime revision, got %#v", attempt, view.Status.Latest)
		}
		if len(view.Status.Submissions) != 0 {
			t.Errorf("reconcile %d: stale draft created a parameter submission: %#v", attempt, view.Status.Submissions)
		}
		conditionFound := false
		for _, condition := range view.Status.Conditions {
			if condition.Type == parameterViewSyncedCondition {
				conditionFound = true
				if condition.Status != metav1.ConditionFalse || condition.Reason != parameterViewReasonDraftOutdated {
					t.Errorf("reconcile %d: expected DraftOutdated condition, got %q", attempt, condition.Reason)
				}
			}
		}
		if !conditionFound {
			t.Errorf("reconcile %d: synced condition is missing", attempt)
		}
	}
	view.Spec.ResetToLatest = true
	view.Generation++
	if err := cli.Update(ctx, view); err != nil {
		t.Fatalf("request reset failed: %v", err)
	}
	reconcileView(parameterViewName)
	view = getView(parameterViewName)
	if view.Spec.ResetToLatest || view.Spec.Content.Text != latestContent || view.Status.Base != view.Status.Latest {
		t.Fatalf("expected reset to refresh content and base and clear the flag, got spec %#v and base %#v", view.Spec, view.Status.Base)
	}
	if view.Status.Phase != parametersv1alpha1.ParameterViewSyncedPhase || len(view.Status.Submissions) != 0 {
		t.Fatalf("expected reset to sync without submitting parameters, got status %#v", view.Status)
	}
	if err := cli.Get(ctx, compParamKey, compParam); err != nil {
		t.Fatalf("get component parameter after reset failed: %v", err)
	}
	if !reflect.DeepEqual(compParam.Spec.Desired, desired) {
		t.Fatalf("reset changed concurrent desired parameters: %#v", compParam.Spec.Desired)
	}
	const recoveredContent = "[mysqld]\nmax_connections=1500\nsync_binlog=2\n"
	view.Spec.Content.Text = recoveredContent
	view.Generation++
	if err := cli.Update(ctx, view); err != nil {
		t.Fatalf("reapply saved edit failed: %v", err)
	}
	reconcileView(parameterViewName)
	view = getView(parameterViewName)
	if view.Status.Phase != parametersv1alpha1.ParameterViewApplyingPhase {
		t.Fatalf("expected reapplied edit to be submitted, got phase %q with message %q", view.Status.Phase, view.Status.Message)
	}
	if err := cli.Get(ctx, compParamKey, compParam); err != nil {
		t.Fatalf("get component parameter after recovery failed: %v", err)
	}
	for key, value := range map[string]string{"mysqld.max_connections": "1500", "mysqld.sync_binlog": "2"} {
		if got := compParam.Spec.Desired.Assignments[key]; got == nil || *got != value {
			t.Errorf("expected recovered desired %s=%s, got %#v", key, value, got)
		}
	}
	if len(view.Status.Submissions) != 1 {
		t.Fatalf("expected one submission after recovery, got %#v", view.Status.Submissions)
	}
}
