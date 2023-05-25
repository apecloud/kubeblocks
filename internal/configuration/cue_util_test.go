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
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/require"
)

func TestCue(t *testing.T) {

	cueTes := `
// Release notes:
// - You can now specify your age and your hobby!
#V1: {
    age:   >=0 & <=100
    hobby: string
}
// Release notes:
// - People get to be older than 100, so we relaxed it.
// - It seems not many people have a hobby, so we made it optional.
#V2: {
    age:    >=0 & <=150 // people get older now
    hobby?: string      // some people don't have a hobby
}
// Release notes:
// - Actually no one seems to have a hobby nowadays anymore, so we dropped the field.
#V3: {
    age: >=0 & <=150
}

x1: {
	age : 99
}

x2: {
	age : 180
}

x3: {
	age : 180
}

`

	context := cuecontext.New()
	inst := context.CompileString(cueTes)
	// V1 ⊆ V2
	// V2 ⊆ V3
	v1, err1 := inst.LookupField("#V1")
	v2, err2 := inst.LookupField("#V2")
	v3, err3 := inst.LookupField("#V3")
	if err1 != nil || err2 != nil || err3 != nil {
		t.Fail()
	}

	// test := V2 ∪ V1
	test := v2.Value.Unify(v1.Value)
	v2.Value.Eval()

	// test ⊆ V1
	// test ⊆ V2
	require.False(t, test.Subsumes(v2.Value))
	require.True(t, test.Subsumes(v1.Value))
	require.True(t, v1.Value.Subsumes(test))
	require.True(t, v2.Value.Subsumes(test))

	// Check if V2 is backwards compatible with V1
	require.True(t, v2.Value.Subsumes(v1.Value)) // true

	// Check if V3 is backwards compatible with V2
	require.False(t, v3.Value.Subsumes(v2.Value)) // false
}

func TestCfgDataValidateByCue(t *testing.T) {
	type args struct {
		cueTpl string
		data   []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			cueTpl: `
		#SectionType: {
		   slow_query_log?: string & "0" | "2" | "OFF" | "ON" | *"OFF"
		   auto_increment_increment?: int & >= 1 & <= 65535 | *1
		   innodb_buffer_pool_size?: int & >= 5242880 & <= 18446744073709551615 @k8sResource(quantity)
		   innodb_autoinc_lock_mode?: int & 0 | 1 | 2 | *2
			port?: int
		}
		[SectionName=_] : #SectionType
		`,
			data: []string{`
		{ "section" : {
			"slow_query_log":"OFF",
			"auto_increment_increment":"28",
			"innodb_buffer_pool_size":"1024M",
			"port":"306"
		}}`},
		},
		wantErr: false,
	}, {
		name: "normal_test_for_bool",
		args: args{
			cueTpl: `slow_query_log: string & "0" | "1" | "OFF" | "ON" | *"OFF"`,
			data: []string{
				`{"slow_query_log": "0"}`,
				`{"slow_query_log": "1"}`,
				`{"slow_query_log": "OFF"}`,
				`{"slow_query_log": "ON"}`,
			},
		},
		wantErr: false,
	}, {
		name: "normal_test_for_bool_failed",
		args: args{
			cueTpl: `slow_query_log: string & "0" | "1" | "OFF" | "ON" | *"OFF"`,
			data: []string{
				`{"slow_query_log": 0}`,
				`{"slow_query_log": 1}`,
				`{"slow_query_log": "O"}`,
			},
		},
		wantErr: true,
	}, {
		name: "normal_test_for_string",
		args: args{
			cueTpl: `innodb_buffer_pool_size?: int & >= 5242880 & <= 18446744073709551615 @k8sResource(quantity)`,
			data: []string{
				"{}",
				`{"innodb_buffer_pool_size": "512M"}`,
				`{"innodb_buffer_pool_size": "1024M"}`,
				`{"innodb_buffer_pool_size": "1024000000"}`,
				`{"innodb_buffer_pool_size": "1G"}`,
				`{"innodb_buffer_pool_size": "1Gi"}`,
			},
		},
		wantErr: false,
	}, {
		name: "normal_test_for_string",
		args: args{
			cueTpl: `innodb_buffer_pool_size: int & >= 5242880 & <= 18446744073709551615 @k8sResource(quantity)`,
			data: []string{
				`{"innodb_buffer_pool_size": "5M"}`,
				`{"innodb_buffer_pool_size": "1024"}`,
				`{"innodb_buffer_pool_size": "199K"}`,
				`{"innodb_buffer_pool_size": "abcd"}`,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Nil(t, CueValidate(tt.args.cueTpl))
			for _, v := range tt.args.data {
				var unstructedObj any
				require.Nil(t, json.Unmarshal([]byte(v), &unstructedObj))
				if err := unstructuredDataValidateByCue(tt.args.cueTpl, unstructedObj, false); (err != nil) != tt.wantErr {
					t.Errorf("unstructuredDataValidateByCue() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
