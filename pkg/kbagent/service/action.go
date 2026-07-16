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
	"regexp"
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
		if err := validateActionResultPolicy(&action); err != nil {
			return nil, errors.Wrapf(proto.ErrBadRequest, "invalid result policy for action %q: %v", action.Name, err)
		}
		sa.actions[action.Name] = &actions[i]
	}
	logger.Info(fmt.Sprintf("create service %s", sa.Kind()), "actions", strings.Join(maps.Keys(sa.actions), ","))
	return sa, nil
}

const (
	maxActionFailureCodes = 32
)

var actionResultCodePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

func validateActionResultPolicy(action *proto.Action) error {
	if action == nil || action.ResultPolicy == nil {
		return nil
	}
	if action.Exec == nil {
		return fmt.Errorf("result policy is only supported for exec actions")
	}
	if len(action.ResultPolicy.FailureCodes) > maxActionFailureCodes {
		return fmt.Errorf("result policy has more than %d failure codes", maxActionFailureCodes)
	}
	seenExitCodes := make(map[int32]string)
	for _, mapping := range action.ResultPolicy.FailureCodes {
		if !actionResultCodePattern.MatchString(mapping.Code) || len(mapping.Code) > 63 {
			return fmt.Errorf("result code %q is invalid", mapping.Code)
		}
		if mapping.ExecExitCode < 1 || mapping.ExecExitCode > 255 {
			return fmt.Errorf("exec exit code %d for result code %q is outside [1, 255]", mapping.ExecExitCode, mapping.Code)
		}
		if existing, ok := seenExitCodes[mapping.ExecExitCode]; ok {
			return fmt.Errorf("exec exit code %d is mapped to both %q and %q", mapping.ExecExitCode, existing, mapping.Code)
		}
		seenExitCodes[mapping.ExecExitCode] = mapping.Code
	}
	return nil
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
		rsp.Code = proto.ActionResultCode(err)
		rsp.Retryable = proto.ActionResultRetryable(err)
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
	retryPolicy := resolveRetryPolicy(action.RetryPolicy, req.RetryPolicy)
	if req.NonBlocking == nil || !*req.NonBlocking {
		return callActionWithRetry(ctx, action, req.Parameters, req.Arguments, timeout, retryPolicy)
	}
	return s.handleRequestNonBlocking(ctx, req, action, timeout, retryPolicy)
}

func (s *actionService) handleRequestNonBlocking(ctx context.Context, req *proto.ActionRequest, action *proto.Action, timeout *int32, retryPolicy *proto.RetryPolicy) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	running, ok := s.runningActions[req.Action]
	if !ok {
		resultChan, err := nonBlockingCallActionWithRetry(ctx, action, req.Parameters, req.Arguments, timeout, retryPolicy)
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

func resolveRetryPolicy(actionRetryPolicy *proto.RetryPolicy, requestRetryPolicy *proto.RetryPolicy) *proto.RetryPolicy {
	if requestRetryPolicy != nil {
		return requestRetryPolicy
	}
	return actionRetryPolicy
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
	if len(arguments) > 0 && action.Exec == nil {
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
	if err == nil || retryPolicy == nil || retryPolicy.MaxRetries <= 0 || explicitlyNonRetryable(err) {
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
		if err == nil || explicitlyNonRetryable(err) {
			return output, err
		}
	}
	return output, err
}

func explicitlyNonRetryable(err error) bool {
	retryable := proto.ActionResultRetryable(err)
	return retryable != nil && !*retryable
}
