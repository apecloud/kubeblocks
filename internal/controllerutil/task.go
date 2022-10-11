/*
Copyright 2022 The KubeBlocks Authors

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
package controllerutil

import (
	"context"

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
type TaskFunction func(context.Context, client.Client, interface{}) error

func (t *Task) Exec(ctx context.Context, cli client.Client) error {
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
