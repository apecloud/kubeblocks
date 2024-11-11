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
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

type taskService struct {
	logger        logr.Logger
	actionService *actionService
	tasks         []proto.Task
}

type task interface {
	run(ctx context.Context) (chan error, error)
	status(ctx context.Context, event *proto.TaskEvent)
}

func (s *taskService) runTasks(ctx context.Context) error {
	for _, task := range s.tasks {
		// run tasks one by one
		if slices.Contains(strings.Split(task.Replicas, ","), util.PodName()) {
			start := time.Now()
			if err := s.runTask(ctx, task); err != nil {
				s.logger.Error(err, fmt.Sprintf("failed to run task: %v, time elapsed: %s", task, time.Since(start)))
				return err
			}
			s.logger.Info(fmt.Sprintf("succeed to run task: %v, time elapsed: %s", task, time.Since(start)))
		}
	}
	return nil
}

func (s *taskService) runTask(ctx context.Context, task proto.Task) error {
	t := s.newTask(task)
	if t == nil {
		return nil
	}

	event := proto.TaskEvent{
		Instance:  task.Instance,
		Task:      task.Task,
		UID:       task.UID,
		Replica:   util.PodName(),
		StartTime: time.Now(),
	}

	notify := func(err error, exit, exited chan struct{}) error {
		if exit != nil && exited != nil {
			close(exit)
			<-exited
		}
		if task.NotifyAtFinish {
			event.EndTime = time.Now()
			if err == nil {
				event.Code = 0
			} else {
				event.Code = -1
				event.Message = err.Error()
			}
			err1 := s.notify(task, event, true)
			if err == nil { // the run error takes precedence
				err = err1
			}
		}
		return err
	}

	ch, err1 := t.run(ctx)
	if err1 != nil {
		return notify(err1, nil, nil)
	}

	exit, exited := s.report(ctx, task, t, event)

	return notify(s.wait(ch), exit, exited)
}

func (s *taskService) newTask(task proto.Task) task {
	if task.NewReplica != nil {
		return &newReplicaTask{
			logger:        s.logger,
			actionService: s.actionService,
			task:          task.NewReplica,
		}
	}
	return nil
}

func (s *taskService) report(ctx context.Context, task proto.Task, t task, event proto.TaskEvent) (chan struct{}, chan struct{}) {
	if task.ReportPeriodSeconds > 0 {
		exit, exited := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(exited)
			ticker := time.NewTicker(time.Duration(task.ReportPeriodSeconds) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-exit:
					return
				case <-ticker.C:
					eventCopy := event
					t.status(ctx, &event)
					if !reflect.DeepEqual(event, eventCopy) {
						_ = s.notify(task, event, false)
					}
				}
			}
		}()
		return exit, exited
	}
	return nil, nil
}

func (s *taskService) wait(ch chan error) error {
	if ch != nil {
		err, ok := <-ch
		if !ok {
			err = errors.New("runtime error: error chan closed unexpectedly")
		}
		return err
	}
	return nil
}

func (s *taskService) notify(task proto.Task, event proto.TaskEvent, sync bool) error {
	msg, err := json.Marshal(&event)
	if err == nil {
		return util.SendEventWithMessage(&s.logger, "task", string(msg), sync)
	} else {
		s.logger.Error(err, fmt.Sprintf("failed to marshal task event, task: %v", task))
		return err
	}
}
