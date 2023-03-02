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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

const (
	socketPrefix = "unix://"
)

func ExecShellCommand(cmd *exec.Cmd) (string, error) {
	var (
		errOut bytes.Buffer
		stdOut bytes.Buffer
	)

	cmd.Stderr = &errOut
	cmd.Stdout = &stdOut
	if err := cmd.Run(); err != nil {
		return "", cfgcore.WrapError(err, "failed to run command[%s], error output: \n%v", cmd.String(), errOut.String())
	}

	ret := stdOut.String()
	return ret, nil
}

func isSocketFile(file string) bool {
	info, err := os.Stat(extractSocketPath(file))
	if err != nil {
		return false
	}
	if info.Mode()&fs.ModeSocket == fs.ModeSocket {
		return true
	}
	return false
}

func formatSocketPath(path string) string {
	if strings.HasPrefix(path, socketPrefix) {
		return path
	}
	return fmt.Sprintf("%s%s", socketPrefix, path)
}

func extractSocketPath(path string) string {
	return strings.TrimPrefix(path, socketPrefix)
}
