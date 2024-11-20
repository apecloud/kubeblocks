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
	"time"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	newReplicaDataDump              = "dataDump"
	newReplicaDataLoad              = "dataLoad"
	newReplicaConnectTimeoutSeconds = 10

	targetPodNameEnv = "KB_TARGET_POD_NAME"
)

type newReplicaTask struct {
	logger        logr.Logger
	actionService *actionService
	task          *proto.NewReplicaTask
}

var _ task = &newReplicaTask{}

func (s *newReplicaTask) run(ctx context.Context) (chan error, error) {
	action, ok := s.actionService.actions[newReplicaDataLoad]
	if !ok {
		return nil, fmt.Errorf("%s is not supported", newReplicaDataLoad)
	}

	conn, err := s.handshake(ctx)
	if err != nil {
		return nil, err
	}

	return runCommandX(ctx, action.Exec, s.task.Parameters, s.task.TimeoutSeconds, conn, nil, nil)
}

func (s *newReplicaTask) status(ctx context.Context, event *proto.TaskEvent) {
	// TODO: query the progress
	event.Code = 0
	event.Output = nil
	event.Message = ""
}

func (s *newReplicaTask) handshake(ctx context.Context) (net.Conn, error) {
	conn, err := s.connectToRemote(ctx)
	if err != nil {
		return nil, err
	}

	// reuse the action request as the handshake packet, define a new one when needed
	req := proto.ActionRequest{
		Action:         newReplicaDataDump,
		Parameters:     s.task.Parameters,
		TimeoutSeconds: s.task.TimeoutSeconds,
	}
	if req.Parameters == nil {
		req.Parameters = make(map[string]string)
	}
	req.Parameters[targetPodNameEnv] = util.PodName()
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

func (s *newReplicaTask) connectToRemote(ctx context.Context) (net.Conn, error) {
	if len(s.task.Remote) == 0 {
		return nil, fmt.Errorf("remote server is required")
	}
	if s.task.Port == 0 {
		return nil, fmt.Errorf("remote port is required")
	}
	dialer := &net.Dialer{
		Timeout: newReplicaConnectTimeoutSeconds * time.Second,
	}
	return dialer.Dial("tcp", fmt.Sprintf("%s:%d", s.task.Remote, s.task.Port))
}
