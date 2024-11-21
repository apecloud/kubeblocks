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

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

type streamingServer struct {
	logger   logr.Logger
	config   Config
	service  service.Service
	listener net.Listener
}

var _ Server = &streamingServer{}

// StartNonBlocking starts a new server in a goroutine.
func (s *streamingServer) StartNonBlocking() error {
	s.logger.Info("starting the streaming server")

	if s.service == nil {
		s.logger.Info("has no streaming service defined")
		return nil
	}

	var err1 error
	s.listener, err1 = net.Listen("tcp", fmt.Sprintf("%s:%v", s.config.Address, s.config.StreamingPort))
	if err1 != nil {
		s.logger.Error(err1, "listen address", s.config.Address, "port", s.config.StreamingPort)
		return err1
	}

	go func() {
		var tempErr error
		for {
			conn, err2 := s.listener.Accept()
			if err2 != nil {
				var netErr net.Error
				if errors.As(errors.Unwrap(err2), &netErr) && netErr.Temporary() {
					if tempErr == nil || !errors.Is(err2, tempErr) {
						s.logger.Error(err2, "accept new connection error")
					}
					tempErr = err2
					continue // TODO: back-off
				}
				panic(fmt.Sprintf("accept new connection error: %v", err2))
			}
			tempErr = nil
			go s.handleConn(conn)
		}
	}()

	return nil
}

func (s *streamingServer) Close() error {
	err := s.close(s.listener)
	if err != nil {
		s.logger.Error(err, "failed to close the streaming server")
	}
	return err
}

func (s *streamingServer) close(c io.Closer) error {
	if c != nil {
		return c.Close()
	}
	return nil
}

func (s *streamingServer) silentClose(c io.Closer) {
	_ = s.close(c)
}

func (s *streamingServer) handleConn(conn net.Conn) {
	defer s.silentClose(conn)

	logger := s.logger.WithValues("remote", conn.RemoteAddr())
	logger.Info("accepted a new streaming connection")

	now := time.Now()
	err := s.service.HandleConn(context.Background(), conn)
	if err != nil {
		logger.Error(err, "handle streaming connection error")
	} else {
		logger.Info("handle streaming connection done", "elapsed", time.Since(now))
	}
}
