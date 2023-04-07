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

package configmanager

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_newFiles(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to Getwd directory")
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), "files-test-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)
	createTestConfigureDirectory(t, tmpDir, "my.test.yaml", "a: b")

	type args struct {
		basePath string
	}
	tests := []struct {
		name    string
		args    args
		file    string
		want    string
		wantErr bool
	}{{
		name: "testFiles",
		args: args{
			basePath: pwd,
		},
		file:    "not_file",
		wantErr: true,
	}, {
		name: "testFiles",
		args: args{
			basePath: tmpDir,
		},
		file: "my.test.yaml",
		want: "a: b",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := newFiles(tt.args.basePath)
			v, err := files.Get(tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.want != v {
				t.Errorf("Get() is not expected %s : %s", v, tt.want)
			}
		})
	}
}
