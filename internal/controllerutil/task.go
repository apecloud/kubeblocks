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

package controllerutil

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TaskType string

const (
	TaskTypeSerial   TaskType = "serial"
	TaskTypeParallel TaskType = "parallel"
)

type Task struct {
	Type         TaskType
	SubTasks     []Task
	PreFunction  TaskFunction
	ExecFunction TaskFunction
	PostFunction TaskFunction
	Context      map[string]interface{}
}

func NewTask() Task {
	t := Task{}
	t.Context = map[string]interface{}{}
	return t
}

// TaskFunction REVIEW (cx): using interface{} is rather error-prone
type TaskFunction func(RequestCtx, client.Client, interface{}) error

func (t *Task) Exec(ctx RequestCtx, cli client.Client) error {
	if t.PreFunction != nil {
		if err := t.PreFunction(ctx, cli, t.Context["pre"]); err != nil {
			return err
		}
	}
	if len(t.SubTasks) == 0 {
		if t.ExecFunction != nil {
			if err := t.ExecFunction(ctx, cli, t.Context["exec"]); err != nil {
				return err
			}
		}
	} else {
		if t.Type == TaskTypeParallel {
			// parallel
		} else {
			for _, subTask := range t.SubTasks {
				if err := subTask.Exec(ctx, cli); err != nil {
					return err
				}
			}
		}
	}
	if t.PostFunction != nil {
		if err := t.PostFunction(ctx, cli, t.Context["post"]); err != nil {
			return err
		}
	}
	return nil
}
