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

	viper "github.com/apecloud/kubeblocks/pkg/viperx"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

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
	stdoutChan chan []byte
	stderrChan chan []byte
	errChan    chan error
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
	return s.encode(s.handleRequest(ctx, req)), nil
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
	if action.Name == "find" {
		return s.handleFindAction(ctx, req, action)
	}
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
		return nil, proto.ErrInProgress
	}
	delete(s.runningActions, req.Action)
	if *err != nil {
		return nil, *err
	}
	return *gather(running.stdoutChan), nil
}

func (s *actionService) handleFindAction(ctx context.Context, req *proto.ActionRequest, action *proto.Action) ([]byte, error) {
	eyeEnv := make(map[string]string)
	eyeEnvStr := viper.GetString("KB_AGENT_FINDER")
	err := json.Unmarshal([]byte(eyeEnvStr), &eyeEnv)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal eye env error")
	}
	parameters := req.Parameters
	if len(parameters) == 0 {
		return nil, errors.Wrap(proto.ErrBadRequest, "missing parameters")
	}
	res, ok := eyeEnv[parameters["actionName"]]
	if !ok {
		return nil, errors.Wrapf(proto.ErrNotDefined, "%s is not found", parameters["actionName"])
	}
	return []byte(res), nil
}
