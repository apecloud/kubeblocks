/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package controllerutil

import (
	"encoding/json"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
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

func TestIsRerender(t *testing.T) {
	type args struct {
		cm   *corev1.ConfigMap
		item parametersv1alpha1.ConfigTemplateItemDetail
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{

		name: "test",
		args: args{
			cm: nil,
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
			},
		},
		want: true,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
			},
		},
		want: false,
	}, {
		name: "import-template-test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, "").
				GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
				CustomTemplates: &appsv1.ConfigTemplateExtension{
					TemplateRef: "contig-test-template",
					Namespace:   "default",
					Policy:      appsv1.PatchPolicy,
				},
			},
		},
		want: true,
	}, {
		name: "import-template-test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, `
{
  "userConfigTemplates": {
    "templateRef": "contig-test-template",
    "namespace": "default",
    "policy": "patch"
  }
}
`).
				GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
				CustomTemplates: &appsv1.ConfigTemplateExtension{
					TemplateRef: "contig-test-template",
					Namespace:   "default",
					Policy:      appsv1.PatchPolicy,
				},
			},
		},
		want: false,
	}, {
		name: "payload-test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, "").
				GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name:    "test",
				Payload: parametersv1alpha1.Payload{},
			},
		},
		want: false,
	}, {
		name: "payload-test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, ` {"payload":{"key":"value"}} `).
				GetObject(),
			item: parametersv1alpha1.ConfigTemplateItemDetail{
				Name: "test",
				Payload: parametersv1alpha1.Payload{
					"key": transformPayload("value"),
				},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRerender(tt.args.cm, tt.args.item); got != tt.want {
				t.Errorf("IsRerender() = %v, want %v", got, tt.want)
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
			if got := GetConfigSpecReconcilePhase(tt.args.cm, tt.args.item, tt.args.status); got != tt.want {
				t.Errorf("GetConfigSpecReconcilePhase() = %v, want %v", got, tt.want)
			}
		})
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

func TestCheckAndPatchPayload(t *testing.T) {
	type args struct {
		item      *parametersv1alpha1.ConfigTemplateItemDetail
		payloadID string
		payload   interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{{
		name: "test",
		args: args{
			item:      &parametersv1alpha1.ConfigTemplateItemDetail{},
			payloadID: constant.BinaryVersionPayload,
			payload:   "md5-12912uy1232o9y2",
		},
		want: true,
	}, {
		name: "invalid-item-test",
		args: args{
			payloadID: constant.BinaryVersionPayload,
			payload:   "md5-12912uy1232o9y2",
		},
		want: false,
	}, {
		name: "test-delete-payload",
		args: args{
			item: &parametersv1alpha1.ConfigTemplateItemDetail{
				Payload: parametersv1alpha1.Payload{
					constant.BinaryVersionPayload: json.RawMessage("md5-12912uy1232o9y2"),
				},
			},
			payloadID: constant.BinaryVersionPayload,
			payload:   nil,
		},
		want: true,
	}, {
		name: "test-update-payload",
		args: args{
			item: &parametersv1alpha1.ConfigTemplateItemDetail{
				Payload: parametersv1alpha1.Payload{
					constant.BinaryVersionPayload: json.RawMessage("md5-12912uy1232o9y2"),
					constant.ComponentResourcePayload: transformPayload(map[string]any{
						"limit": map[string]string{
							"cpu":    "100m",
							"memory": "100Mi",
						},
					}),
				},
			},
			payloadID: constant.ComponentResourcePayload,
			payload: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200m"),
				},
			},
		},
		want: true,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckAndPatchPayload(tt.args.item, tt.args.payloadID, tt.args.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckAndPatchPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CheckAndPatchPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func transformPayload(data interface{}) json.RawMessage {
	raw, _ := buildPayloadAsUnstructuredObject(data)
	return raw
}
