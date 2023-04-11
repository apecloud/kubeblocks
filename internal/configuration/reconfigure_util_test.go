/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configuration

import (
	"testing"

	"github.com/StudioSol/set"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

func TestGetUpdateParameterList(t *testing.T) {
	testData := `{
	"a": "b",
	"f": 10.2,
	"c": [
		"edcl",
		"cde"
	],
	"d" : [],
	"n" : [{}],
	"xxx" : [
		{
			"test1": 2,
			"test2": 5
		}
	],
	"g": {
		"cd" : "abcd",
		"msld" :  {
			"cakl": 100,
			"dg": "abcd"
		}
	}}
`
	expected := set.NewLinkedHashSetString("a", "f", "c", "xxx.test1", "xxx.test2", "g.msld.cakl", "g.msld.dg", "g.cd")
	params, err := getUpdateParameterList(newCfgDiffMeta(testData, nil, nil), "")
	require.Nil(t, err)
	require.True(t, util.EqSet(expected,
		set.NewLinkedHashSetString(params...)), "param: %v, expected: %v", params, expected.AsSlice())
}

func newCfgDiffMeta(testData string, add, delete map[string]interface{}) *ConfigPatchInfo {
	return &ConfigPatchInfo{
		UpdateConfig: map[string][]byte{
			"test": []byte(testData),
		},
		AddConfig:    add,
		DeleteConfig: delete,
	}
}

func TestIsUpdateDynamicParameters(t *testing.T) {
	type args struct {
		ccSpec *appsv1alpha1.ConfigConstraintSpec
		diff   *ConfigPatchInfo
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{{
		name: "test",
		// null
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{},
			diff:   newCfgDiffMeta(`null`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// error
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{},
			diff:   newCfgDiffMeta(`invalid json formatter`, nil, nil),
		},
		want:    false,
		wantErr: true,
	}, {
		name: "test",
		// add/delete config file
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{},
			diff:   newCfgDiffMeta(`{}`, map[string]interface{}{"a": "b"}, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// not set static or dynamic parameters
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{},
			diff:   newCfgDiffMeta(`{"a":"b"}`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// static parameters contains
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{
				StaticParameters: []string{"param1", "param2", "param3"},
			},
			diff: newCfgDiffMeta(`{"param3":"b"}`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// static parameters not contains
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{
				StaticParameters: []string{"param1", "param2", "param3"},
			},
			diff: newCfgDiffMeta(`{"param4":"b"}`, nil, nil),
		},
		want:    true,
		wantErr: false,
	}, {
		name: "test",
		// dynamic parameters contains
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{
				DynamicParameters: []string{"param1", "param2", "param3"},
			},
			diff: newCfgDiffMeta(`{"param1":"b", "param3": 20}`, nil, nil),
		},
		want:    true,
		wantErr: false,
	}, {
		name: "test",
		// dynamic parameters not contains
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{
				DynamicParameters: []string{"param1", "param2", "param3"},
			},
			diff: newCfgDiffMeta(`{"param1":"b", "param4": 20}`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// dynamic/static parameters not contains
		args: args{
			ccSpec: &appsv1alpha1.ConfigConstraintSpec{
				DynamicParameters: []string{"dparam1", "dparam2", "dparam3"},
				StaticParameters:  []string{"sparam1", "sparam2", "sparam3"},
			},
			diff: newCfgDiffMeta(`{"a":"b"}`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsUpdateDynamicParameters(tt.args.ccSpec, tt.args.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsUpdateDynamicParameters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsUpdateDynamicParameters() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSchedulableConfigResource(t *testing.T) {
	tests := []struct {
		name   string
		object client.Object
		want   bool
	}{{
		name:   "test",
		object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{}},
		want:   false,
	}, {
		name: "test",
		object: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constant.AppNameLabelKey:        "test",
					constant.AppInstanceLabelKey:    "test",
					constant.KBAppComponentLabelKey: "component",
				},
			},
		},
		want: false,
	}, {
		name: "test",
		object: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constant.AppNameLabelKey:                        "test",
					constant.AppInstanceLabelKey:                    "test",
					constant.KBAppComponentLabelKey:                 "component",
					constant.CMConfigurationTemplateNameLabelKey:    "test_config_template",
					constant.CMConfigurationConstraintsNameLabelKey: "test_config_constraint",
					constant.CMConfigurationSpecProviderLabelKey:    "for_test_config",
					constant.CMConfigurationTypeLabelKey:            constant.ConfigInstanceType,
				},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSchedulableConfigResource(tt.object); got != tt.want {
				t.Errorf("IsSchedulableConfigResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsChangedImportTemplate(t *testing.T) {
	type args struct {
		template *appsv1alpha1.ImportConfigTemplate
		cm       *corev1.ConfigMap
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "test",
		args: args{
			template: nil,
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.CMImportedConfigTemplateLabelKey: "test",
					},
				},
			},
		},
		want: true,
	}, {
		name: "test",
		args: args{
			template: &appsv1alpha1.ImportConfigTemplate{
				Namespace:   "default",
				Name:        "test",
				TemplateRef: "for_test",
			},
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.CMImportedConfigTemplateLabelKey: "default/for_test",
					},
				},
			},
		},
		want: false,
	}, {
		name: "test",
		args: args{
			template: &appsv1alpha1.ImportConfigTemplate{
				Namespace:   "default",
				Name:        "test",
				TemplateRef: "for_test",
			},
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.CMImportedConfigTemplateLabelKey: "default/test2",
					},
				},
			},
		},
		want: true,
	}, {
		name: "test",
		args: args{
			template: nil,
			cm:       &corev1.ConfigMap{},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChangedImportTemplate(tt.args.template, tt.args.cm); got != tt.want {
				t.Errorf("IsChangedImportTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
