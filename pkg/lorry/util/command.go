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
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ExecCommand(ctx context.Context, command []string, envs []string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("command can not be empty")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = envs
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return "", err
	}

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	err := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		err = errors.New(string(exitErr.Stderr))
	}

	return buf.String(), err
}

func GetGlobalSharedEnvs() ([]string, error) {
	envSetRequired := sets.New(
		constant.KBEnvPodFQDN,
		constant.KBEnvServicePort,
		constant.KBEnvServiceUser,
		constant.KBEnvServicePassword,
	)
	envSetGot := sets.KeySet(map[string]string{})
	envs := make([]string, 0, envSetRequired.Len())
	Es := os.Environ()
	for _, env := range Es {
		keys := strings.SplitN(env, "=", 2)
		if len(keys) != 2 {
			continue
		}
		if envSetRequired.Has(keys[0]) {
			envs = append(envs, env)
			envSetGot.Insert(keys[0])
		}
	}
	// if len(envs) != envSetRequired.Len() {
	// 	return nil, errors.Errorf("%v envs is unset", sets.List(envSetRequired.Difference(envSetGot)))
	// }

	return envs, nil
}
