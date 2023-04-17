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
	"reflect"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/require"
)

func TestCueTypeExtractorVisit(t *testing.T) {
	type args struct {
		cue        string
		fieldTypes map[string]CueType
	}
	tests := []struct {
		name string
		args args
	}{{
		name: "test_without_type",
		args: args{
			cue:        `a:int`,
			fieldTypes: map[string]CueType{"a": IntType},
		},
	}, {
		name: "normal_test",
		args: args{
			cue: `#a:int`,
			fieldTypes: map[string]CueType{
				"#a": IntType,
			},
		},
	}, {
		name: "complex_test",
		args: args{
			cue: `#a: {
		b : #c
		g : #j
		
		#j : {
				"x": string
				"y": int & > 100
				"m": #n
			}
		}
		
		#n : {
			"d" : {}
			"j" : null
		}
		
		#c : {
			e: int
			f: string|float|int & 2000 | "100.10" | 200 | * "100.10"
		}
		`,
			fieldTypes: map[string]CueType{
				"#a":    StructType,
				"b":     StructType,
				"g":     StructType,
				"#c":    StructType,
				"e":     IntType,
				"f":     StringType,
				"#j":    StructType,
				"x":     StringType,
				"y":     IntType,
				"#n":    StructType,
				"m":     StructType,
				"d":     StructType,
				"j":     NullableType,
				"b.e":   IntType,
				"b.f":   StringType,
				"g.x":   StringType,
				"g.y":   IntType,
				"g.m":   StructType,
				"g.m.d": StructType,
				"g.m.j": NullableType,
				"m.d":   StructType,
				"m.j":   NullableType,
			},
		},
	}, {
		name: "map_list_test",
		args: args{
			cue: `#a: {
		b: int
		c: string|int
		d: string & "a" | "b"
		e: string & "a" | "b" | *"a"
		g: [string]: {
			"ga": string
			"zz": int
			"xxx": [...int]
		}
		i:[int]
		}`,
			fieldTypes: map[string]CueType{
				"#a": StructType,
				"b":  IntType,
				"c":  StringType,
				"d":  StringType,
				"e":  StringType,
				"g":  StructType,
				"i":  IntType,
			},
		},
	}, {
		name: "invalid_test",
		args: args{
			cue: `
		a : 100
		b : 20.10
		#c : {
			g : a + b
		}
		`,
			fieldTypes: map[string]CueType{
				"#c": StructType,
				"g":  FloatType,
				"a":  IntType,
				"b":  FloatType,
			},
		},
	}, {
		name: "attr_test",
		args: args{
			cue: `a : int @k8sResource(quantity)`,
			fieldTypes: map[string]CueType{
				"a": K8SQuantityType,
			},
		},
	}, {
		name: "attr_test",
		args: args{
			cue: `a : int @storeResource()`,
			fieldTypes: map[string]CueType{
				"a": ClassicStorageType,
			},
		},
	}, {
		name: "attr_test",
		args: args{
			cue: `a : int @timeDurationResource()`,
			fieldTypes: map[string]CueType{
				"a": ClassicTimeDurationType,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := cuecontext.New()
			tpl := context.CompileString(tt.args.cue)
			require.Nil(t, tpl.Err())
			c := &cueTypeExtractor{
				context: context,
			}
			c.Visit(tpl)
			require.EqualValues(t, tt.args.fieldTypes, c.fieldTypes)
		})
	}
}

func TestTransNumberOrBoolType(t *testing.T) {
	type args struct {
		t        CueType
		objs     []string
		expected []interface{}
		expand   string
		// obj reflect.Value
		// fn  UpdateFn
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "testInt",
		args: args{
			t:        IntType,
			objs:     []string{"100", "-100", "0", "-1", "1"},
			expected: []interface{}{100, -100, 0, -1, 1},
		},
		wantErr: false,
	}, {
		name: "testFloat",
		args: args{
			t:        FloatType,
			objs:     []string{"100.1", "-100.2", "0", "-1.11", "1.11", "1000"},
			expected: []interface{}{100.1, -100.2, 0, -1.11, 1.11, 1000.0},
		},
		wantErr: false,
	}, {
		name: "testBool",
		args: args{
			t:        BoolType,
			objs:     []string{"true", "1", "0", "false", "t", "f"},
			expected: []interface{}{true, true, false, false, true, false},
		},
		wantErr: false,
	}, {
		name: "testBoolFail",
		args: args{
			t:        BoolType,
			objs:     []string{"2.0", "5.6", "abcd", " "},
			expected: []interface{}{true, true, false, false},
		},
		wantErr: true,
	}, {
		name: "testIntFail",
		args: args{
			t:        IntType,
			objs:     []string{"100.0", "abc", "@", "-1.0"},
			expected: []interface{}{100, -100, 0, -1},
		},
		wantErr: true,
	}, {
		name: "testFloatFail",
		args: args{
			t:        FloatType,
			objs:     []string{"abc", " ", "--0.", "a-1.11", " 5"},
			expected: []interface{}{100.1, -100.2, 0, -1.11, 5},
		},
		wantErr: true,
	}, {
		name: "testMemoryType",
		args: args{
			t:        K8SQuantityType,
			objs:     []string{"1Gi", "1G", "10M", "100", "1000m"},
			expected: []interface{}{1024 * 1024 * 1024, 1000 * 1000 * 1000, 10 * 1000 * 1000, 100, 1},
		},
		wantErr: false,
	}, {
		name: "testClassResource",
		args: args{
			t:        ClassicStorageType,
			objs:     []string{"1G", "1GB", "1K", "1M", "1MB", "100T", "10TB", "888", "20mb", "-1"},
			expected: []interface{}{1024 * 1024 * 1024, 1024 * 1024 * 1024, 1024, 1024 * 1024, 1024 * 1024, 100 * TByte, 10 * TByte, 888, 20 * 1024 * 1024, -1},
		},
		wantErr: false,
	}, {
		name: "testClassResource",
		args: args{
			t:        ClassicStorageType,
			objs:     []string{"1G", "1MB", "100T", "10TB"},
			expected: []interface{}{1024 * 1024 / 16, 1024 / 16, 100 * GByte / 16, 10 * GByte / 16},
			expand:   "16KB",
		},
		wantErr: false,
	}, {
		name: "testClassResource",
		args: args{
			t:        ClassicStorageType,
			objs:     []string{"G", "", "1KK", "1o", "1MB1"},
			expected: []interface{}{0, 0, 0, 0, 0},
		},
		wantErr: true,
	}, {
		name: "testClassResource",
		args: args{
			t:        ClassicTimeDurationType,
			objs:     []string{"1", "100", "1s", "1min", "20m", "5d", "10000ms", "20MIN"},
			expected: []interface{}{1, 100, 1000, 60 * 1000, 20 * 60 * 1000, 5 * Day, 10000, 20 * 60 * 1000},
		},
		wantErr: false,
	}, {
		name: "testClassResource",
		args: args{
			t:        ClassicTimeDurationType,
			objs:     []string{"", "100yy", "s", "min45", "second"},
			expected: []interface{}{0, 0, 0, 0, 0},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < len(tt.args.objs); i++ {
				if err := transNumberOrBoolType(tt.args.t, reflect.ValueOf(tt.args.objs[i]), func(v interface{}) {
					require.EqualValues(t, v, tt.args.expected[i])
				}, tt.args.expand, false); (err != nil) != tt.wantErr {
					t.Errorf("transNumberOrBoolType() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
