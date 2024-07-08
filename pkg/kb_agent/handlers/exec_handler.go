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

package handlers

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type ExecHandler struct {
	Logger logr.Logger

	Executor util.Executor
}

var _ Handler = &ExecHandler{}

func NewExecHandler(properties map[string]string) (*ExecHandler, error) {
	logger := ctrl.Log.WithName("EXEC handler")

	h := &ExecHandler{
		Logger:   logger,
		Executor: &util.ExecutorImpl{},
	}

	return h, nil
}

func (h *ExecHandler) Do(ctx context.Context, setting util.HandlerSpec, args map[string]any) (*Response, error) {
	if len(setting.Command) == 0 {
		h.Logger.Info("action command is empty!")
		return nil, nil
	}
	envs := util.GetAllEnvs(args)
	h.Logger.Info("execute action", "commands", setting.Command, "envs", envs)
	if setting.TimeoutSeconds > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(setting.TimeoutSeconds)*time.Second)
		defer cancel()
		ctx = timeoutCtx
	}

	output, err := h.Executor.ExecCommand(ctx, setting.Command, envs)

	if err != nil {
		return nil, errors.Wrap(err, "ExecHandler executes action failed")
	}

	h.Logger.V(1).Info("execute action", "output", output)
	resp := &Response{
		Message: output,
	}
	return resp, err
}
