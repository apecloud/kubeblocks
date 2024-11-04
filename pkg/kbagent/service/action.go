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
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type ReqCtx string

const ReqCtxKey ReqCtx = "reqCtx"

func newActionService(logger logr.Logger, actions []proto.Action) (*actionService, error) {
	sa := &actionService{
		logger:         logger,
		actions:        make(map[string]*proto.Action),
		mutex:          sync.Mutex{},
		runningActions: map[string]*runningAction{},
	}
	for i, action := range actions {
		sa.actions[action.Name] = &actions[i]
	}
	logger.Info(fmt.Sprintf("create service %s", sa.Kind()), "actions", strings.Join(maps.Keys(sa.actions), ","))
	return sa, nil
}

type actionService struct {
	logger  logr.Logger
	actions map[string]*proto.Action

	mutex          sync.Mutex
	runningActions map[string]*runningAction
}

type runningAction struct {
	resultChan chan *commandResult
}

var _ Service = &actionService{}

func (s *actionService) Kind() string {
	return proto.ServiceAction.Kind
}

func (s *actionService) URI() string {
	return proto.ServiceAction.URI
}

func (s *actionService) Start() error {
	return nil
}

func (s *actionService) HandleRequest(ctx context.Context, payload []byte) ([]byte, error) {
	req, err := s.decode(payload)
	if err != nil {
		return s.encode(nil, err), nil
	}
	resp, err := s.handleRequest(ctx, req)
	result := string(resp)
	if err != nil {
		result = err.Error()
	}
	s.logger.Info("Action Executed", "Action", req.Action, "result", result)
	return s.encode(resp, err), nil
}

func (s *actionService) decode(payload []byte) (*proto.ActionRequest, error) {
	req := &proto.ActionRequest{}
	if err := json.Unmarshal(payload, req); err != nil {
		return nil, errors.Wrapf(proto.ErrBadRequest, "unmarshal action request error: %s", err.Error())
	}
	return req, nil
}

func (s *actionService) encode(out []byte, err error) []byte {
	rsp := &proto.ActionResponse{}
	if err == nil {
		rsp.Output = out
	} else {
		rsp.Error = proto.Error2Type(err)
		rsp.Message = err.Error()
	}
	data, _ := json.Marshal(rsp)
	return data
}

func (s *actionService) handleRequest(ctx context.Context, req *proto.ActionRequest) ([]byte, error) {
	if _, ok := s.actions[req.Action]; !ok {
		return nil, errors.Wrapf(proto.ErrNotDefined, "%s is not defined", req.Action)
	}
	action := s.actions[req.Action]
	if action.Exec == nil {
		return nil, errors.Wrap(proto.ErrNotImplemented, "only exec action is supported")
	}
	return s.handleExecAction(ctx, req, action)
}

func (s *actionService) handleExecAction(ctx context.Context, req *proto.ActionRequest, action *proto.Action) ([]byte, error) {
	if req.NonBlocking == nil || !*req.NonBlocking {
		return runCommand(ctx, action.Exec, req.Parameters, req.TimeoutSeconds)
	}
	return s.handleExecActionNonBlocking(ctx, req, action)
}

func (s *actionService) handleExecActionNonBlocking(ctx context.Context, req *proto.ActionRequest, action *proto.Action) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	running, ok := s.runningActions[req.Action]
	if !ok {
		resultChan, err := runCommandNonBlocking(ctx, action.Exec, req.Parameters, req.TimeoutSeconds)
		if err != nil {
			return nil, err
		}
		running = &runningAction{
			resultChan: resultChan,
		}
		s.runningActions[req.Action] = running
	}
	result := gather(running.resultChan)
	if result == nil {
		return nil, proto.ErrInProgress
	}
	delete(s.runningActions, req.Action)
	if (*result).err != nil {
		return nil, (*result).err
	}
	return (*result).stdout.Bytes(), nil
}
