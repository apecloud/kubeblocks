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

package util

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

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
