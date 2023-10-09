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

package grpc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	. "github.com/apecloud/kubeblocks/lorry/binding"
	probe2 "github.com/apecloud/kubeblocks/lorry/middleware/probe"
)

type GRPCServer struct {
	character string
	logger    logr.Logger
	router    func(ctx context.Context) (*ProbeResponse, error)
}

func (s *GRPCServer) Check(ctx context.Context, in *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	s.logger.Info("role probe request", "type", s.character)
	resp, err := s.router(ctx)

	if err != nil {
		s.logger.Error(err, "role probe failed")
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, err
	}

	code, ok := resp.Metadata[StatusCode]
	if ok && code == OperationFailedHTTPCode {
		s.logger.Info("Detect event changed", "role", string(resp.Data))
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, errors.New(string(resp.Data))
	}

	meta, _ := json.Marshal(resp.Metadata)

	s.logger.Info("Nothing happened", "meta", string(meta), "data", string(resp.Data))
	return &health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING}, nil
}

func (s *GRPCServer) Watch(in *health.HealthCheckRequest, _ health.Health_WatchServer) error {
	// didn't implement the `watch` function
	return status.Error(codes.Unimplemented, "unimplemented")
}

func NewGRPCServer() *GRPCServer {
	characterType := viper.GetString(constant.KBEnvCharacterType)
	return &GRPCServer{
		character: characterType,
		logger:    ctrl.Log.WithName("grpc"),
		router:    probe2.GetGrpcRouter(characterType),
	}
}
