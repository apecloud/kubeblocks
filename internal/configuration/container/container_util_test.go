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

package container

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"

	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

func testContainer(service string, id string, state string) types.Container {
	name := "/" + strings.TrimPrefix(id, "/")
	workingDir, _ := filepath.Abs(".")
	return types.Container{
		ID:    id,
		Names: []string{name},
		Labels: testapps.WithMap("service", service,
			"working_dir", workingDir,
			"project", "test_project"),
		State: state,
	}
}

func TestIsSocketFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "SocketFileTest-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	var (
		testFile1 = filepath.Join(tmpDir, "file1")
		testFile2 = filepath.Join(tmpDir, "file2")
		testFile3 = filepath.Join(tmpDir, "file3.sock")
	)

	if err := os.WriteFile(testFile2, []byte(``), fs.ModePerm); err != nil {
		t.Errorf("failed to  write file: %s", testFile2)
	}

	l, err := net.Listen("unix", testFile3)
	if err != nil {
		t.Errorf("failed to  create socket file: %s", testFile3)
	}
	defer l.Close()

	tests := []struct {
		name string
		args string
		want bool
	}{{
		name: "socketTest",
		args: testFile1,
		want: false,
	}, {
		name: "socketTest",
		args: testFile2,
		want: false,
	}, {
		name: "socketTest",
		args: testFile3,
		want: true,
	}, {
		name: "socketTest",
		args: formatSocketPath(testFile3),
		want: true,
	}, {
		name: "socketTest",
		// for test formatSocketPath
		args: formatSocketPath(formatSocketPath(testFile3)),
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSocketFile(tt.args); got != tt.want {
				t.Errorf("isSocketFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
