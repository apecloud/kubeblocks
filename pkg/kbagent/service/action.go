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

package service

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

type actionService struct {
	actions *appsv1alpha1.ComponentLifecycleActions
}

func (s *actionService) call(ctx context.Context, action string, parameters map[string]string) ([]byte, error) {
	return nil, nil
}

func execute(ctx context.Context, commands []string, args map[string]any) ([]byte, error) {
	if len(commands) == 0 {
		return nil, fmt.Errorf("commands can not be empty")
	}
	envs := util.GetAllEnvs(args)
	if setting.TimeoutSeconds > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(setting.TimeoutSeconds)*time.Second)
		defer cancel()
		ctx = timeoutCtx
	}

	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)
	cmd.Env = envs
	bytes, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		err = errors.New(string(exitErr.Stderr))
	}
	return bytes, err
}
