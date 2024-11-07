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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	dataDump = "dataDump"
	dataLoad = "dataLoad"
)

type dataLoadTask struct {
	logger        logr.Logger
	actionService *actionService
}

func (s *dataLoadTask) run(ctx context.Context, task *proto.DataLoadTask) error {
	action, ok := s.actionService.actions[dataLoad]
	if !ok {
		return fmt.Errorf("%s is not supported", dataLoad)
	}

	conn, err := s.handshake(ctx, task)
	if err != nil {
		return err
	}

	return s.streamingLoad(ctx, task, action, conn)
}

// TODO: implement status to query the load progress
// func (s *dataLoadTask) status(ctx context.Context) error {
//	return nil
// }

func (s *dataLoadTask) handshake(ctx context.Context, task *proto.DataLoadTask) (net.Conn, error) {
	conn, err := s.connectToRemote(ctx, task)
	if err != nil {
		return nil, err
	}

	req := proto.ActionRequest{
		Action:         dataDump,
		Parameters:     task.Parameters,
		TimeoutSeconds: task.TimeoutSeconds,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if len(data) > maxStreamingHandshakePacketSize {
		return nil, fmt.Errorf("handshake packet size is too large: %d", len(data))
	}

	ret, err := conn.Write(data)
	if err != nil {
		return nil, err
	}
	if ret != len(data) {
		return nil, fmt.Errorf("write streaming handshake request to remote error")
	}

	return conn, nil
}

func (s *dataLoadTask) connectToRemote(ctx context.Context, task *proto.DataLoadTask) (net.Conn, error) {
	if len(task.Remote) == 0 {
		return nil, fmt.Errorf("remote server is required")
	}
	if task.Port == nil || *task.Port == 0 {
		return nil, fmt.Errorf("remote port is required")
	}
	// TODO: connect timeout
	dialer := &net.Dialer{}
	return dialer.Dial("tcp", fmt.Sprintf("%s:%d", task.Remote, *task.Port))
}

func (s *dataLoadTask) streamingLoad(ctx context.Context, task *proto.DataLoadTask, action *proto.Action, conn net.Conn) error {
	errChan, err1 := runCommandX(ctx, action.Exec, task.Parameters, task.TimeoutSeconds, conn, nil, nil)
	if err1 != nil {
		return err1
	}
	err2, ok := <-errChan
	if !ok {
		err2 = errors.New("runtime error: error chan closed unexpectedly")
	}
	return err2
}
