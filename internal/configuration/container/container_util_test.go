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

package container

import (
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"

	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

func testContainer(service string, id string, state string) types.Container {
	name := "/" + strings.TrimPrefix(id, "/")
	workingDir, _ := filepath.Abs(".")
	return types.Container{
		ID:    id,
		Names: []string{name},
		Labels: testdbaas.WithMap("service", service,
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

func TestExecShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *exec.Cmd
		want    string
		wantErr bool
	}{{
		name:    "pwd_test",
		cmd:     exec.Command("env"),
		want:    "",
		wantErr: false,
	}, {
		name:    "pwd_test",
		cmd:     exec.Command("not_command"),
		want:    "",
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExecShellCommand(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecShellCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("ExecShellCommand() got = %v, want %v", got, tt.want)
			}
		})
	}
}
