/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"errors"
	"net"
	"testing"
	"time"

	"k8s.io/klog/v2/ktesting"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type errorCloser struct {
	err error
}

func (c errorCloser) Close() error {
	return c.err
}

func TestNewStreamingServer(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	svc := &serverFakeService{kind: proto.ServiceStreaming.Kind, uri: proto.ServiceStreaming.URI}
	srv := NewStreamingServer(logger, Config{StreamingPort: 3502}, svc)
	if srv == nil {
		t.Fatalf("NewStreamingServer() returned nil")
	}
	if _, ok := srv.(*streamingServer); !ok {
		t.Fatalf("NewStreamingServer() returned %T, want *streamingServer", srv)
	}
}

func TestStreamingServerStartNonBlockingStableBranches(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	srv := &streamingServer{logger: logger}
	if err := srv.StartNonBlocking(); err != nil {
		t.Fatalf("StartNonBlocking without service error = %v", err)
	}

	srv = &streamingServer{
		logger:  logger,
		config:  Config{Address: "256.256.256.256", StreamingPort: 3502},
		service: &serverFakeService{kind: proto.ServiceStreaming.Kind, uri: proto.ServiceStreaming.URI},
	}
	if err := srv.StartNonBlocking(); err == nil {
		t.Fatalf("expected listen error")
	}
}

func TestStreamingServerStartNonBlockingAcceptsConnection(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	called := make(chan net.Conn, 1)
	srv := &streamingServer{
		logger: logger,
		config: Config{Address: "127.0.0.1", StreamingPort: 0},
		service: &serverFakeService{
			kind:       proto.ServiceStreaming.Kind,
			uri:        proto.ServiceStreaming.URI,
			connCalled: called,
		},
	}
	if err := srv.StartNonBlocking(); err != nil {
		t.Fatalf("StartNonBlocking() error = %v", err)
	}
	conn, err := net.Dial("tcp", srv.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial streaming listener: %v", err)
	}
	defer conn.Close()

	select {
	case accepted := <-called:
		if accepted == nil {
			t.Fatalf("accepted nil conn")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for accepted connection")
	}
}

func TestStreamingServerCloseHelpers(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	srv := &streamingServer{logger: logger}
	if err := srv.Close(); err != nil {
		t.Fatalf("Close nil listener error = %v", err)
	}
	if err := srv.close(nil); err != nil {
		t.Fatalf("close nil error = %v", err)
	}

	closeErr := errors.New("close")
	if err := srv.close(errorCloser{err: closeErr}); !errors.Is(err, closeErr) {
		t.Fatalf("close error = %v, want %v", err, closeErr)
	}
	srv.silentClose(errorCloser{err: closeErr})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv.listener = listener
	if err := srv.Close(); err != nil {
		t.Fatalf("Close listener error = %v", err)
	}
}

func TestStreamingServerHandleConn(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	called := make(chan net.Conn, 1)
	srv := &streamingServer{
		logger: logger,
		service: &serverFakeService{
			kind:       proto.ServiceStreaming.Kind,
			uri:        proto.ServiceStreaming.URI,
			connCalled: called,
		},
	}
	serverConn, clientConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(serverConn)
	}()

	select {
	case conn := <-called:
		if conn == nil {
			t.Fatalf("HandleConn received nil conn")
		}
	case <-done:
		t.Fatalf("handleConn returned before service was called")
	}
	<-done

	if _, err := clientConn.Read(make([]byte, 1)); err == nil {
		t.Fatalf("expected closed pipe after handleConn")
	}
}
