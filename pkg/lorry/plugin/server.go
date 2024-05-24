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

package plugin

import (
	context "context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

// NonBlockingGRPCServer Defines Non blocking GRPC server interfaces
type NonBlockingGRPCServer interface {
	// Start services at the endpoint
	Start(endpoint string, servicePlugin ServicePluginServer)
	// Waits for the service to stop
	Wait()
	// Stops the service gracefully
	Stop()
	// Stops the service forcefully
	ForceStop()
}

func NewNonBlockingGRPCServer(logger logr.Logger) NonBlockingGRPCServer {
	return &nonBlockingGRPCServer{
		logger: logger,
	}
}

// NonBlocking server
type nonBlockingGRPCServer struct {
	wg     sync.WaitGroup
	server *grpc.Server
	logger logr.Logger
}

func (s *nonBlockingGRPCServer) Start(endpoint string, servicePlugin ServicePluginServer) {

	s.wg.Add(1)

	go s.serve(endpoint, servicePlugin)
}

func (s *nonBlockingGRPCServer) Wait() {
	s.wg.Wait()
}

func (s *nonBlockingGRPCServer) Stop() {
	s.server.GracefulStop()
}

func (s *nonBlockingGRPCServer) ForceStop() {
	s.server.Stop()
}

func (s *nonBlockingGRPCServer) serve(endpoint string, servicePlugin ServicePluginServer) {
	proto, addr, err := ParseEndpoint(endpoint)
	if err != nil {
		panic(err.Error())
	}

	if proto == "unix" {
		addr = "/" + addr
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			panic(fmt.Sprintf("Failed to remove %s, error: %s", addr, err.Error()))
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		panic(fmt.Sprintf("Failed to listen: %v", err))
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.logGRPC),
	}
	server := grpc.NewServer(opts...)
	s.server = server

	RegisterServicePluginServer(server, servicePlugin)

	s.logger.Info("Listening for connections on address", "addr", listener.Addr())

	err = server.Serve(listener)
	if err != nil {
		panic(err.Error())
	}

}

func (s *nonBlockingGRPCServer) logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	s.logger.V(1).Info("GRPC call", "fullMethod", info.FullMethod)
	s.logger.V(2).Info("GRPC call", "request", StripSecrets(req))
	resp, err := handler(ctx, req)
	if err != nil {
		s.logger.Info("GRPC call failed", "error", err.Error())
	} else {
		s.logger.V(2).Info("GRPC call", "response", StripSecrets(resp))
	}
	return resp, err
}
