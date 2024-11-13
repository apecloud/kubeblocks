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
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	maxStreamingHandshakePacketSize = 4096
)

func newStreamingService(logger logr.Logger, actionService *actionService, streamingActions []string) (*streamingService, error) {
	ss := &streamingService{
		logger:           logger,
		streamingActions: make(map[string]*proto.Action),
	}
	for _, a := range streamingActions {
		if _, ok := actionService.actions[a]; !ok {
			return nil, fmt.Errorf("action %s has not defined", a)
		}
		ss.streamingActions[a] = actionService.actions[a]
	}
	logger.Info(fmt.Sprintf("create service %s", ss.Kind()),
		"actions", strings.Join(maps.Keys(ss.streamingActions), ","))
	return ss, nil
}

type streamingService struct {
	logger           logr.Logger
	streamingActions map[string]*proto.Action
}

var _ Service = &streamingService{}

func (s *streamingService) Kind() string {
	return proto.ServiceStreaming.Kind
}

func (s *streamingService) URI() string {
	return proto.ServiceStreaming.URI
}

func (s *streamingService) Start() error {
	return nil
}

func (s *streamingService) HandleConn(ctx context.Context, conn net.Conn) error {
	req, err := s.handshake(ctx, conn)
	if err != nil {
		return err
	}

	action, ok := s.streamingActions[req.Action]
	if !ok {
		return fmt.Errorf("%s is not supported", req.Action)
	}

	return s.streaming(ctx, conn, action, req)
}

func (s *streamingService) HandleRequest(ctx context.Context, payload []byte) ([]byte, error) {
	return nil, errors.Wrapf(proto.ErrNotImplemented, "service %s does not support request handling", s.Kind())
}

func (s *streamingService) handshake(ctx context.Context, conn net.Conn) (*proto.ActionRequest, error) {
	req := &proto.ActionRequest{}
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(req); err != nil {
		return nil, errors.Wrapf(proto.ErrBadRequest, "read and unmarshal action request error: %s", err.Error())
	}
	return req, nil
}

func (s *streamingService) streaming(ctx context.Context, conn net.Conn, action *proto.Action, req *proto.ActionRequest) error {
	errChan, err1 := runCommandX(ctx, action.Exec, req.Parameters, req.TimeoutSeconds, nil, conn, nil)
	if err1 != nil {
		return err1
	}
	err2, ok := <-errChan
	if !ok {
		err2 = errors.New("runtime error: error chan closed unexpectedly")
	}
	return err2
}
