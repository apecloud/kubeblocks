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

package gotemplate

import (
	"reflect"
	"testing"
)

func Test_regexStringSubmatch(t *testing.T) {
	type args struct {
		regex string
		s     string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{{
		name: "test",
		args: args{
			regex: `^(\d+)K$`,
			s:     "123K",
		},
		want:    []string{"123K", "123"},
		wantErr: false,
	}, {
		name: "test",
		args: args{
			regex: `^(\d+)M$`,
			s:     "123",
		},
		want:    nil,
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := regexStringSubmatch(tt.args.regex, tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("regexStringSubmatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("regexStringSubmatch() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fromYAML(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{{
		name: "test",
		args: args{
			str: ``,
		},
		want: map[string]interface{}{},
	}, {
		name: "test",
		args: args{
			str: `efg`,
		},
		want:    map[string]interface{}{},
		wantErr: true,
	}, {
		name: "test",
		args: args{
			str: `a: 
                    b: "c"
                    c: "d"
`,
		},
		want: map[string]interface{}{
			"a": map[interface{}]interface{}{
				"b": "c",
				"c": "d",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fromYAML(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("fromYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fromYAML() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fromYAMLArray(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		want    []interface{}
		wantErr bool
	}{{
		name: "test",
		args: args{
			str: ``,
		},
		want: nil,
	}, {
		name: "test",
		args: args{
			str: `abc: efg`,
		},
		wantErr: true,
	}, {
		name: "test",
		args: args{
			str: `
- a
- b
- c
`,
		},
		want: []interface{}{
			"a",
			"b",
			"c",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fromYAMLArray(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("fromYAMLArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fromYAMLArray() got = %v, want %v", got, tt.want)
			}
		})
	}
}
