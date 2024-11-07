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

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type taskService struct {
	logger        logr.Logger
	actionService *actionService
	tasks         []proto.Task
}

func (s *taskService) runTasks(ctx context.Context) error {
	for _, task := range s.tasks {
		if err := s.runTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (s *taskService) runTask(ctx context.Context, task proto.Task) error {
	// TODO: report task status to controller
	switch {
	case task.DataLoad != nil:
		return (&dataLoadTask{
			logger:        s.logger,
			actionService: s.actionService,
		}).run(ctx, task.DataLoad)
	default:
		return nil
	}
}
