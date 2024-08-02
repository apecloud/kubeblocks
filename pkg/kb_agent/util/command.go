/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

type Executor interface {
	ExecCommand(ctx context.Context, command []string, envs []string) (string, error)
}

type ExecutorImpl struct{}

func (e *ExecutorImpl) ExecCommand(ctx context.Context, command []string, envs []string) (string, error) {
	return ExecCommand(ctx, command, envs)
}

func ExecCommand(ctx context.Context, command []string, envs []string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("command can not be empty")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = envs
	bytes, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		err = errors.New(string(exitErr.Stderr))
	}
	return string(bytes), err
}

func GetAllEnvs(args map[string]any) []string {
	envs := os.Environ()
	for k, v := range args {
		env := fmt.Sprintf("%s=%v", strings.ToUpper(k), v)
		envs = append(envs, env)
	}
	return envs
}
