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

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"

	"google.golang.org/grpc/codes"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const (
	DefaultPort     = "7373"
	DefaultQueryUrl = "role"
)

type GrpcServer struct {
	RoleAgent
}

func (s *GrpcServer) Check(ctx context.Context, in *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	opsRes, shouldNotify := s.CheckRole(ctx)
	if _, exist := opsRes["event"]; !exist || len(opsRes) == 0 {
		log.Printf("CheckRole returns empty\n")
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, errors.New("CheckRole returns empty")
	}

	buf, err := json.Marshal(opsRes)
	if err != nil {
		log.Printf("parse opsresult error: %v\n", err)
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, err
	}

	roleInfo := string(buf)
	if shouldNotify {
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, errors.New(roleInfo)
	} else {
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING}, nil
	}
}

func (s *GrpcServer) Watch(in *health.HealthCheckRequest, _ health.Health_WatchServer) error {
	// didn't implement the `watch` function
	return status.Error(codes.Unimplemented, "unimplemented")
}

func (s *GrpcServer) Init() error {
	return s.RoleAgent.Init()
}

func NewGrpcServer(path string) *GrpcServer {
	return &GrpcServer{RoleAgent: *NewRoleAgent(os.Stdin, "", path)}
}
