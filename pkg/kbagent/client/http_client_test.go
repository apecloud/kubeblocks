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

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read")
}

func newHTTPClientForTest(t *testing.T, handler http.HandlerFunc) (*httpClient, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	host, portString, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int32
	if _, err := fmt.Sscan(portString, &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return &httpClient{host: host, port: port, client: server.Client()}, server.Close
}

func TestHTTPClientAction(t *testing.T) {
	cli, closeServer := newHTTPClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != proto.ServiceAction.URI || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"done","output":"b2s="}`))
	})
	defer closeServer()

	resp, err := cli.Action(context.Background(), proto.ActionRequest{Action: "backup"})
	if err != nil {
		t.Fatalf("Action() error = %v", err)
	}
	if resp.Message != "done" || string(resp.Output) != "ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}

	resp, err = cli.Action(context.WithValue(context.Background(), constant.DryRunContextKey, true), proto.ActionRequest{Action: "backup"})
	if err != nil || resp.Message != "" || resp.Error != "" || len(resp.Output) != 0 {
		t.Fatalf("dry-run Action() = %#v, %v", resp, err)
	}
}

func TestHTTPClientRequestAndDecodeErrors(t *testing.T) {
	cli, closeServer := newHTTPClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("case") {
		case "bad-status":
			w.WriteHeader(http.StatusAccepted)
		case "invalid-json":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("{"))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"error":"failed"}`))
		}
	})
	defer closeServer()

	if _, err := cli.request(context.Background(), " bad method", "http://bad-url", nil); err == nil {
		t.Fatalf("expected request construction error")
	}

	body, err := cli.request(context.Background(), http.MethodPost, "http://"+net.JoinHostPort(cli.host, fmt.Sprint(cli.port))+proto.ServiceAction.URI, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	_ = body.Close()

	_, err = cli.request(context.Background(), http.MethodPost, "http://"+net.JoinHostPort(cli.host, fmt.Sprint(cli.port))+proto.ServiceAction.URI+"?case=bad-status", nil)
	if err == nil {
		t.Fatalf("expected unexpected status error")
	}

	body, err = cli.request(context.Background(), http.MethodPost, "http://"+net.JoinHostPort(cli.host, fmt.Sprint(cli.port))+proto.ServiceAction.URI+"?case=invalid-json", nil)
	if err != nil {
		t.Fatalf("request invalid-json error = %v", err)
	}
	defer body.Close()
	if _, err := decode(body, &proto.ActionResponse{}); err == nil {
		t.Fatalf("expected decode error")
	}

	if _, err := decode(io.NopCloser(errorReader{}), &proto.ActionResponse{}); err == nil {
		t.Fatalf("expected read error")
	}
}
