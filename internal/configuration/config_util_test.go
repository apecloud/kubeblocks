/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package configuration

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/StudioSol/set"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("config_util", func() {

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("MergeAndValidateConfigs", func() {
		It("Should succeed with no error", func() {
			type args struct {
				configConstraint v1alpha1.ConfigConstraintSpec
				baseCfg          map[string]string
				updatedParams    []ParamPairs
				cmKeys           []string
			}

			configConstraintObj := testapps.NewCustomizedObj("resources/mysql-config-constraint.yaml",
				&v1alpha1.ConfigConstraint{}, func(cc *v1alpha1.ConfigConstraint) {
					if ccContext, err := testdata.GetTestDataFileContent("cue_testdata/pg14.cue"); err == nil {
						cc.Spec.ConfigurationSchema = &v1alpha1.CustomParametersValidation{
							CUE: string(ccContext),
						}
					}
					cc.Spec.FormatterConfig = &v1alpha1.FormatterConfig{
						Format: v1alpha1.Properties,
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
					updatedParams: []ParamPairs{
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
					updatedParams: []ParamPairs{
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

				option := CfgOption{
					Type:    CfgTplType,
					CfgType: tt.args.configConstraint.FormatterConfig.Format,
				}

				patch, err := CreateMergePatch(&ConfigResource{
					ConfigData: tt.args.baseCfg,
				}, &ConfigResource{
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
			sets: set.NewLinkedHashSetString(),
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

func TestMergeUpdatedConfig(t *testing.T) {
	type args struct {
		baseMap    map[string]string
		updatedMap map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{{
		name: "normal_test",
		args: args{
			baseMap: map[string]string{
				"key1": "context1",
				"key2": "context2",
				"key3": "context3",
			},
			updatedMap: map[string]string{
				"key2": "new context",
			},
		},
		want: map[string]string{
			"key1": "context1",
			"key2": "new context",
			"key3": "context3",
		},
	}, {
		name: "not_expected_update_test",
		args: args{
			baseMap: map[string]string{
				"key1": "context1",
				"key2": "context2",
				"key3": "context3",
			},
			updatedMap: map[string]string{
				"key6": "context6",
			},
		},
		want: map[string]string{
			"key1": "context1",
			"key2": "context2",
			"key3": "context3",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeUpdatedConfig(tt.args.baseMap, tt.args.updatedMap); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeUpdatedConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyConfigPatch(t *testing.T) {
	type args struct {
		baseCfg           []byte
		updatedParameters map[string]string
		formatConfig      *v1alpha1.FormatterConfig
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			baseCfg: []byte(`[test]
test=test`),
			updatedParameters: map[string]string{
				"a":               "b",
				"max_connections": "600",
			},
			formatConfig: &v1alpha1.FormatterConfig{
				Format: v1alpha1.Ini,
				FormatterOptions: v1alpha1.FormatterOptions{
					IniConfig: &v1alpha1.IniConfig{
						SectionName: "test",
					}}},
		},
		want: `[test]
a=b
max_connections=600
test=test
`,
		wantErr: false,
	}, {
		name: "normal_test",
		args: args{
			baseCfg: []byte(` `),
			updatedParameters: map[string]string{
				"a": "b",
				"c": "d e f g",
			},
			formatConfig: &v1alpha1.FormatterConfig{
				Format: v1alpha1.RedisCfg,
			},
		},
		want:    "a b\nc d e f g",
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyConfigPatch(tt.args.baseCfg, tt.args.updatedParameters, tt.args.formatConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyConfigPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ApplyConfigPatch() got = %v, want %v", got, tt.want)
			}
		})
	}
}
