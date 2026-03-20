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
	"encoding/json"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestFromUpdatedConfig(t *testing.T) {
	type args struct {
		base map[string]string
		sets *set.LinkedHashSetString
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{{
		name: "normal_test",
		args: args{
			base: map[string]string{
				"key1": "config context1",
				"key2": "config context2",
				"key3": "config context2",
			},
			sets: set.NewLinkedHashSetString("key1", "key3"),
		},
		want: map[string]string{
			"key1": "config context1",
			"key3": "config context2",
		},
	}, {
		name: "none_updated_test",
		args: args{
			base: map[string]string{
				"key1": "config context1",
				"key2": "config context2",
				"key3": "config context2",
			},
			sets: cfgutil.NewSet(),
		},
		want: map[string]string{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromUpdatedConfig(tt.args.base, tt.args.sets); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fromUpdatedConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetConfigSpecReconcilePhase(t *testing.T) {
	type args struct {
		cm     *corev1.ConfigMap
		item   parametersv1alpha1.ConfigTemplateItemDetail
		status *parametersv1alpha1.ConfigTemplateItemDetailStatus
	}
	tests := []struct {
		name string
		args args
		want parametersv1alpha1.ParameterPhase
	}{{
		name: "test",
		args: args{
			cm: nil,
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
			},
		},
		want: parametersv1alpha1.CCreatingPhase,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
			},
			status: &parametersv1alpha1.ConfigTemplateItemDetailStatus{
				Phase: parametersv1alpha1.CInitPhase,
			},
		},
		want: parametersv1alpha1.CPendingPhase,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, `{"name":"test"}`).
				GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
			},
			status: &parametersv1alpha1.ConfigTemplateItemDetailStatus{
				Phase: parametersv1alpha1.CUpgradingPhase,
			},
		},
		want: parametersv1alpha1.CUpgradingPhase,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUpdatedParametersReconciledPhase(tt.args.cm, tt.args.item, tt.args.status, 0); got != tt.want {
				t.Errorf("GetUpdatedParametersReconciledPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyConfigManagerRequiredForParamsDefs(t *testing.T) {
	newParamsDef := func(reloadAction *parametersv1alpha1.ReloadAction) *parametersv1alpha1.ParametersDefinition {
		pd := &parametersv1alpha1.ParametersDefinition{}
		pd.Spec.ReloadAction = reloadAction
		return pd
	}

	tests := []struct {
		name       string
		paramsDefs []*parametersv1alpha1.ParametersDefinition
		want       bool
	}{
		{
			name: "no legacy actions",
			paramsDefs: []*parametersv1alpha1.ParametersDefinition{
				newParamsDef(nil),
			},
			want: false,
		},
		{
			name: "auto reload action requires legacy config manager",
			paramsDefs: []*parametersv1alpha1.ParametersDefinition{
				newParamsDef(&parametersv1alpha1.ReloadAction{
					AutoTrigger: &parametersv1alpha1.AutoTrigger{ProcessName: "mysqld"},
				}),
			},
			want: true,
		},
		{
			name: "shell reload action requires legacy config manager",
			paramsDefs: []*parametersv1alpha1.ParametersDefinition{
				newParamsDef(&parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"bash", "-c", "reload"}},
				}),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LegacyConfigManagerRequiredForParamsDefs(tt.paramsDefs); got != tt.want {
				t.Fatalf("LegacyConfigManagerRequiredForParamsDefs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyConfigManagerRequiredForCluster(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
		wantErr     bool
	}{
		{
			name: "missing annotation",
			want: false,
		},
		{
			name: "enabled",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "true",
			},
			want: true,
		},
		{
			name: "disabled",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "false",
			},
			want: false,
		},
		{
			name: "invalid value",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &appsv1.Cluster{}
			cluster.Annotations = tt.annotations
			got, err := LegacyConfigManagerRequiredForCluster(cluster)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LegacyConfigManagerRequiredForCluster() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("LegacyConfigManagerRequiredForCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyConfigManagerRequirementStateForCluster(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        LegacyConfigManagerRequirementState
		wantErr     bool
	}{
		{
			name: "missing annotation",
			want: LegacyConfigManagerRequirementUnknown,
		},
		{
			name: "enabled",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "true",
			},
			want: LegacyConfigManagerRequirementKeep,
		},
		{
			name: "disabled",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "false",
			},
			want: LegacyConfigManagerRequirementCleanup,
		},
		{
			name: "empty means unknown",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "",
			},
			want: LegacyConfigManagerRequirementUnknown,
		},
		{
			name: "invalid value",
			annotations: map[string]string{
				constant.LegacyConfigManagerRequiredAnnotationKey: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &appsv1.Cluster{}
			cluster.Annotations = tt.annotations
			got, err := LegacyConfigManagerRequirementStateForCluster(cluster)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LegacyConfigManagerRequirementStateForCluster() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("LegacyConfigManagerRequirementStateForCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveCmpdParametersDefs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = parametersv1alpha1.AddToScheme(scheme)

	cmpd := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-8.0.30"},
		Spec: appsv1.ComponentDefinitionSpec{
			ServiceVersion: "8.0.30",
		},
		Status: appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
	}
	pd := &parametersv1alpha1.ParametersDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-params"},
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			ComponentDef: "mysql-8",
			TemplateName: "mysql-config",
			FileName:     "my.cnf",
			FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
			},
		},
		Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmpd, pd).Build()
	configRender, paramsDefs, err := ResolveCmpdParametersDefs(context.Background(), cli, cmpd)
	if err != nil {
		t.Fatalf("ResolveCmpdParametersDefs() error = %v", err)
	}
	if len(paramsDefs) != 1 {
		t.Fatalf("ResolveCmpdParametersDefs() paramsDefs len = %d, want 1", len(paramsDefs))
	}
	if configRender == nil {
		t.Fatalf("ResolveCmpdParametersDefs() configRender = nil")
	}
	if len(configRender.Spec.Configs) != 1 {
		t.Fatalf("ResolveCmpdParametersDefs() configs len = %d, want 1", len(configRender.Spec.Configs))
	}
	if configRender.Spec.Configs[0].TemplateName != "mysql-config" {
		t.Fatalf("ResolveCmpdParametersDefs() templateName = %q, want %q", configRender.Spec.Configs[0].TemplateName, "mysql-config")
	}
}

func TestResolveCmpdParametersDefsRejectsDuplicateFiles(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = parametersv1alpha1.AddToScheme(scheme)

	cmpd := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-8.0.30"},
		Spec: appsv1.ComponentDefinitionSpec{
			ServiceVersion: "8.0.30",
		},
		Status: appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
	}
	newPD := func(name string) *parametersv1alpha1.ParametersDefinition {
		return &parametersv1alpha1.ParametersDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: parametersv1alpha1.ParametersDefinitionSpec{
				ComponentDef: "mysql-8",
				TemplateName: "mysql-config",
				FileName:     "my.cnf",
				FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
					Format: parametersv1alpha1.Ini,
				},
			},
			Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
		}
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmpd, newPD("pd-1"), newPD("pd-2")).Build()
	_, _, err := ResolveCmpdParametersDefs(context.Background(), cli, cmpd)
	if err == nil {
		t.Fatalf("ResolveCmpdParametersDefs() error = nil, want duplicate-file error")
	}
}

var _ = Describe("config_util", func() {

	var k8sMockClient *testutil.K8sClientMockHelper

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sMockClient.Finish()
	})

	Context("MergeAndValidateConfigs", func() {
		It("Should succeed with no error", func() {
			type args struct {
				parametersDef parametersv1alpha1.ParametersDefinition
				baseCfg       map[string]string
				updatedParams []core.ParamPairs
				configs       []parametersv1alpha1.ComponentConfigDescription
			}

			cfgContext, _ := testdata.GetTestDataFileContent("cue_testdata/pg14.conf")
			ccContext, _ := testdata.GetTestDataFileContent("cue_testdata/pg14.cue")
			paramsDef := parametersv1alpha1.ParametersDefinition{
				Spec: parametersv1alpha1.ParametersDefinitionSpec{
					FileName: "key",
					ParametersSchema: &parametersv1alpha1.ParametersSchema{
						CUE: string(ccContext),
					},
				},
			}

			tests := []struct {
				name    string
				args    args
				want    map[string]string
				wantErr bool
			}{{
				name: "pg1_merge",
				args: args{
					parametersDef: paramsDef,
					baseCfg: map[string]string{
						"key":  string(cfgContext),
						"key2": "not support context",
					},
					updatedParams: []core.ParamPairs{
						{
							Key: "key",
							UpdatedParams: map[string]interface{}{
								"max_connections": "200",
								"shared_buffers":  "512M",
							},
						},
					},
					configs: []parametersv1alpha1.ComponentConfigDescription{{Name: "key", FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Properties}}},
				},
				want: map[string]string{
					"max_connections": "200",
					"shared_buffers":  "512M",
				},
			}, {
				name: "not_support_key_updated",
				args: args{
					parametersDef: paramsDef,
					baseCfg: map[string]string{
						"key":  string(cfgContext),
						"key2": "not_support_context",
					},
					updatedParams: []core.ParamPairs{
						{
							Key: "key",
							UpdatedParams: map[string]interface{}{
								"max_connections": "200",
								"shared_buffers":  "512M",
							},
						},
					},
					configs: []parametersv1alpha1.ComponentConfigDescription{{Name: "key2", FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Properties}}},
				},
				wantErr: true,
			}}
			for _, tt := range tests {
				got, err := MergeAndValidateConfigs(tt.args.baseCfg, tt.args.updatedParams, []*parametersv1alpha1.ParametersDefinition{&tt.args.parametersDef}, tt.args.configs)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				if tt.wantErr {
					continue
				}

				option := core.CfgOption{
					Type:         core.CfgTplType,
					FileFormatFn: core.WithConfigFileFormat(tt.args.configs),
				}

				patch, err := core.CreateMergePatch(&core.ConfigResource{
					ConfigData: tt.args.baseCfg,
				}, &core.ConfigResource{
					ConfigData: got,
				}, option)
				Expect(err).Should(Succeed())

				var patchJSON map[string]string
				Expect(json.Unmarshal(patch.UpdateConfig["key"], &patchJSON)).Should(Succeed())
				Expect(patchJSON).Should(BeEquivalentTo(tt.want))
			}
		})
	})

})

func Test_filterImmutableParameters(t *testing.T) {
	type args struct {
		parameters      map[string]any
		immutableParams []string
	}
	tests := []struct {
		name string
		args args
		want map[string]any
	}{{
		name: "test",
		args: args{
			parameters: map[string]any{
				"a": "b",
				"c": "d",
			},
		},
		want: map[string]any{
			"a": "b",
			"c": "d",
		},
	}, {
		name: "test",
		args: args{
			parameters: map[string]any{
				"a": "b",
				"c": "d",
			},
			immutableParams: []string{"a", "d"},
		},
		want: map[string]any{
			"c": "d",
		},
	}, {
		name: "test",
		args: args{
			parameters:      map[string]any{},
			immutableParams: []string{"a", "d"},
		},
		want: map[string]any{},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramsDefs := []*parametersv1alpha1.ParametersDefinition{{
				Spec: parametersv1alpha1.ParametersDefinitionSpec{
					FileName:            "test",
					ImmutableParameters: tt.args.immutableParams,
				},
			}}
			if got := filterImmutableParameters(tt.args.parameters, "test", paramsDefs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterImmutableParameters() = %v, want %v", got, tt.want)
			}
		})
	}
}
