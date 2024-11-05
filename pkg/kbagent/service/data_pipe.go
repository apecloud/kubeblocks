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
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	dataPipeReadAction  = "dataLoad"
	dataPipeWriteAction = "dataDump"
	defaultPipePort     = 8088
)

func newDataPipeService(logger logr.Logger, actionService *actionService) (Service, error) {
	sp := &dataPipeService{
		logger:        logger,
		actionService: actionService,
	}
	return sp, nil
}

type dataPipeService struct {
	logger        logr.Logger
	actionService *actionService
	listener      net.Listener
	dataPipe      *dataPipe
}

type dataPipe struct {
	conn    net.Conn
	errChan chan error
}

var _ Service = &dataPipeService{}

func (s *dataPipeService) Kind() string {
	return proto.ServiceProbe.Kind
}

func (s *dataPipeService) URI() string {
	return proto.ServiceProbe.URI
}

func (s *dataPipeService) Start() error {
	return s.listen()
}

func (s *dataPipeService) HandleRequest(ctx context.Context, payload []byte) ([]byte, error) {
	req, err := s.decode(payload)
	if err != nil {
		return s.encode(nil, err), nil
	}
	return s.encode(s.handleRequest(ctx, req)), nil
}

func (s *dataPipeService) decode(payload []byte) (*proto.DataPipeRequest, error) {
	req := &proto.DataPipeRequest{}
	if err := json.Unmarshal(payload, req); err != nil {
		return nil, errors.Wrapf(proto.ErrBadRequest, "unmarshal data-pipe request error: %s", err.Error())
	}
	return req, nil
}

func (s *dataPipeService) encode(out []byte, err error) []byte {
	rsp := &proto.DataPipeResponse{}
	if err == nil {
		rsp.Output = out
	} else {
		rsp.Error = proto.Error2Type(err)
		rsp.Message = err.Error()
	}
	data, _ := json.Marshal(rsp)
	return data
}

func (s *dataPipeService) handleRequest(ctx context.Context, i interface{}) ([]byte, error) {
	req := i.(*proto.DataPipeRequest)

	actionName := dataPipeReadAction
	if req.Write {
		actionName = dataPipeWriteAction
	}

	action, ok := s.actionService.actions[actionName]
	if !ok {
		return nil, fmt.Errorf("%s is not supported", actionName)
	}
	if req.Write {
		return nil, s.handlePipeWriteRequest(ctx, req, action)
	}
	return nil, s.handlePipeReadRequest(ctx, req, action)
}

func (s *dataPipeService) handlePipeWriteRequest(ctx context.Context, req *proto.DataPipeRequest, action *proto.Action) error {
	if s.pipe != nil {
		return s.polling()
	}

	listener, _, _ := s.openDataPipe(req)

	conn, err := s.connectToPeer(req)
	if err != nil {
		return err
	}
	errChan, err := runCommandX(ctx, action.Exec, req.Parameters, req.TimeoutSeconds, nil, conn, nil)
	if err != nil {
		return err
	}
	s.dataPipe = &dataPipe{
		conn:    conn,
		errChan: errChan,
	}
	return s.polling()
}

func (s *dataPipeService) dataDump(ctx context.Context, req *proto.DataPipeRequest, action *proto.Action, conn net.Conn) error {
	errChan, err := runCommandX(ctx, action.Exec, req.Parameters, req.TimeoutSeconds, nil, conn, nil)
	if err != nil {
		return err
	}
	s.dataPipe = &dataPipe{
		conn:    conn,
		errChan: errChan,
	}
	return s.polling()
}

func (s *dataPipeService) listen() error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", defaultPipePort))
	if err != nil {
		return err
	}
	go func() {
		defer s.listener.Close()
		for {
			conn, err1 := s.listener.Accept()
			if err1 != nil {
				s.logger.Error(err1, "accept new connection error")
				continue
			}
			go s.handleConn4Dump(conn)
		}
	}()
	return nil
}

func (s *dataPipeService) handleConn4Dump(conn net.Conn) {
	defer conn.Close()

	logger := s.logger.WithValues("remote", conn.RemoteAddr())
	logger.Info("accepted new connection for dump")

	now := time.Now()

	action, ok := s.actionService.actions[dataPipeWriteAction]
	if !ok {
		logger.Info("action is not supported", "action", dataPipeWriteAction)
		return
	}
	if action.Exec == nil {
		logger.Info("action is not supported", "action", dataPipeWriteAction)
	}
	errBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
	errChan, err := runCommandX(context.Background(), action.Exec, nil, nil, nil, conn, errBuf)
	if err != nil {
		logger.Error(err, "launch dump command error")
		return
	}

	// wait for the command to finish
	execErr := <-errChan
	if execErr != nil {
		logger.Error(execErr, "execute dump command error", "stderr", errBuf.String())
	}

	logger.Info("dump finished", "elapsed", time.Since(now))
}

func (s *dataPipeService) connectTo(req *proto.DataPipeRequest) (net.Conn, error) {
	if len(req.Peer) == 0 {
		return nil, fmt.Errorf("peer is required")
	}
	if req.Port == 0 {
		req.Port = defaultPipePort
	}
	// TODO: connect timeout
	dialer := &net.Dialer{}
	return dialer.Dial("tcp", fmt.Sprintf("%s:%d", req.Peer, req.Port))
}

func (s *dataPipeService) connectToPeer(req *proto.DataPipeRequest) (net.Conn, error) {
	if len(req.Peer) == 0 {
		return nil, fmt.Errorf("peer is required")
	}
	if req.Port == 0 {
		req.Port = defaultPipePort
	}
	// TODO: connect timeout
	dialer := &net.Dialer{}
	return dialer.Dial("tcp", fmt.Sprintf("%s:%d", req.Peer, req.Port))
}

func (s *dataPipeService) polling() error {
	err := gather(s.pipe.errChan)
	if err == nil {
		return fmt.Errorf("inprogress") // TODO
	}
	s.pipe.conn.Close()
	s.pipe = nil
	return *err
}

func (s *dataPipeService) handlePipeReadRequest(ctx context.Context, req *proto.DataPipeRequest, action *proto.Action) error {
	listener, conn, err := s.listen4Read(req)
	if err != nil {
		return nil
	}
	defer listener.Close()
	defer conn.Close()

	errChan, err := runCommandX(ctx, action.Exec, req.Parameters, req.TimeoutSeconds, conn, nil)
	if err != nil {
		return err
	}

	return <-errChan
}

func (s *dataPipeService) listen4Read(req *proto.DataPipeRequest) (net.Listener, net.Conn, error) {
	if req.Port == 0 {
		req.Port = defaultPipePort
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", req.Port))
	if err != nil {
		return nil, nil, err
	}

	for {
		conn, err1 := listener.Accept()
		if err1 != nil {
			s.logger.Error(err1, "accept new connection error")
			continue
		}

		s.logger.Info("accepted new connection from", conn.RemoteAddr())

		return listener, conn, nil
	}
}

func (s *dataPipeService) openDataPipe(req *proto.DataPipeRequest) (net.Listener, net.Conn, error) {
	if req.Port == 0 {
		req.Port = defaultPipePort
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", req.Port))
	if err != nil {
		return nil, nil, err
	}

	for {
		conn, err1 := listener.Accept()
		if err1 != nil {
			s.logger.Error(err1, "accept new connection error")
			continue
		}

		s.logger.Info("accepted new connection from", conn.RemoteAddr())

		return listener, conn, nil
	}
}
