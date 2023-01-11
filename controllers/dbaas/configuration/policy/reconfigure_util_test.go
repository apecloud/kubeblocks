/*
Copyright ApeCloud Inc.

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

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
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
		"msld" : "cakl"
	}}
`
	params, err := getUpdateParameterList(newCfgDiffMeta(testData, nil, nil))
	require.Nil(t, err)
	require.Equal(t, cfgcore.NewSetFromList(
		[]string{
			"a", "c_1", "c_0", "msld", "cd", "f", "test1", "test2",
		}),
		cfgcore.NewSetFromList(params))
}

func newCfgDiffMeta(testData string, add, delete map[string]interface{}) *cfgcore.ConfigDiffInformation {
	return &cfgcore.ConfigDiffInformation{
		UpdateConfig: map[string][]byte{
			"test": []byte(testData),
		},
		AddConfig:    add,
		DeleteConfig: delete,
	}
}

func TestIsUpdateDynamicParameters(t *testing.T) {
	type args struct {
		tpl  *dbaasv1alpha1.ConfigConstraintSpec
		diff *cfgcore.ConfigDiffInformation
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
			tpl:  &dbaasv1alpha1.ConfigConstraintSpec{},
			diff: newCfgDiffMeta(`null`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// error
		args: args{
			tpl:  &dbaasv1alpha1.ConfigConstraintSpec{},
			diff: newCfgDiffMeta(`invalid json formatter`, nil, nil),
		},
		want:    false,
		wantErr: true,
	}, {
		name: "test",
		// add/delete config file
		args: args{
			tpl:  &dbaasv1alpha1.ConfigConstraintSpec{},
			diff: newCfgDiffMeta(`{}`, map[string]interface{}{"a": "b"}, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// not set static or dynamic parameters
		args: args{
			tpl:  &dbaasv1alpha1.ConfigConstraintSpec{},
			diff: newCfgDiffMeta(`{"a":"b"}`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// static parameters contains
		args: args{
			tpl: &dbaasv1alpha1.ConfigConstraintSpec{
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
			tpl: &dbaasv1alpha1.ConfigConstraintSpec{
				StaticParameters: []string{"param1", "param2", "param3"},
			},
			diff: newCfgDiffMeta(`{"param4":"b"}`, nil, nil),
		},
		want:    false,
		wantErr: false,
	}, {
		name: "test",
		// dynamic parameters contains
		args: args{
			tpl: &dbaasv1alpha1.ConfigConstraintSpec{
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
			tpl: &dbaasv1alpha1.ConfigConstraintSpec{
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
			tpl: &dbaasv1alpha1.ConfigConstraintSpec{
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
			got, err := isUpdateDynamicParameters(tt.args.tpl, tt.args.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("isUpdateDynamicParameters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isUpdateDynamicParameters() got = %v, want %v", got, tt.want)
			}
		})
	}
}
