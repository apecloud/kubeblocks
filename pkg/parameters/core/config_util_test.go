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

package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

// import (
// 	"reflect"
// 	"strings"
// 	"testing"
//
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
//
// 	"sigs.k8s.io/controller-runtime/pkg/client"
//
// 	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
// 	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
// 	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
// 	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
// )
//
// var _ = Describe("config_util", func() {
//
// 	var k8sMockClient *testutil.K8sClientMockHelper
//
// 	BeforeEach(func() {
// 		// Add any setup steps that needs to be executed before each test
// 		k8sMockClient = testutil.NewK8sMockClient()
// 	})
//
// 	AfterEach(func() {
// 		// Add any teardown steps that needs to be executed after each test
// 		k8sMockClient.Finish()
// 	})
//
// 	Context("common funcs test", func() {
// 		It("GetReloadOptions Should success without error", func() {
// 			mockTpl := appsv1beta1.ConfigConstraint{
// 				Spec: appsv1beta1.ConfigConstraintSpec{
// 					ReloadAction: &appsv1beta1.ReloadAction{
// 						UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 							Signal:      "HUB",
// 							ProcessName: "for_test",
// 						},
// 					},
// 				},
// 			}
// 			tests := []struct {
// 				name    string
// 				tpls    []appsv1.ComponentConfigSpec
// 				want    *appsv1beta1.ReloadAction
// 				wantErr bool
// 			}{{
// 				// empty config templates
// 				name:    "test",
// 				tpls:    nil,
// 				want:    nil,
// 				wantErr: false,
// 			}, {
// 				// empty config templates
// 				name:    "test",
// 				tpls:    []appsv1.ComponentConfigSpec{},
// 				want:    nil,
// 				wantErr: false,
// 			}, {
// 				// config templates without configConstraintObj
// 				name: "test",
// 				tpls: []appsv1.ComponentConfigSpec{{
// 					ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
// 						Name: "for_test",
// 					},
// 				}, {
// 					ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
// 						Name: "for_test2",
// 					},
// 				}},
// 				want:    nil,
// 				wantErr: false,
// 			}, {
// 				// normal
// 				name: "test",
// 				tpls: []appsv1.ComponentConfigSpec{{
// 					ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
// 						Name: "for_test",
// 					},
// 					ConfigConstraintRef: "eg_v1",
// 				}},
// 				want:    mockTpl.Spec.ReloadAction,
// 				wantErr: false,
// 			}, {
// 				// not exist config constraint
// 				name: "test",
// 				tpls: []appsv1.ComponentConfigSpec{{
// 					ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
// 						Name: "for_test",
// 					},
// 					ConfigConstraintRef: "not_exist",
// 				}},
// 				want:    nil,
// 				wantErr: true,
// 			}}
//
// 			k8sMockClient.MockGetMethod(testutil.WithGetReturned(func(key client.ObjectKey, obj client.Object) error {
// 				if strings.Contains(key.Name, "not_exist") {
// 					return MakeError("not exist config!")
// 				}
// 				testutil.SetGetReturnedObject(obj, &mockTpl)
// 				return nil
// 			}, testutil.WithMaxTimes(len(tests))))
//
// 			for _, tt := range tests {
// 				got, _, err := GetReloadOptions(k8sMockClient.Client(), ctx, tt.tpls)
// 				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
// 				Expect(reflect.DeepEqual(got, tt.want)).Should(BeTrue())
// 			}
// 		})
// 	})
//
// })
//
// func TestMergeUpdatedConfig(t *testing.T) {
// 	type args struct {
// 		baseMap    map[string]string
// 		updatedMap map[string]string
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want map[string]string
// 	}{{
// 		name: "normal_test",
// 		args: args{
// 			baseMap: map[string]string{
// 				"key1": "context1",
// 				"key2": "context2",
// 				"key3": "context3",
// 			},
// 			updatedMap: map[string]string{
// 				"key2": "new context",
// 			},
// 		},
// 		want: map[string]string{
// 			"key1": "context1",
// 			"key2": "new context",
// 			"key3": "context3",
// 		},
// 	}, {
// 		name: "not_expected_update_test",
// 		args: args{
// 			baseMap: map[string]string{
// 				"key1": "context1",
// 				"key2": "context2",
// 				"key3": "context3",
// 			},
// 			updatedMap: map[string]string{
// 				"key6": "context6",
// 			},
// 		},
// 		want: map[string]string{
// 			"key1": "context1",
// 			"key2": "context2",
// 			"key3": "context3",
// 		},
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := MergeUpdatedConfig(tt.args.baseMap, tt.args.updatedMap); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("MergeUpdatedConfig() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

func TestMergeUpdatedConfig(t *testing.T) {
	base := map[string]string{
		"mysql.cnf": "[mysqld]",
		"proxy.cnf": "port=3306",
	}
	updated := map[string]string{
		"mysql.cnf": "[mysqld]\nmax_connections=200",
		"extra.cnf": "ignored=true",
	}

	got := MergeUpdatedConfig(base, updated)
	require.Equal(t, map[string]string{
		"mysql.cnf": "[mysqld]\nmax_connections=200",
		"proxy.cnf": "port=3306",
	}, got)
}

func TestFromStringMap(t *testing.T) {
	port := "3306"
	hosts := `["mysql-0","mysql-1"]`
	params := map[string]*string{
		"port":   &port,
		"unset":  nil,
		"@hosts": &hosts,
	}

	got, err := FromStringMap(params, ValueTransformerFunc(func(value string, fieldName string) (any, error) {
		if fieldName == "port" {
			return "tcp:" + value, nil
		}
		return value, nil
	}))
	require.NoError(t, err)
	require.Equal(t, "tcp:3306", got["port"])
	require.Nil(t, got["unset"])
	require.Equal(t, []interface{}{"mysql-0", "mysql-1"}, got["hosts"])

	_, err = FromStringMap(map[string]*string{"port": &port}, ValueTransformerFunc(func(string, string) (any, error) {
		return nil, errors.New("bad value")
	}))
	require.ErrorContains(t, err, "bad value")
}

func TestApplyConfigPatch(t *testing.T) {
	base := []byte("[mysqld]\nmax_connections=100\nold_value=keep\n")
	format := &parametersv1alpha1.FileFormatConfig{
		Format: parametersv1alpha1.Ini,
		FormatterAction: parametersv1alpha1.FormatterAction{
			IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
		},
	}

	got, err := ApplyConfigPatch(base, map[string]*string{
		"max_connections": ptr.To("200"),
		"old_value":       nil,
	}, format, nil)
	require.NoError(t, err)
	require.Contains(t, got, "[mysqld]")
	require.Contains(t, got, "max_connections=200")

	_, err = ApplyConfigPatch([]byte(":"), map[string]*string{"max_connections": ptr.To("200")}, format, nil)
	require.Error(t, err)
}

func TestIsWatchModuleForShellTrigger(t *testing.T) {
	sync := true
	async := false

	tests := []struct {
		name    string
		trigger *parametersv1alpha1.ShellTrigger
		want    bool
	}{
		{name: "nil trigger", want: true},
		{name: "nil sync", trigger: &parametersv1alpha1.ShellTrigger{}, want: true},
		{name: "sync trigger", trigger: &parametersv1alpha1.ShellTrigger{Sync: &sync}, want: false},
		{name: "async trigger", trigger: &parametersv1alpha1.ShellTrigger{Sync: &async}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsWatchModuleForShellTrigger(tt.trigger))
		})
	}
}

func TestArrayFieldHelpers(t *testing.T) {
	require.True(t, hasArrayField("@hosts"))
	require.False(t, hasArrayField("hosts"))
	require.Equal(t, "@hosts", transArrayFieldName("hosts"))
	require.Equal(t, "hosts", GetValidFieldName("@hosts"))
	require.Equal(t, "hosts", GetValidFieldName("hosts"))
	require.Equal(t, "", transJSONString(nil))
	require.Equal(t, []any{}, fromJSONString(ptr.To("")))
	require.True(t, reflect.DeepEqual([]any{"a"}, fromJSONString(ptr.To(`["a"]`))))
	require.True(t, strings.HasPrefix(transJSONString([]string{"a"}), "["))
}

//
// func TestApplyConfigPatch(t *testing.T) {
// 	type args struct {
// 		baseCfg           []byte
// 		updatedParameters map[string]string
// 		formatConfig      *appsv1beta1.FileFormatConfig
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		want    string
// 		wantErr bool
// 	}{{
// 		name: "normal_test",
// 		args: args{
// 			baseCfg: []byte(`[test]
// test=test`),
// 			updatedParameters: map[string]string{
// 				"a":               "b",
// 				"max_connections": "600",
// 			},
// 			formatConfig: &appsv1beta1.FileFormatConfig{
// 				Format: appsv1beta1.Ini,
// 				FormatterAction: appsv1beta1.FormatterAction{
// 					IniConfig: &appsv1beta1.IniConfig{
// 						SectionName: "test",
// 					}}},
// 		},
// 		want: `[test]
// a=b
// max_connections=600
// test=test
// `,
// 		wantErr: false,
// 	}, {
// 		name: "normal_test",
// 		args: args{
// 			baseCfg: []byte(` `),
// 			updatedParameters: map[string]string{
// 				"a": "b",
// 				"c": "d e f g",
// 			},
// 			formatConfig: &appsv1beta1.FileFormatConfig{
// 				Format: appsv1beta1.RedisCfg,
// 			},
// 		},
// 		want:    "a b\nc d e f g",
// 		wantErr: false,
// 	}, {
// 		name: "badcase_test",
// 		args: args{
// 			baseCfg: []byte(` `),
// 			updatedParameters: map[string]string{
// 				"ENABLE_MODULES":     "true",
// 				"HUGGINGFACE_APIKEY": "kssdlsdjskwssl",
// 			},
// 			formatConfig: &appsv1beta1.FileFormatConfig{
// 				Format: appsv1beta1.Dotenv,
// 			},
// 		},
// 		// fix begin
// 		// ENABLE_MODULES=0x1400004f130
// 		// HUGGINGFACE_APIKEY=0x1400004f140
// 		want:    "ENABLE_MODULES=true\nHUGGINGFACE_APIKEY=kssdlsdjskwssl\n",
// 		wantErr: false,
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := ApplyConfigPatch(tt.args.baseCfg, FromStringPointerMap(tt.args.updatedParameters), tt.args.formatConfig)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("ApplyConfigPatch() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if got != tt.want {
// 				t.Errorf("ApplyConfigPatch() got = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
//
// func TestFromValueToString(t *testing.T) {
// 	type args struct {
// 		val interface{}
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want string
// 	}{{
// 		name: "test",
// 		args: args{
// 			val: "testTest",
// 		},
// 		want: "testTest",
// 	}, {
// 		name: "test",
// 		args: args{
// 			val: "",
// 		},
// 		want: "",
// 	}, {
// 		name: "test",
// 		args: args{
// 			val: nil,
// 		},
// 		want: "",
// 	}, {
// 		name: "test",
// 		args: args{
// 			val: "/abdet/sds",
// 		},
// 		want: "",
// 	}, {
// 		name: "test",
// 		args: args{
// 			val: "abdet/sds-",
// 		},
// 		want: "",
// 	}, {
// 		name: "test",
// 		args: args{
// 			val: "abcdASls-sda_102.382",
// 		},
// 		want: "abcdASls-sda_102.382",
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := FromValueToString(tt.args.val); got != tt.want {
// 				t.Errorf("FromValueToString() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
//
// func TestFromStringMap(t *testing.T) {
// 	type args struct {
// 		m map[string]*string
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want map[string]interface{}
// 	}{{
// 		name: "test",
// 		args: args{
// 			m: map[string]*string{
// 				"abcd":       cfgutil.ToPointer("test"),
// 				"null_field": nil,
// 			},
// 		},
// 		want: map[string]interface{}{
// 			"abcd":       "test",
// 			"null_field": nil,
// 		},
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := FromStringMap(tt.args.m); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("FromStringMap() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
