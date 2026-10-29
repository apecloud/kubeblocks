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
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	parampkg "github.com/apecloud/kubeblocks/pkg/parameters"
)

func TestReconcileInitialImmutableArray(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme} {
		if err := addToScheme(scheme); err != nil {
			t.Fatal(err)
		}
	}
	configSpec := appsv1.ComponentFileTemplate{Name: "config", Template: "template", Namespace: "default", ExternalManaged: ptr.To(true)}
	compDef := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "array-component"},
		Spec:       appsv1.ComponentDefinitionSpec{Configs: []appsv1.ComponentFileTemplate{configSpec}},
		Status:     appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
	}
	// Schema-free definitions preserve array field prefixes through the default classifier.
	paramsDef := &parametersv1alpha1.ParametersDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "array-parameters"},
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			ComponentDef:        compDef.Name,
			TemplateName:        configSpec.Name,
			FileName:            "config.json",
			FileFormatConfig:    &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.JSON},
			ImmutableParameters: []string{"hosts"},
		},
		Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
	}
	baseConfig := &corev1.ConfigMap{Data: map[string]string{"config.json": `{"hosts":["template"],"mode":"default"}`}}
	current := baseConfig.DeepCopy()
	current.ObjectMeta = metav1.ObjectMeta{
		Name: "rendered-config", Namespace: "default",
		Labels: constant.GetCompLabels("cluster", "component"),
	}
	current.Labels[constant.CMConfigurationTemplateNameLabelKey] = configSpec.Template
	current.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigInstanceType
	current.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpec.Name
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{Name: "component-parameters", Namespace: "default"},
		Spec: parametersv1alpha1.ComponentParameterSpec{
			ClusterName: "cluster", ComponentName: "component",
			Initial:           &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{"@hosts": ptr.To(`["initial"]`)}},
			ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Name: configSpec.Name, ConfigSpec: &configSpec}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(compDef, paramsDef, current, compParam).Build()
	fetchTask := &Task{}
	fetchTask.ComponentDefObj = compDef
	projectAndApply := func(wantMode string) *corev1.ConfigMap {
		t.Helper()
		changed, err := reconcileParameterValuesIntoSpec(ctx, cli, compParam, fetchTask)
		if err != nil || !changed {
			t.Fatalf("reconcile parameter values: changed=%v, error=%v", changed, err)
		}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(compParam), compParam); err != nil {
			t.Fatal(err)
		}
		descs, defs, err := parampkg.ResolveCmpdParametersDefs(ctx, cli, compDef)
		if err != nil {
			t.Fatal(err)
		}
		configmaps, err := resolveComponentRefConfigMap(ctx, cli, compParam.Namespace, compParam.Spec.ClusterName, compParam.Spec.ComponentName)
		if err != nil {
			t.Fatal(err)
		}
		initial, err := classifyManagedParameterInputs(compParam.Spec.Initial, compDef, configmaps, descs, defs)
		if err != nil {
			t.Fatal(err)
		}
		item := parampkg.GetConfigTemplateItem(&compParam.Spec, configSpec.Name)
		applied, err := parampkg.ApplyParameters(*item, baseConfig, descs, defs, parampkg.DerefMapValues(initial[configSpec.Name]))
		if err != nil {
			t.Fatalf("apply projected parameters: %v", err)
		}
		var got struct {
			Hosts []string `json:"hosts"`
			Mode  string   `json:"mode"`
		}
		if err := json.Unmarshal([]byte(applied.Data["config.json"]), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Hosts) != 1 || got.Hosts[0] != "initial" || got.Mode != wantMode {
			t.Fatalf("expected initial hosts and mode %q, got %s", wantMode, applied.Data["config.json"])
		}
		return applied
	}

	applied := projectAndApply("default")
	current.Data = applied.Data
	if err := cli.Update(ctx, current); err != nil {
		t.Fatal(err)
	}
	compParam.Spec.Desired = &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{"mode": ptr.To("updated")}}
	if err := cli.Update(ctx, compParam); err != nil {
		t.Fatal(err)
	}
	projectAndApply("updated")
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
