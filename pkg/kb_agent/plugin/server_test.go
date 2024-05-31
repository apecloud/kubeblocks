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
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type mockServicePluginServer struct {
	UnimplementedServicePluginServer
}

func (s *mockServicePluginServer) GetPluginInfo(ctx context.Context, in *GetPluginInfoRequest) (*GetPluginInfoResponse, error) {
	return &GetPluginInfoResponse{
		Name:        "mock",
		Version:     "1.0.0",
		ServiceType: "mockDB",
	}, nil
}

func TestNewNonBlockingGRPCServer(t *testing.T) {
	logger := logr.Discard()
	server := NewNonBlockingGRPCServer(logger)

	assert.NotNil(t, server)
}

func TestServe(t *testing.T) {
	logger := logr.Discard()

	// Create a temporary directory for the Unix socket
	tmpDir, err := os.MkdirTemp("", "test-socket")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate a random Unix socket path
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create a new ServicePluginServer
	servicePlugin := &mockServicePluginServer{}

	// Create a new nonBlockingGRPCServer
	server := &nonBlockingGRPCServer{
		logger: logger,
	}

	// Start serving on the Unix socket
	go server.serve("unix:/"+socketPath, servicePlugin)

	// Wait for the server to start
	time.Sleep(time.Millisecond * 100)

	// Create a gRPC client connection to the Unix socket
	// Define a custom dialer function
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return net.DialTimeout("unix", addr, 100*time.Second)
	}
	conn, err := grpc.Dial(socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Create a new ServicePluginClient
	client := NewServicePluginClient(conn)

	// Call a gRPC method on the server
	_, err = client.GetPluginInfo(context.Background(), &GetPluginInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Stop the server
	server.Stop()

	// Wait for the server to stop
	server.Wait()
}
