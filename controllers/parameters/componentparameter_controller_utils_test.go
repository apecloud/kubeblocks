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
	"errors"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	parampkg "github.com/apecloud/kubeblocks/pkg/parameters"
)

type countingGetClient struct {
	client.Client
	err      error
	getCount int
}

func (c *countingGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCount++
	return c.err
}

func TestNormalizeManagedParameterInputs(t *testing.T) {
	t.Run("updates override assignments and remove is explicit", func(t *testing.T) {
		inputs := &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{
				"max_connections": ptr.To("1000"),
				"sync_binlog":     ptr.To("1"),
			},
			Updates: []parametersv1alpha1.ParameterUpdate{
				{Type: parametersv1alpha1.ParameterUpdateSet, Key: "max_connections", Value: ptr.To("2000")},
				{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "sync_binlog"},
			},
		}

		got, err := normalizeManagedParameterInputs(inputs)
		if err != nil {
			t.Fatalf("normalizeManagedParameterInputs() error = %v", err)
		}
		if got["max_connections"] == nil || *got["max_connections"] != "2000" {
			t.Fatalf("expected max_connections to be overridden to 2000, got %#v", got["max_connections"])
		}
		if _, ok := got["sync_binlog"]; !ok {
			t.Fatalf("expected sync_binlog remove marker to be preserved")
		}
		if got["sync_binlog"] != nil {
			t.Fatalf("expected sync_binlog to normalize to nil remove marker, got %#v", got["sync_binlog"])
		}
	})

	t.Run("set without value is rejected", func(t *testing.T) {
		_, err := normalizeManagedParameterInputs(&parametersv1alpha1.ParameterInputs{
			Updates: []parametersv1alpha1.ParameterUpdate{{
				Type: parametersv1alpha1.ParameterUpdateSet,
				Key:  "max_connections",
			}},
		})
		if err == nil {
			t.Fatalf("expected error for set update without value")
		}
	})
}

func TestMergeItemParameters(t *testing.T) {
	t.Run("override replaces managed parameter overlay for a file", func(t *testing.T) {
		item := &parametersv1alpha1.ConfigTemplateItemDetail{
			ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
				"my.cnf": {
					Content: ptr.To("[mysqld]\nmax_connections=1000\n"),
					Parameters: map[string]*string{
						"max_connections": ptr.To("1000"),
						"sync_binlog":     ptr.To("1"),
					},
				},
			},
		}
		updated := map[string]parametersv1alpha1.ParametersInFile{
			"my.cnf": {
				Parameters: map[string]*string{
					"max_connections": nil,
				},
			},
		}

		mergeItemParameters(item, updated, true)

		got := item.ConfigFileParams["my.cnf"]
		if got.Content == nil || *got.Content != "[mysqld]\nmax_connections=1000\n" {
			t.Fatalf("expected non-managed fields to be preserved, got %#v", got.Content)
		}
		if len(got.Parameters) != 1 {
			t.Fatalf("expected managed overlay to be replaced, got %#v", got.Parameters)
		}
		decoded := parampkg.DecodeParameterOverlay(got.Parameters)
		if _, ok := decoded["max_connections"]; !ok {
			t.Fatalf("expected max_connections remove marker to be kept")
		}
		if decoded["max_connections"] != nil {
			t.Fatalf("expected max_connections to be overridden to nil remove marker, got %#v", decoded["max_connections"])
		}
	})
}

func TestMergeMissingConfigFileParams(t *testing.T) {
	dest := &parametersv1alpha1.ConfigTemplateItemDetail{
		ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
			"my.cnf": {
				Parameters: map[string]*string{
					"max_connections": ptr.To("2000"),
				},
			},
		},
	}
	expected := &parametersv1alpha1.ConfigTemplateItemDetail{
		ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
			"my.cnf": {
				Parameters: map[string]*string{
					"max_connections": ptr.To("1000"),
				},
			},
			"log.conf": {
				Parameters: map[string]*string{
					"slow_query_log": ptr.To("1"),
				},
			},
		},
	}

	mergeMissingConfigFileParams(dest, expected)

	if got := dest.ConfigFileParams["my.cnf"].Parameters["max_connections"]; got == nil || *got != "2000" {
		t.Fatalf("expected existing file params to be preserved, got %#v", got)
	}
	if _, ok := dest.ConfigFileParams["log.conf"]; !ok {
		t.Fatalf("expected missing legacy file params to be merged")
	}

	*expected.ConfigFileParams["log.conf"].Parameters["slow_query_log"] = "0"
	if got := dest.ConfigFileParams["log.conf"].Parameters["slow_query_log"]; got == nil || *got != "1" {
		t.Fatalf("expected merged params to be deep-copied, got %#v", got)
	}
}

func TestReconcileConfigItemDetailsExternalManagedEmptyTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(core) error = %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(apps) error = %v", err)
	}
	if err := parametersv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(parameters) error = %v", err)
	}

	compDef := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "obce"},
		Spec: appsv1.ComponentDefinitionSpec{
			Configs: []appsv1.ComponentFileTemplate{{
				Name:            "oceanbase-sysvars",
				Template:        "",
				ExternalManaged: ptr.To(true),
			}},
		},
	}
	paramsDef := &parametersv1alpha1.ParametersDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "obce-sysvars"},
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			ComponentDef: compDef.Name,
			TemplateName: "oceanbase-sysvars",
			FileName:     "sysvars.conf",
			FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Properties,
			},
		},
		Status: parametersv1alpha1.ParametersDefinitionStatus{
			Phase: parametersv1alpha1.PDAvailablePhase,
		},
	}
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "obce-oceanbase",
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{
			ClusterName:   "obce",
			ComponentName: "oceanbase",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(compDef, paramsDef, compParam).Build()

	updated, err := reconcileConfigItemDetailsIntoSpec(context.Background(), cli, compParam, &Task{
		ResourceFetcher: parampkg.ResourceFetcher[Task]{ComponentDefObj: compDef},
	})
	if err != nil {
		t.Fatalf("reconcileConfigItemDetailsIntoSpec() error = %v", err)
	}
	if !updated {
		t.Fatalf("expected ConfigItemDetails update for externalManaged config with empty template")
	}
	if got := compParam.Spec.ConfigItemDetails; len(got) != 1 || got[0].Name != "oceanbase-sysvars" {
		t.Fatalf("unexpected ConfigItemDetails: %#v", got)
	}
	if compParam.Spec.ConfigItemDetails[0].ConfigSpec == nil || compParam.Spec.ConfigItemDetails[0].ConfigSpec.Template != "" {
		t.Fatalf("expected empty-template config spec to be preserved, got %#v", compParam.Spec.ConfigItemDetails[0].ConfigSpec)
	}
}

func TestExternalManagedEmptyTemplateOnlyUpdatesStatus(t *testing.T) {
	item := parametersv1alpha1.ConfigTemplateItemDetail{
		Name: "oceanbase-sysvars",
		ConfigSpec: &appsv1.ComponentFileTemplate{
			Name:            "oceanbase-sysvars",
			ExternalManaged: ptr.To(true),
		},
	}
	status := &parametersv1alpha1.ConfigTemplateItemDetailStatus{
		Name:  item.Name,
		Phase: parametersv1alpha1.CInitPhase,
	}

	t.Run("task does not read the runtime ConfigMap", func(t *testing.T) {
		taskStatus := status.DeepCopy()
		task := newTask(item, taskStatus, 7)
		if err := task.Do(&Task{}, nil, "revision-1"); err != nil {
			t.Fatalf("Task.Do() error = %v", err)
		}
		if taskStatus.Phase != parametersv1alpha1.CFinishedPhase || taskStatus.UpdateRevision != "revision-1" ||
			taskStatus.LastDoneRevision != "revision-1" || taskStatus.Message != nil {
			t.Fatalf("expected status-only merge for delegated config, got %#v", taskStatus)
		}
		beforeReplay := taskStatus.DeepCopy()
		if err := task.Do(&Task{}, nil, "revision-1"); err != nil {
			t.Fatalf("Task.Do() replay error = %v", err)
		}
		if !reflect.DeepEqual(taskStatus, beforeReplay) {
			t.Fatalf("expected exact status replay, before = %#v, after = %#v", beforeReplay, taskStatus)
		}
	})

	t.Run("sync implementation keeps the same defensive contract", func(t *testing.T) {
		syncStatus := status.DeepCopy()
		fetcher := &Task{ResourceFetcher: parampkg.ResourceFetcher[Task]{
			ComponentObj: &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Generation: 7}},
		}}
		if err := syncImpl(&taskContext{}, fetcher, item, syncStatus, "revision-1", nil); err != nil {
			t.Fatalf("syncImpl() error = %v", err)
		}
		if syncStatus.Phase != parametersv1alpha1.CFinishedPhase || syncStatus.UpdateRevision != "revision-1" ||
			syncStatus.LastDoneRevision != "revision-1" || syncStatus.Message != nil {
			t.Fatalf("expected status-only merge for delegated config, got %#v", syncStatus)
		}
	})

	t.Run("adding a template re-enters the normal task path", func(t *testing.T) {
		withTemplate := item
		withTemplate.ConfigSpec = item.ConfigSpec.DeepCopy()
		withTemplate.ConfigSpec.Template = "default-template"
		evolvedStatus := &parametersv1alpha1.ConfigTemplateItemDetailStatus{
			Name:             item.Name,
			Phase:            parametersv1alpha1.CFinishedPhase,
			UpdateRevision:   "revision-1",
			LastDoneRevision: "revision-1",
		}
		before := evolvedStatus.DeepCopy()
		lookupErr := errors.New("runtime ConfigMap lookup reached")
		spy := &countingGetClient{err: lookupErr}
		resource := &Task{}
		resource.Init(&render.ResourceCtx{
			Context:       context.Background(),
			Client:        spy,
			Namespace:     "default",
			ClusterName:   "obce",
			ComponentName: "oceanbase",
		}, resource)

		task := newTask(withTemplate, evolvedStatus, 7)
		if err := task.Do(resource, nil, "revision-2"); !errors.Is(err, lookupErr) {
			t.Fatalf("expected normal runtime ConfigMap lookup error, got %v", err)
		}
		if spy.getCount != 1 {
			t.Fatalf("expected one runtime ConfigMap lookup, got %d", spy.getCount)
		}
		if !reflect.DeepEqual(evolvedStatus, before) {
			t.Fatalf("failed normal lookup must not fake a new terminal revision: before = %#v, after = %#v", before, evolvedStatus)
		}
	})
}

func TestMergeAndApplyConfigDoesNotWriteExternalManagedEmptyTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(core) error = %v", err)
	}
	item := parametersv1alpha1.ConfigTemplateItemDetail{
		Name: "external-config",
		ConfigSpec: &appsv1.ComponentFileTemplate{
			Name:            "external-config",
			ExternalManaged: ptr.To(true),
		},
	}
	owner := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{Name: "params", Namespace: "default", UID: "owner-uid"},
	}

	t.Run("does not update an existing user ConfigMap", func(t *testing.T) {
		running := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "user-config", Namespace: "default"},
			Data:       map[string]string{"sysvars.conf": "user-value"},
		}
		expected := running.DeepCopy()
		expected.Data["sysvars.conf"] = "controller-value"
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
		resourceCtx := &render.ResourceCtx{Context: context.Background(), Client: cli}

		if err := mergeAndApplyConfig(resourceCtx, expected, running, owner, item, 7, "revision-1"); err != nil {
			t.Fatalf("mergeAndApplyConfig() error = %v", err)
		}
		got := &corev1.ConfigMap{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), got); err != nil {
			t.Fatalf("get ConfigMap error = %v", err)
		}
		if got.Data["sysvars.conf"] != "user-value" || len(got.OwnerReferences) != 0 || len(got.Finalizers) != 0 {
			t.Fatalf("external ConfigMap was mutated: %#v", got)
		}
	})

	t.Run("does not create a user-owned ConfigMap", func(t *testing.T) {
		expected := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "user-config", Namespace: "default"},
			Data:       map[string]string{"sysvars.conf": "controller-value"},
		}
		cli := fake.NewClientBuilder().WithScheme(scheme).Build()
		resourceCtx := &render.ResourceCtx{Context: context.Background(), Client: cli}

		if err := mergeAndApplyConfig(resourceCtx, expected, nil, owner, item, 7, "revision-1"); err != nil {
			t.Fatalf("mergeAndApplyConfig() error = %v", err)
		}
		got := &corev1.ConfigMap{}
		err := cli.Get(context.Background(), client.ObjectKeyFromObject(expected), got)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected external ConfigMap to remain absent, got %#v, error = %v", got, err)
		}
	})

	t.Run("keeps the ordinary managed update path", func(t *testing.T) {
		managedItem := item
		managedItem.ConfigSpec = item.ConfigSpec.DeepCopy()
		managedItem.ConfigSpec.ExternalManaged = nil
		running := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "managed-config", Namespace: "default"},
			Data:       map[string]string{"my.cnf": "old-value"},
		}
		expected := running.DeepCopy()
		expected.Data["my.cnf"] = "new-value"
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
		resourceCtx := &render.ResourceCtx{Context: context.Background(), Client: cli}

		if err := mergeAndApplyConfig(resourceCtx, expected, running, owner, managedItem, 7, "revision-1"); err != nil {
			t.Fatalf("mergeAndApplyConfig() error = %v", err)
		}
		got := &corev1.ConfigMap{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), got); err != nil {
			t.Fatalf("get ConfigMap error = %v", err)
		}
		if got.Data["my.cnf"] != "new-value" {
			t.Fatalf("expected managed ConfigMap update, got %#v", got.Data)
		}
	})

	t.Run("keeps the parameter-managed path when an external template exists", func(t *testing.T) {
		withTemplate := item
		withTemplate.ConfigSpec = item.ConfigSpec.DeepCopy()
		withTemplate.ConfigSpec.Template = "default-template"
		running := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "parameter-managed-config", Namespace: "default"},
			Data:       map[string]string{"my.cnf": "old-value"},
		}
		expected := running.DeepCopy()
		expected.Data["my.cnf"] = "new-value"
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
		resourceCtx := &render.ResourceCtx{Context: context.Background(), Client: cli}

		if err := mergeAndApplyConfig(resourceCtx, expected, running, owner, withTemplate, 7, "revision-1"); err != nil {
			t.Fatalf("mergeAndApplyConfig() error = %v", err)
		}
		got := &corev1.ConfigMap{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), got); err != nil {
			t.Fatalf("get ConfigMap error = %v", err)
		}
		if got.Data["my.cnf"] != "new-value" {
			t.Fatalf("expected parameter-managed ConfigMap update, got %#v", got.Data)
		}
	})
}

func TestReconcileConfigItemDetailsRequiresOrdinaryTemplateConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(core) error = %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(apps) error = %v", err)
	}
	if err := parametersv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(parameters) error = %v", err)
	}

	compDef := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
		Spec: appsv1.ComponentDefinitionSpec{
			Configs: []appsv1.ComponentFileTemplate{{
				Name:      "mysql-config",
				Namespace: "default",
				Template:  "runtime-template",
			}},
		},
	}
	paramsDef := &parametersv1alpha1.ParametersDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-config"},
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			ComponentDef: compDef.Name,
			TemplateName: "mysql-config",
			FileName:     "my.cnf",
			FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Properties,
			},
		},
		Status: parametersv1alpha1.ParametersDefinitionStatus{
			Phase: parametersv1alpha1.PDAvailablePhase,
		},
	}
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "mysql",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(compDef, paramsDef, compParam).Build()

	_, err := reconcileConfigItemDetailsIntoSpec(context.Background(), cli, compParam, &Task{
		ResourceFetcher: parampkg.ResourceFetcher[Task]{ComponentDefObj: compDef},
	})
	if err == nil || !strings.Contains(err.Error(), `configmaps "runtime-template" not found`) {
		t.Fatalf("expected ordinary template ConfigMap lookup error, got %v", err)
	}
}

func TestNilEmptyConfigItemDetailsEquivalence(t *testing.T) {
	t.Run("nil and empty ConfigItemDetails should be treated as equal", func(t *testing.T) {
		merged := parampkg.MergeComponentParameter(
			&parametersv1alpha1.ComponentParameter{},
			&parametersv1alpha1.ComponentParameter{},
			func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {},
		)
		var nilDetails []parametersv1alpha1.ConfigTemplateItemDetail
		emptyDetails := merged.Spec.ConfigItemDetails
		if len(nilDetails) != 0 || len(emptyDetails) != 0 {
			t.Fatalf("expected both to be empty, got nil=%d empty=%d", len(nilDetails), len(emptyDetails))
		}
		bothEmpty := len(nilDetails) == 0 && len(emptyDetails) == 0
		if !bothEmpty {
			t.Fatalf("nil and empty ConfigItemDetails should be treated as equal")
		}
	})
}
