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

package util

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

func ExecShellCommand(cmd *exec.Cmd) (string, error) {
	var (
		errOut bytes.Buffer
		stdOut bytes.Buffer
	)

	cmd.Stderr = &errOut
	cmd.Stdout = &stdOut
	if err := cmd.Run(); err != nil {
		return "", errors.Wrapf(err, "failed to run command[%s], error output: \n%v", cmd.String(), errOut.String())
	}

	ret := stdOut.String()
	return ret, nil
}

func RunShellCommand(cmd string, args ...string) (string, error) {
	return ExecShellCommand(exec.Command(cmd, args...))
}
