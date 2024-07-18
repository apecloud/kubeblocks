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

package handlers

import (
	"context"
	"flag"
	"log"
	"net"
	"testing"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/plugin"
	"google.golang.org/grpc"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewGRPCHandler(t *testing.T) {
	handler, err := NewGRPCHandler(map[string]string{
		"host": "localhost",
		"port": "50051",
	})
	assert.NotNil(t, handler)
	assert.Nil(t, err)
}

func TestGRPCHandlerDo(t *testing.T) {
	ctx := context.Background()
	go func() {
		flag.Parse()
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer()
		plugin.RegisterGrpcServer(s, &MockGrpcServer{})
		log.Printf("server listening at %v", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	setting := util.HandlerSpec{
		GPRC: map[string]string{
			"host": "localhost",
			"port": "50051",
		},
	}
	t.Run("grpc handler is nil", func(t *testing.T) {
		setting := util.HandlerSpec{
			GPRC: nil,
		}
		handler := &GRPCHandler{}
		do, err := handler.Do(ctx, setting, nil)
		assert.Nil(t, do)
		assert.NotNil(t, err)
		assert.Error(t, err, errors.New("grpc setting is nil"))
	})

	t.Run("grpc args is nil", func(t *testing.T) {
		handler, err := NewGRPCHandler(setting.GPRC)
		assert.Nil(t, err)
		do, err := handler.Do(ctx, setting, nil)
		assert.Nil(t, do)
		assert.NotNil(t, err)
		assert.Error(t, err, errors.New("args is nil"))
	})

	t.Run("grpc do success", func(t *testing.T) {
		handler, err := NewGRPCHandler(setting.GPRC)
		assert.Nil(t, err)
		args := map[string]interface{}{
			"methodName": "test",
			"username":   "admin",
			"password":   "admin",
		}
		result, err := handler.Do(ctx, setting, args)
		assert.NotNil(t, result)
		assert.Nil(t, err)
		assert.Equal(t, "methodName : test", result.Message)
	})

	t.Run("grpc do not implemented", func(t *testing.T) {
		handler, err := NewGRPCHandler(setting.GPRC)
		assert.Nil(t, err)
		args := map[string]interface{}{
			"methodName": "notImplemented",
			"username":   "admin",
		}
		result, err := handler.Do(ctx, setting, args)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Error(t, err, errors.New("not implemented"))
	})
}

type MockGrpcServer struct {
	*plugin.UnimplementedGrpcServer
}

func (s *MockGrpcServer) Call(ctx context.Context, in *plugin.Request) (*plugin.Response, error) {
	methodName := in.MethodName
	parameters := in.Parameters
	m, _ := util.ParseArgs(parameters)
	for k, v := range m {
		log.Printf("key: %s, value: %s", k, v)
	}
	switch methodName {
	case "test":
		return &plugin.Response{Message: "methodName : " + methodName}, nil
	default:
		return nil, errors.New("not implemented")
	}
}
