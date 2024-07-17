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
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	actionVersion = "v1.0"
	actionURI     = "action"
)

func newActionService(logger logr.Logger, actions []proto.Action) (*actionService, error) {
	sa := &actionService{logger: logger}
	for i, action := range actions {
		sa.actions[action.Name] = &actions[i]
	}
	return sa, nil
}

type actionService struct {
	logger  logr.Logger
	actions map[string]*proto.Action
}

var _ Service = &actionService{}

func (s *actionService) Kind() string {
	return "Action"
}

func (s *actionService) Version() string {
	return actionVersion
}

func (s *actionService) URI() string {
	return actionURI
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

func (s *actionService) Call(ctx context.Context, i interface{}) ([]byte, error) {
	req := i.(*proto.ActionRequest)
	if _, ok := s.actions[req.Action]; !ok {
		return nil, fmt.Errorf("%s is not supported", req.Action)
	}
	return s.callAction(ctx, req)
}

func (s *actionService) callAction(ctx context.Context, req *proto.ActionRequest) ([]byte, error) {
	action := s.actions[req.Action]
	if action.Exec != nil {
		return s.callExecAction(ctx, req, action)
	}
	return nil, fmt.Errorf("only exec action is supported: %s", req.Action)
}

func (s *actionService) callExecAction(ctx context.Context, req *proto.ActionRequest, action *proto.Action) ([]byte, error) {
	// TODO: non-blocking & timeout
	return execute(ctx, action.Exec.Commands, action.Exec.Args, req.Parameters, req.TimeoutSeconds)
}

func execute(ctx context.Context, commands []string, args []string, parameters map[string]string, timeout *int32) ([]byte, error) {
	if timeout != nil && *timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
		defer cancel()
		ctx = timeoutCtx
	}

	mergedArgs := make([]string, 0)
	if len(commands) > 1 {
		mergedArgs = append(mergedArgs, commands[1:]...)
	}
	mergedArgs = append(mergedArgs, args...)

	cmd := exec.CommandContext(ctx, commands[0], mergedArgs...)
	cmd.Env = util.EnvM2L(parameters)
	bytes, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		err = errors.New(string(exitErr.Stderr))
	}
	return bytes, err
}
