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

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type ExecHandler struct {
	HandlerBase

	Executor util.Executor
}

var _ Handler = &ExecHandler{}

func NewExecHandler(properties map[string]string) (*ExecHandler, error) {
	logger := ctrl.Log.WithName("EXEC handler")

	handlerBase, err := NewHandlerBase(logger)
	if err != nil {
		return nil, err
	}

	h := &ExecHandler{
		HandlerBase: *handlerBase,
		Executor:    &util.ExecutorImpl{},
	}

	return h, nil
}

func (h *ExecHandler) Do(ctx context.Context, setting util.Handlers, args map[string]any) (map[string]any, error) {
	if len(setting.Command) == 0 {
		h.Logger.Info("action command is empty!")
		return nil, nil
	}
	envs := util.GetAllEnvs(args)
	h.Logger.Info("execute action", "commands", setting.Command, "envs", envs)
	output, err := h.Executor.ExecCommand(ctx, setting.Command, envs)
	var result map[string]any
	if output != "" {
		h.Logger.Info("execute action", "output", output)
		result = map[string]any{"output": output}
	}
	return result, err
}
