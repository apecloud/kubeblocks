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

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestCheckExcludeConfigDifference(t *testing.T) {
	type args struct {
		oldVersion map[string]string
		newVersion map[string]string
		keys       []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "emptyKeys",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
			},
			newVersion: map[string]string{
				"a": "test1",
			},
		},
		want: false,
	}, {
		name: "emptyKeys",
		args: args{
			oldVersion: map[string]string{
				"a": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
			},
		},
		want: true,
	}, {
		name: "emptyKeys",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
			},
			newVersion: map[string]string{
				"b": "test1",
			},
		},
		want: true,
	}, {
		name: "keyTest",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
				"b": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
				"b": "test3",
			},
			keys: []string{"b"},
		},
		want: false,
	}, {
		name: "keyTest",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
				"b": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
				"b": "test3",
			},
			keys: []string{"a"},
		},
		want: true,
	}, {
		name: "keyTest",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
				"b": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
				"b": "test3",
			},
			keys: []string{"a", "b"},
		},
		want: false,
	}, {
		name: "keyCountTest",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
				"b": "test2",
				"c": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
				"b": "test3",
			},
			keys: []string{"b"},
		},
		want: true,
	}, {
		name: "notKeyTest",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
				"b": "test2",
				"c": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
				"b": "test3",
			},
			keys: []string{"b", "e", "f", "g"},
		},
		want: true,
	}, {
		name: "notKeyTest",
		args: args{
			oldVersion: map[string]string{
				"a": "test1",
				"b": "test2",
				"c": "test2",
			},
			newVersion: map[string]string{
				"a": "test1",
				"b": "test3",
			},
			keys: []string{"b", "c", "f", "g"},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkExcludeConfigDifference(tt.args.oldVersion, tt.args.newVersion, tt.args.keys); got != tt.want {
				t.Errorf("checkExcludeConfigDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateConfigurePatch(t *testing.T) {
	v1 := `
[mysqld]
key_buffer_size=16777216
max_connections=2666
authentication_policy=mysql_native_password,
back_log=5285
binlog_cache_size=32768
binlog_format=MIXED
binlog_order_commits=ON
binlog_row_image=FULL
connect_timeout=10
`
	v2 := `
[mysqld]
authentication_policy=mysql_native_password,
back_log=5285
binlog_cache_size=32768
binlog_format=MIXED
binlog_order_commits=ON
binlog_row_image=FULL
connect_timeout=10
key_buffer_size=18874368
max_connections=666
`

	type args struct {
		oldVersion        map[string]string
		newVersion        map[string]string
		format            v1alpha1.CfgFileFormat
		keys              []string
		enableExcludeDiff bool
	}
	tests := []struct {
		name        string
		args        args
		want        *ConfigPatchInfo
		excludeDiff bool
		wantErr     bool
	}{{
		name: "patchTestWithoutKeys",
		args: args{
			oldVersion: map[string]string{
				"my.cnf": v1,
			},
			newVersion: map[string]string{
				"my.cnf": v2,
			},
			format:            v1alpha1.Ini,
			enableExcludeDiff: true,
		},
		want:        &ConfigPatchInfo{IsModify: true},
		excludeDiff: false,
	}, {
		name: "failedPatchTestWithoutKeys",
		args: args{
			oldVersion: map[string]string{
				"my.cnf":    v1,
				"other.cnf": "context",
			},
			newVersion: map[string]string{
				"my.cnf":    v2,
				"other.cnf": "context",
			},
			format:            v1alpha1.Ini,
			enableExcludeDiff: true,
		},
		want:        &ConfigPatchInfo{IsModify: true},
		excludeDiff: false,
		wantErr:     true,
	}, {
		name: "patchTest",
		args: args{
			oldVersion: map[string]string{
				"my.cnf":    v1,
				"other.cnf": "context",
			},
			newVersion: map[string]string{
				"my.cnf":    v2,
				"other.cnf": "context",
			},
			keys:              []string{"my.cnf"},
			format:            v1alpha1.Ini,
			enableExcludeDiff: true,
		},
		want:        &ConfigPatchInfo{IsModify: true},
		excludeDiff: false,
	}, {
		name: "patchTest",
		args: args{
			oldVersion: map[string]string{
				"my.cnf":    v1,
				"other.cnf": "context",
			},
			newVersion: map[string]string{
				"my.cnf":    v1,
				"other.cnf": "context difference",
			},
			keys:              []string{"my.cnf"},
			format:            v1alpha1.Ini,
			enableExcludeDiff: true,
		},
		want:        &ConfigPatchInfo{IsModify: false},
		excludeDiff: true,
	}, {
		name: "patchTest",
		args: args{
			oldVersion: map[string]string{
				"my.cnf":    v1,
				"other.cnf": "context",
			},
			newVersion: map[string]string{
				"my.cnf":    v2,
				"other.cnf": "context difference",
			},
			keys:              []string{"my.cnf"},
			format:            v1alpha1.Ini,
			enableExcludeDiff: false,
		},
		want:        &ConfigPatchInfo{IsModify: true},
		excludeDiff: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, excludeDiff, err := CreateConfigPatch(tt.args.oldVersion, tt.args.newVersion, tt.args.format, tt.args.keys, tt.args.enableExcludeDiff)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConfigPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.IsModify != tt.want.IsModify {
				t.Errorf("CreateConfigPatch() got = %v, want %v", got, tt.want)
			}
			if excludeDiff != tt.excludeDiff {
				t.Errorf("CreateConfigPatch() got1 = %v, want %v", excludeDiff, tt.excludeDiff)
			}
		})
	}
}

func TestLoadRawConfigObject(t *testing.T) {
	getFileContentFn := func(file string) string {
		content, _ := testdata.GetTestDataFileContent(file)
		return string(content)
	}

	type args struct {
		data         map[string]string
		formatConfig *v1alpha1.FormatterConfig
		keys         []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "test",
		args: args{
			data: map[string]string{"key": getFileContentFn("cue_testdata/mysql.cnf")},
			formatConfig: &v1alpha1.FormatterConfig{
				Format: v1alpha1.Ini,
				FormatterOptions: v1alpha1.FormatterOptions{
					IniConfig: &v1alpha1.IniConfig{
						SectionName: "mysqld",
					}},
			}},
		wantErr: false,
	}, {
		name: "test",
		args: args{
			data: map[string]string{"key": getFileContentFn("cue_testdata/pg14.conf")},
			formatConfig: &v1alpha1.FormatterConfig{
				Format: v1alpha1.Properties,
			}},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadRawConfigObject(tt.args.data, tt.args.formatConfig, tt.args.keys)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRawConfigObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestTransformConfigFileToKeyValueMap(t *testing.T) {
	mysqlConfig := `
[mysqld]
key_buffer_size=16777216
log_error=/data/mysql/logs/mysql.log
`
	mongodbConfig := `
systemLog:
  logRotate: reopen
  path: /data/mongodb/logs/mongodb.log
  verbosity: 0
`
	tests := []struct {
		name         string
		fileName     string
		formatConfig *v1alpha1.FormatterConfig
		configData   []byte
		expected     map[string]string
	}{{
		name:     "mysql-test",
		fileName: "my.cnf",
		formatConfig: &v1alpha1.FormatterConfig{
			Format: v1alpha1.Ini,
			FormatterOptions: v1alpha1.FormatterOptions{
				IniConfig: &v1alpha1.IniConfig{
					SectionName: "mysqld",
				},
			},
		},
		configData: []byte(mysqlConfig),
		expected: map[string]string{
			"key_buffer_size": "16777216",
			"log_error":       "/data/mysql/logs/mysql.log",
		},
	}, {
		name:     "mongodb-test",
		fileName: "mongodb.conf",
		formatConfig: &v1alpha1.FormatterConfig{
			Format: v1alpha1.YAML,
			FormatterOptions: v1alpha1.FormatterOptions{
				IniConfig: &v1alpha1.IniConfig{
					SectionName: "default",
				},
			},
		},
		configData: []byte(mongodbConfig),
		expected: map[string]string{
			"systemLog.logRotate": "reopen",
			"systemLog.path":      "/data/mongodb/logs/mongodb.log",
			"systemLog.verbosity": "0",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := TransformConfigFileToKeyValueMap(tt.fileName, tt.formatConfig, tt.configData)
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("TransformConfigFileToKeyValueMap() res = %v, res %v", res, tt.expected)
				return
			}
		})
	}
}
