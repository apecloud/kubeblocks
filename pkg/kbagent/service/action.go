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
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	actionServiceName    = "Action"
	actionServiceVersion = "v1.0"
)

func newActionService(logger logr.Logger, actions []proto.Action) (*actionService, error) {
	sa := &actionService{
		logger:         logger,
		actions:        make(map[string]*proto.Action),
		runningActions: map[string]*runningAction{},
	}
	for i, action := range actions {
		sa.actions[action.Name] = &actions[i]
	}
	logger.Info(fmt.Sprintf("create service %s", sa.Kind()), "actions", strings.Join(maps.Keys(sa.actions), ","))
	return sa, nil
}

type actionService struct {
	logger         logr.Logger
	actions        map[string]*proto.Action
	runningActions map[string]*runningAction
}

type runningAction struct {
	stdoutChan chan []byte
	stderrChan chan []byte
	errChan    chan error
}

var _ Service = &actionService{}

func (s *actionService) Kind() string {
	return actionServiceName
}

func (s *actionService) Version() string {
	return actionServiceVersion
}

func (s *actionService) Start() error {
	return nil
}

func (s *actionService) Decode(payload []byte) (interface{}, error) {
	req := &proto.ActionRequest{}
	if err := json.Unmarshal(payload, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *actionService) HandleRequest(ctx context.Context, i interface{}) ([]byte, error) {
	req := i.(*proto.ActionRequest)
	if _, ok := s.actions[req.Action]; !ok {
		return nil, errors.Wrap(ErrNotDefined, fmt.Sprintf("%s is not defined", req.Action))
	}
	return s.handleActionRequest(ctx, req)
}

func (s *actionService) handleActionRequest(ctx context.Context, req *proto.ActionRequest) ([]byte, error) {
	action := s.actions[req.Action]
	if action.Exec != nil {
		return s.handleExecAction(ctx, req, action)
	}
	return nil, errors.Wrap(ErrNotImplemented, "only exec action is supported")
}

func (s *actionService) handleExecAction(ctx context.Context, req *proto.ActionRequest, action *proto.Action) ([]byte, error) {
	if req.NonBlocking != nil && *req.NonBlocking {
		return s.handleExecActionNonBlocking(ctx, req, action)
	}
	return runCommand(ctx, action.Exec, req.Parameters, req.TimeoutSeconds)
}

func (s *actionService) handleExecActionNonBlocking(ctx context.Context, req *proto.ActionRequest, action *proto.Action) ([]byte, error) {
	running, ok := s.runningActions[req.Action]
	if !ok {
		stdoutChan, stderrChan, errChan, err := runCommandNonBlocking(ctx, action.Exec, req.Parameters, req.TimeoutSeconds)
		if err != nil {
			return nil, err
		}
		running = &runningAction{
			stdoutChan: stdoutChan,
			stderrChan: stderrChan,
			errChan:    errChan,
		}
		s.runningActions[req.Action] = running
	}
	err := gather(running.errChan)
	if err == nil {
		return nil, ErrInProgress
	}
	if *err != nil {
		return nil, *err
	}
	return *gather(running.stdoutChan), nil
}
