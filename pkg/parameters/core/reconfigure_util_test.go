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

package core

import (
	"testing"

	"github.com/StudioSol/set"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/util"
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

	// for trim
	expected = set.NewLinkedHashSetString("msld.cakl", "msld.dg", "cd")
	params, err = getUpdateParameterList(newCfgDiffMeta(testData, nil, nil), "g")
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

func TestIsDynamicParameter(t *testing.T) {
	type args struct {
		paramName string
		ccSpec    *parametersv1alpha1.ParametersDefinitionSpec
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "test",
		// null
		args: args{
			paramName: "test",
			ccSpec:    &parametersv1alpha1.ParametersDefinitionSpec{},
		},
		want: false,
	}, {
		name: "test",
		// static parameters contains
		args: args{
			ccSpec: &parametersv1alpha1.ParametersDefinitionSpec{
				StaticParameters: []string{"param1", "param2", "param3"},
			},
			paramName: "param3",
		},
		want: false,
	}, {
		name: "test",
		// static parameters not contains
		args: args{
			paramName: "param4",
			ccSpec: &parametersv1alpha1.ParametersDefinitionSpec{
				StaticParameters: []string{"param1", "param2", "param3"},
			},
		},
		want: true,
	}, {
		name: "test",
		// dynamic parameters contains
		args: args{
			paramName: "param1",
			ccSpec: &parametersv1alpha1.ParametersDefinitionSpec{
				DynamicParameters: []string{"param1", "param2", "param3"},
			},
		},
		want: true,
	}, {
		name: "test",
		// dynamic/static parameters not contains
		args: args{
			paramName: "test",
			ccSpec: &parametersv1alpha1.ParametersDefinitionSpec{
				DynamicParameters: []string{"dparam1", "dparam2", "dparam3"},
				StaticParameters:  []string{"sparam1", "sparam2", "sparam3"},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDynamicParameter(tt.args.paramName, tt.args.ccSpec); got != tt.want {
				t.Errorf("IsDynamicParameter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasDynamicParameterUpdate(t *testing.T) {
	config := &parametersv1alpha1.FileFormatConfig{
		Format: parametersv1alpha1.Ini,
		FormatterAction: parametersv1alpha1.FormatterAction{
			IniConfig: &parametersv1alpha1.IniConfig{
				SectionName: "mysqld",
			},
		},
	}
	pd := &parametersv1alpha1.ParametersDefinitionSpec{
		DynamicParameters: []string{"binlog_expire_logs_seconds"},
	}

	tests := []struct {
		name  string
		patch string
		want  bool
	}{
		{
			name:  "static only",
			patch: `{"mysqld":{"table_open_cache_instances":"8"}}`,
			want:  false,
		},
		{
			name:  "dynamic only",
			patch: `{"mysqld":{"binlog_expire_logs_seconds":"432000"}}`,
			want:  true,
		},
		{
			name:  "mixed",
			patch: `{"mysqld":{"binlog_expire_logs_seconds":"432000","table_open_cache_instances":"8"}}`,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasDynamicParameterUpdate(config, pd, tt.patch)
			if err != nil {
				t.Fatalf("HasDynamicParameterUpdate() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("HasDynamicParameterUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}
