/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package parameters

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

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
			persisted := &unstructured.Unstructured{}
			persisted.SetGroupVersionKind(parametersv1alpha1.GroupVersion.WithKind("ParameterView"))
			if err := cli.Get(ctx, request.NamespacedName, persisted); err != nil {
				t.Fatal(err)
			}
			submissions, _, err := unstructured.NestedSlice(persisted.Object, "status", "submissions")
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
			if err := unstructured.SetNestedSlice(persisted.Object, submissions, "status", "submissions"); err != nil {
				t.Fatal(err)
			}
			if err := cli.Status().Update(ctx, persisted); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(ctx, request.NamespacedName, persisted); err != nil {
				t.Fatal(err)
			}
			submissions, _, err = unstructured.NestedSlice(persisted.Object, "status", "submissions")
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
			submissions, _, err = unstructured.NestedSlice(persisted.Object, "status", "submissions")
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
