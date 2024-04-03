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

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
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
		item v1alpha1.ConfigurationItemDetail
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{

		name: "test",
		args: args{
			cm: nil,
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
			},
		},
		want: true,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
			},
		},
		want: false,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name:    "test",
				Version: "v1",
			},
		},
		want: true,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.CMConfigurationTemplateVersion, "v1").
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name:    "test",
				Version: "v2",
			},
		},
		want: true,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.CMConfigurationTemplateVersion, "v1").
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name:    "test",
				Version: "v1",
			},
		},
		want: false,
	}, {
		name: "import-template-test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, "").
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
				ImportTemplateRef: &v1alpha1.ConfigTemplateExtension{
					TemplateRef: "contig-test-template",
					Namespace:   "default",
					Policy:      v1alpha1.PatchPolicy,
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
  "importTemplateRef": {
    "templateRef": "contig-test-template",
    "namespace": "default",
    "policy": "patch"
  }
}
`).
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
				ImportTemplateRef: &v1alpha1.ConfigTemplateExtension{
					TemplateRef: "contig-test-template",
					Namespace:   "default",
					Policy:      v1alpha1.PatchPolicy,
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
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
				Payload: v1alpha1.Payload{
					Data: map[string]any{
						"key": "value",
					},
				},
			},
		},
		want: true,
	}, {
		name: "payload-test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, ` {"payload":{"key":"value"}} `).
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
				Payload: v1alpha1.Payload{
					Data: map[string]any{
						"key": "value",
					},
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
		item   v1alpha1.ConfigurationItemDetail
		status *v1alpha1.ConfigurationItemDetailStatus
	}
	tests := []struct {
		name string
		args args
		want v1alpha1.ConfigurationPhase
	}{{
		name: "test",
		args: args{
			cm: nil,
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
			},
		},
		want: v1alpha1.CCreatingPhase,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
			},
			status: &v1alpha1.ConfigurationItemDetailStatus{
				Phase: v1alpha1.CInitPhase,
			},
		},
		want: v1alpha1.CPendingPhase,
	}, {
		name: "test",
		args: args{
			cm: builder.NewConfigMapBuilder("default", "test").
				AddAnnotations(constant.ConfigAppliedVersionAnnotationKey, `{"name":"test"}`).
				GetObject(),
			item: v1alpha1.ConfigurationItemDetail{
				Name: "test",
			},
			status: &v1alpha1.ConfigurationItemDetailStatus{
				Phase: v1alpha1.CUpgradingPhase,
			},
		},
		want: v1alpha1.CUpgradingPhase,
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
				configConstraint appsv1beta1.ConfigConstraintSpec
				baseCfg          map[string]string
				updatedParams    []core.ParamPairs
				cmKeys           []string
			}

			configConstraintObj := testapps.NewCustomizedObj("resources/mysql-config-constraint.yaml",
				&appsv1beta1.ConfigConstraint{}, func(cc *appsv1beta1.ConfigConstraint) {
					if ccContext, err := testdata.GetTestDataFileContent("cue_testdata/pg14.cue"); err == nil {
						cc.Spec.ConfigSchema = &appsv1beta1.ConfigSchema{
							CUE: string(ccContext),
						}
					}
					cc.Spec.FormatterConfig = &appsv1beta1.FormatterConfig{
						Format: appsv1beta1.Properties,
					}
				})

			cfgContext, err := testdata.GetTestDataFileContent("cue_testdata/pg14.conf")
			Expect(err).Should(Succeed())

			tests := []struct {
				name    string
				args    args
				want    map[string]string
				wantErr bool
			}{{
				name: "pg1_merge",
				args: args{
					configConstraint: configConstraintObj.Spec,
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
					cmKeys: []string{"key", "key3"},
				},
				want: map[string]string{
					"max_connections": "200",
					"shared_buffers":  "512M",
				},
			}, {
				name: "not_support_key_updated",
				args: args{
					configConstraint: configConstraintObj.Spec,
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
					cmKeys: []string{"key1", "key2"},
				},
				wantErr: true,
			}}
			for _, tt := range tests {
				got, err := MergeAndValidateConfigs(tt.args.configConstraint, tt.args.baseCfg, tt.args.cmKeys, tt.args.updatedParams)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				if tt.wantErr {
					continue
				}

				option := core.CfgOption{
					Type:    core.CfgTplType,
					CfgType: tt.args.configConstraint.FormatterConfig.Format,
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
		item      *v1alpha1.ConfigurationItemDetail
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
			item:      &v1alpha1.ConfigurationItemDetail{},
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
			item: &v1alpha1.ConfigurationItemDetail{
				Payload: v1alpha1.Payload{
					Data: map[string]any{
						constant.BinaryVersionPayload: "md5-12912uy1232o9y2",
					},
				},
			},
			payloadID: constant.BinaryVersionPayload,
			payload:   nil,
		},
		want: true,
	}, {
		name: "test-update-payload",
		args: args{
			item: &v1alpha1.ConfigurationItemDetail{
				Payload: v1alpha1.Payload{
					Data: map[string]any{
						constant.BinaryVersionPayload: "md5-12912uy1232o9y2",
						constant.ComponentResourcePayload: map[string]any{
							"limit": map[string]string{
								"cpu":    "100m",
								"memory": "100Mi",
							},
						},
					},
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
