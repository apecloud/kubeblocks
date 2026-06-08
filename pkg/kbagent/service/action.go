/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

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
	resultChan chan *asyncResult
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

func (s *actionService) HandleConn(ctx context.Context, conn net.Conn) error {
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
	s.logger.Info("Action Executed", "action", req.Action, "result", result)
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
	action, ok := s.actions[req.Action]
	if !ok {
		return nil, errors.Wrapf(proto.ErrNotDefined, "%s is not defined", req.Action)
	}
	if action.Exec == nil && action.HTTP == nil && action.GRPC == nil {
		return nil, errors.Wrapf(proto.ErrBadRequest, "%s is invalid", req.Action)
	}
	// HACK: pre-check for the reconfigure action
	if err := checkReconfigure(ctx, req); err != nil {
		return nil, err
	}
	timeout := resolveTimeout(&action.TimeoutSeconds, req.TimeoutSeconds)
	if req.NonBlocking == nil || !*req.NonBlocking {
		return callActionWithRetry(ctx, action, req.Parameters, req.Arguments, timeout, req.RetryPolicy)
	}
	return s.handleRequestNonBlocking(ctx, req, action, timeout)
}

func (s *actionService) handleRequestNonBlocking(ctx context.Context, req *proto.ActionRequest, action *proto.Action, timeout *int32) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	running, ok := s.runningActions[req.Action]
	if !ok {
		resultChan, err := nonBlockingCallActionWithRetry(ctx, action, req.Parameters, req.Arguments, timeout, req.RetryPolicy)
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

func resolveTimeout(actionTimeout *int32, requestTimeout *int32) *int32 {
	if requestTimeout != nil {
		return requestTimeout
	}
	return actionTimeout
}

func callActionWithRetry(ctx context.Context, action *proto.Action, parameters map[string]string, arguments [][]string, timeout *int32, retryPolicy *proto.RetryPolicy) ([]byte, error) {
	if len(arguments) == 0 {
		return callActionWithRetryOnce(ctx, action, parameters, nil, timeout, retryPolicy)
	}
	if action.Exec == nil {
		return nil, errors.Wrapf(proto.ErrBadRequest, "runtime arguments are only supported for exec actions")
	}
	output := bytes.NewBuffer(nil)
	for _, args := range arguments {
		out, err := callActionWithRetryOnce(ctx, action, parameters, args, timeout, retryPolicy)
		if err != nil {
			return output.Bytes(), err
		}
		if out != nil {
			output.Write(out)
		}
	}
	return output.Bytes(), nil
}

func nonBlockingCallActionWithRetry(ctx context.Context, action *proto.Action, parameters map[string]string, arguments [][]string, timeout *int32, retryPolicy *proto.RetryPolicy) (chan *asyncResult, error) {
	if len(arguments) == 0 {
		return nonBlockingCallAction(ctx, action, parameters, nil, timeout)
	}
	if action.Exec == nil {
		return nil, errors.Wrapf(proto.ErrBadRequest, "runtime arguments are only supported for exec actions")
	}
	resultChan := make(chan *asyncResult, 1)
	go func() {
		stdout, err := callActionWithRetry(ctx, action, parameters, arguments, timeout, retryPolicy)
		resultChan <- &asyncResult{
			err:    err,
			stdout: bytes.NewBuffer(stdout),
			stderr: bytes.NewBuffer(nil),
		}
	}()
	return resultChan, nil
}

func callActionWithRetryOnce(ctx context.Context, action *proto.Action, parameters map[string]string, arguments []string, timeout *int32, retryPolicy *proto.RetryPolicy) ([]byte, error) {
	output, err := blockingCallAction(ctx, action, parameters, arguments, timeout)
	if err == nil || retryPolicy == nil || retryPolicy.MaxRetries <= 0 {
		return output, err
	}

	interval := retryPolicy.RetryInterval
	for i := 0; i < retryPolicy.MaxRetries; i++ {
		if interval > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(interval):
			}
		}
		output, err = blockingCallAction(ctx, action, parameters, arguments, timeout)
		if err == nil {
			return output, nil
		}
	}
	return output, err
}
