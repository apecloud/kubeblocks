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

package util

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestCheckPathExists(t *testing.T) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "test-")
	defer os.RemoveAll(tmpDir)

	var (
		testFile1 = filepath.Join(tmpDir, "for_test.yaml")
		testFile2 = filepath.Join(tmpDir, "for_not_test.yaml")
	)

	if err := os.WriteFile(testFile1, []byte("for_test"), fs.ModePerm); err != nil {
		t.Errorf("failed to  write file: %s", testFile1)
	}

	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			path: testFile1,
		},
		want: true,
	}, {
		name: "not_exist_test",
		args: args{
			path: testFile2,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckPathExists(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPathExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CheckPathExists() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromConfigFiles(t *testing.T) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "test-")
	defer os.RemoveAll(tmpDir)

	type args struct {
		files map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			files: map[string]string{
				"test.yaml":  "---",
				"test2.conf": "test2",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir := filepath.Join(tmpDir, "test", tt.name)
			_ = os.MkdirAll(workDir, fs.ModePerm)
			sets := NewSet()
			for f, context := range tt.args.files {
				sets.Add(filepath.Join(workDir, f))
				if err := os.WriteFile(filepath.Join(workDir, f), []byte(context), fs.ModePerm); err != nil {
					t.Errorf("failed to  write file: %s", f)
				}
			}
			if tt.want == nil {
				tt.want = tt.args.files
			}
			got, err := FromConfigFiles(sets.AsSlice())
			if (err != nil) != tt.wantErr {
				t.Errorf("FromConfigFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromConfigFiles() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromYamlConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "test-")
	defer os.RemoveAll(tmpDir)

	type args[T any] struct {
		obj T
	}
	type testCase[T any] struct {
		name        string
		args        args[T]
		expectedObj T
		wantErr     bool
	}
	tests := []testCase[struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`
		Status            v1alpha1.Phase `json:"status"`
	}]{{
		name: "normal_test",
		args: args[struct {
			metav1.TypeMeta   `json:",inline"`
			metav1.ObjectMeta `json:"metadata,omitempty"`
			Status            v1alpha1.Phase `json:"status"`
		}]{
			obj: struct {
				metav1.TypeMeta   `json:",inline"`
				metav1.ObjectMeta `json:"metadata,omitempty"`
				Status            v1alpha1.Phase `json:"status"`
			}{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigConstraint",
					APIVersion: "apps.kubeblocks.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "for_test",
				},
				Status: v1alpha1.AvailablePhase,
			},
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := ToYamlConfig(tt.args.obj)
			if err != nil {
				t.Errorf("ToYamlConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			f := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(f, b, fs.ModePerm); err != nil {
				t.Errorf("failed to  write file: %s", f)
			}
			if err := FromYamlConfig(f, &tt.expectedObj); (err != nil) != tt.wantErr {
				t.Errorf("FromYamlConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.obj, tt.expectedObj) {
				t.Errorf("FromYamlConfig() = %v, want %v", tt.args.obj, tt.expectedObj)
			}
		})
	}
}

func TestToArgs(t *testing.T) {
	type args struct {
		m map[string]string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{{
		name: "normal_test",
		args: args{
			m: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		want: []string{"key1", "value1", "key2", "value2"},
	}, {
		name: "normal_test",
		args: args{
			m: map[string]string{
				"key": "",
			},
		},
		want: []string{"key", ""},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewSet(ToArgs(tt.args.m)...)
			for _, v := range tt.want {
				require.True(t, r.InArray(v))
			}
		})
	}
}
