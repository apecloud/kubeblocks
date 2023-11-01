/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package grpcserver

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type GRPCServer struct {
	logger             logr.Logger
	checkRoleOperation operations.Operation
}

var (
	grpcPort int
)

const (
	DefaultGRPCPort = 50001
)

func init() {
	flag.IntVar(&grpcPort, "grpcport", DefaultGRPCPort, "lorry grpc default port")
}

func (s *GRPCServer) Check(ctx context.Context, in *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	resp, err := s.checkRoleOperation.Do(ctx, nil)

	var status = health.HealthCheckResponse_SERVING
	if err != nil {
		status = health.HealthCheckResponse_NOT_SERVING
		if _, ok := err.(util.ProbeError); !ok {
			s.logger.Error(err, "role probe failed")
			return &health.HealthCheckResponse{Status: status}, err
		} else {
			body, _ := json.Marshal(resp.Data)
			s.logger.Info("Role changed event detected", "role", string(body))
			return &health.HealthCheckResponse{Status: status}, errors.New(string(body))
		}
	}

	s.logger.Info("No event detected", "response", resp)
	return &health.HealthCheckResponse{Status: status}, nil
}

func (s *GRPCServer) Watch(in *health.HealthCheckRequest, _ health.Health_WatchServer) error {
	// didn't implement the `watch` function
	return status.Error(codes.Unimplemented, "unimplemented")
}

func (s *GRPCServer) StartNonBlocking() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		return errors.Wrap(err, "grpc server listen failed")
	}
	server := grpc.NewServer()
	health.RegisterHealthServer(server, s)

	go func() {
		err = server.Serve(listen)
		if err != nil {
			s.logger.Error(err, "grpcserver serve failed")
		}
	}()
	return nil
}

func NewGRPCServer() (*GRPCServer, error) {
	checkRoleOperation, ok := operations.Operations()[strings.ToLower(string(util.CheckRoleOperation))]
	if !ok {
		return nil, errors.New("check role operation not found")
	}
	err := checkRoleOperation.Init(context.Background())
	if err != nil {
		return nil, err
	}

	return &GRPCServer{
		logger:             ctrl.Log.WithName("grpc"),
		checkRoleOperation: checkRoleOperation,
	}, nil
}
