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
