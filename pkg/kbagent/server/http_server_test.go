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
	"context"
	"errors"
	"net"
	"testing"

	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2/ktesting"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

type serverFakeService struct {
	kind       string
	uri        string
	output     []byte
	err        error
	connCalled chan net.Conn
}

func (s *serverFakeService) Kind() string {
	return s.kind
}

func (s *serverFakeService) URI() string {
	return s.uri
}

func (s *serverFakeService) Start() error {
	return nil
}

func (s *serverFakeService) HandleConn(_ context.Context, conn net.Conn) error {
	if s.connCalled != nil {
		s.connCalled <- conn
	}
	return s.err
}

func (s *serverFakeService) HandleRequest(_ context.Context, payload []byte) ([]byte, error) {
	if string(payload) == "error" {
		return nil, s.err
	}
	return s.output, nil
}

func TestNewHTTPServer(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	srv := NewHTTPServer(logger, Config{Port: 3501}, nil)
	if srv == nil {
		t.Fatalf("NewHTTPServer() returned nil")
	}
	if _, ok := srv.(*httpServer); !ok {
		t.Fatalf("NewHTTPServer() returned %T, want *httpServer", srv)
	}
}

func TestHTTPServerStartNonBlockingNoEndpoint(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	srv := &httpServer{
		logger: logger,
		config: Config{Address: "256.256.256.256", Port: 3501},
	}
	if err := srv.StartNonBlocking(); err == nil {
		t.Fatalf("expected no endpoint error")
	}
}

func TestHTTPServerStartNonBlockingTCP(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	srv := &httpServer{
		logger: logger,
		config: Config{Address: "127.0.0.1", Port: 0},
	}
	if err := srv.StartNonBlocking(); err != nil {
		t.Fatalf("StartNonBlocking() error = %v", err)
	}
	if len(srv.servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(srv.servers))
	}
	if srv.servers[0].Concurrency != defaultMaxConcurrency {
		t.Fatalf("concurrency = %d, want %d", srv.servers[0].Concurrency, defaultMaxConcurrency)
	}
}

func TestHTTPServerClose(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	srv := &httpServer{
		logger:  logger,
		servers: []*fasthttp.Server{{}},
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestHTTPServerRouterDispatcherAndRespond(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	svc := &serverFakeService{
		kind:   proto.ServiceAction.Kind,
		uri:    proto.ServiceAction.URI,
		output: []byte(`{"ok":true}`),
		err:    errors.New("failed"),
	}
	srv := &httpServer{
		logger:   logger,
		config:   Config{Logging: true},
		services: []service.Service{svc},
	}
	handler := srv.router()

	ctx := runFastHTTP(handler, fasthttp.MethodPost, proto.ServiceAction.URI, "ok")
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Body()) != `{"ok":true}` {
		t.Fatalf("body = %q", ctx.Response.Body())
	}
	if string(ctx.Response.Header.ContentType()) != jsonContentTypeHeader {
		t.Fatalf("content type = %q", ctx.Response.Header.ContentType())
	}

	ctx = runFastHTTP(handler, fasthttp.MethodPost, proto.ServiceAction.URI, "error")
	if ctx.Response.StatusCode() != fasthttp.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Body()) != "failed" {
		t.Fatalf("error body = %q", ctx.Response.Body())
	}

	ctx = &fasthttp.RequestCtx{}
	httpRespond(ctx, fasthttp.StatusAccepted, nil, nil)
	if ctx.Response.StatusCode() != fasthttp.StatusAccepted || len(ctx.Response.Body()) != 0 {
		t.Fatalf("empty response = %d %q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func runFastHTTP(handler fasthttp.RequestHandler, method, uri, body string) *fasthttp.RequestCtx {
	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI(uri)
	req.SetBodyString(body)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&req, nil, nil)
	handler(ctx)
	return ctx
}
