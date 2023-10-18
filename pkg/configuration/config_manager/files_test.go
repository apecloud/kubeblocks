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

package configmanager

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFiles(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to Getwd directory")
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), "files-test-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)
	MakeTestConfigureDirectory(t, tmpDir, "my.test.yaml", "a: b")

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
