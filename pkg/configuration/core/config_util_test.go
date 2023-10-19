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

package core

import (
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

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

	Context("common funcs test", func() {
		It("GetReloadOptions Should success without error", func() {
			mockTpl := v1alpha1.ConfigConstraint{
				Spec: v1alpha1.ConfigConstraintSpec{
					ReloadOptions: &v1alpha1.ReloadOptions{
						UnixSignalTrigger: &v1alpha1.UnixSignalTrigger{
							Signal:      "HUB",
							ProcessName: "for_test",
						},
					},
				},
			}
			tests := []struct {
				name    string
				tpls    []v1alpha1.ComponentConfigSpec
				want    *v1alpha1.ReloadOptions
				wantErr bool
			}{{
				// empty config templates
				name:    "test",
				tpls:    nil,
				want:    nil,
				wantErr: false,
			}, {
				// empty config templates
				name:    "test",
				tpls:    []v1alpha1.ComponentConfigSpec{},
				want:    nil,
				wantErr: false,
			}, {
				// config templates without configConstraintObj
				name: "test",
				tpls: []v1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: v1alpha1.ComponentTemplateSpec{
						Name: "for_test",
					},
				}, {
					ComponentTemplateSpec: v1alpha1.ComponentTemplateSpec{
						Name: "for_test2",
					},
				}},
				want:    nil,
				wantErr: false,
			}, {
				// normal
				name: "test",
				tpls: []v1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: v1alpha1.ComponentTemplateSpec{
						Name: "for_test",
					},
					ConfigConstraintRef: "eg_v1",
				}},
				want:    mockTpl.Spec.ReloadOptions,
				wantErr: false,
			}, {
				// not exist config constraint
				name: "test",
				tpls: []v1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: v1alpha1.ComponentTemplateSpec{
						Name: "for_test",
					},
					ConfigConstraintRef: "not_exist",
				}},
				want:    nil,
				wantErr: true,
			}}

			k8sMockClient.MockGetMethod(testutil.WithGetReturned(func(key client.ObjectKey, obj client.Object) error {
				if strings.Contains(key.Name, "not_exist") {
					return MakeError("not exist config!")
				}
				testutil.SetGetReturnedObject(obj, &mockTpl)
				return nil
			}, testutil.WithMaxTimes(len(tests))))

			for _, tt := range tests {
				got, _, err := GetReloadOptions(k8sMockClient.Client(), ctx, tt.tpls)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				Expect(reflect.DeepEqual(got, tt.want)).Should(BeTrue())
			}
		})
	})

})

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
	}, {
		name: "badcase_test",
		args: args{
			baseCfg: []byte(` `),
			updatedParameters: map[string]string{
				"ENABLE_MODULES":     "true",
				"HUGGINGFACE_APIKEY": "kssdlsdjskwssl",
			},
			formatConfig: &v1alpha1.FormatterConfig{
				Format: v1alpha1.Dotenv,
			},
		},
		// fix begin
		// ENABLE_MODULES=0x1400004f130
		// HUGGINGFACE_APIKEY=0x1400004f140
		want:    "ENABLE_MODULES=true\nHUGGINGFACE_APIKEY=kssdlsdjskwssl\n",
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyConfigPatch(tt.args.baseCfg, FromStringPointerMap(tt.args.updatedParameters), tt.args.formatConfig)
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

func TestFromValueToString(t *testing.T) {
	type args struct {
		val interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "test",
		args: args{
			val: "testTest",
		},
		want: "testTest",
	}, {
		name: "test",
		args: args{
			val: "",
		},
		want: "",
	}, {
		name: "test",
		args: args{
			val: nil,
		},
		want: "",
	}, {
		name: "test",
		args: args{
			val: "/abdet/sds",
		},
		want: "",
	}, {
		name: "test",
		args: args{
			val: "abdet/sds-",
		},
		want: "",
	}, {
		name: "test",
		args: args{
			val: "abcdASls-sda_102.382",
		},
		want: "abcdASls-sda_102.382",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromValueToString(tt.args.val); got != tt.want {
				t.Errorf("FromValueToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromStringMap(t *testing.T) {
	type args struct {
		m map[string]*string
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{{
		name: "test",
		args: args{
			m: map[string]*string{
				"abcd":       cfgutil.ToPointer("test"),
				"null_field": nil,
			},
		},
		want: map[string]interface{}{
			"abcd":       "test",
			"null_field": nil,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromStringMap(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromStringMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
