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

type mockDBPluginServer struct {
	UnimplementedDBPluginServer
}

func (s *mockDBPluginServer) GetPluginInfo(ctx context.Context, in *GetPluginInfoRequest) (*GetPluginInfoResponse, error) {
	return &GetPluginInfoResponse{
		Name:    "mock",
		Version: "1.0.0",
		DbType:  "mockDB",
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

	// Create a new DBPluginServer
	dbPlugin := &mockDBPluginServer{}

	// Create a new nonBlockingGRPCServer
	server := &nonBlockingGRPCServer{
		logger: logger,
	}

	// Start serving on the Unix socket
	go server.serve("unix:/"+socketPath, dbPlugin)

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

	// Create a new DBPluginClient
	client := NewDBPluginClient(conn)

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
