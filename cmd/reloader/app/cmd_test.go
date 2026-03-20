/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package app

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"

	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
)

type fakeConfigHandler struct{}

func (f *fakeConfigHandler) OnlineUpdate(context.Context, string, map[string]string) error {
	return nil
}

func (f *fakeConfigHandler) VolumeHandle(context.Context, fsnotify.Event) error {
	return nil
}

func (f *fakeConfigHandler) MountPoint() []string {
	return nil
}

func TestStartGRPCServiceEnablesReflection(t *testing.T) {
	t.Helper()

	logger = zap.NewNop().Sugar()
	cfgcore.SetLogger(zap.NewNop())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := &VolumeWatcherOpts{
		ServiceOpt: ReconfigureServiceOptions{
			GrpcPort: port,
			PodIP:    localhostAddress,
		},
		LogLevel: "info",
	}
	if err := startGRPCService(opts, ctx, &fakeConfigHandler{}); err != nil {
		t.Fatalf("failed to start grpc service: %v", err)
	}

	conn, err := grpc.DialContext(ctx, net.JoinHostPort(localhostAddress, strconv.Itoa(port)), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("failed to dial grpc service: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := grpc_reflection_v1.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatalf("failed to create reflection stream: %v", err)
	}

	if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
	}); err != nil {
		t.Fatalf("failed to send reflection request: %v", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("failed to receive reflection response: %v", err)
	}

	servicesResp := resp.GetListServicesResponse()
	if servicesResp == nil {
		t.Fatalf("expected list services response, got: %T", resp.MessageResponse)
	}

	serviceNames := map[string]bool{}
	for _, svc := range servicesResp.Service {
		serviceNames[svc.Name] = true
	}
	if !serviceNames["proto.Reconfigure"] {
		t.Fatalf("expected proto.Reconfigure service in reflection response, got: %v", serviceNames)
	}
}
