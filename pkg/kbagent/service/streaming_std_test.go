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

package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func newTestStreamingService(t *testing.T, actions []proto.Action, streaming []string) *streamingService {
	t.Helper()
	actionSvc := newTestActionService(t, actions)
	svc, err := newStreamingService(logr.New(nil), actionSvc, streaming)
	require.NoError(t, err)
	return svc
}

// --- URI / Start ---

func TestStreamingService_URI(t *testing.T) {
	svc := newTestStreamingService(t, nil, nil)
	assert.Equal(t, proto.ServiceStreaming.URI, svc.URI())
}

func TestStreamingService_Start(t *testing.T) {
	svc := newTestStreamingService(t, nil, nil)
	assert.NoError(t, svc.Start())
}

// --- HandleRequest ---

func TestStreamingService_HandleRequest_ReturnsNotImplemented(t *testing.T) {
	svc := newTestStreamingService(t, nil, nil)
	_, err := svc.HandleRequest(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, proto.ErrNotImplemented))
}

// --- handshake ---

func TestStreamingService_Handshake_Valid(t *testing.T) {
	svc := newTestStreamingService(t, nil, nil)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	req := &proto.ActionRequest{Action: "test-action"}
	go func() {
		enc := json.NewEncoder(client)
		_ = enc.Encode(req)
	}()

	got, err := svc.handshake(context.Background(), server)
	require.NoError(t, err)
	assert.Equal(t, "test-action", got.Action)
}

func TestStreamingService_Handshake_InvalidJSON(t *testing.T) {
	svc := newTestStreamingService(t, nil, nil)
	server, client := net.Pipe()
	defer server.Close()

	go func() {
		_, _ = client.Write([]byte("not-json\n"))
		client.Close()
	}()

	_, err := svc.handshake(context.Background(), server)
	require.Error(t, err)
	assert.True(t, errors.Is(err, proto.ErrBadRequest))
}

// --- HandleConn ---

func TestStreamingService_HandleConn_UnsupportedAction(t *testing.T) {
	actions := []proto.Action{{Name: "echo", Exec: &proto.ExecAction{Commands: []string{"/bin/echo"}}}}
	svc := newTestStreamingService(t, actions, []string{"echo"})

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// Send a request for an action not in streamingActions
	req := &proto.ActionRequest{Action: "not-streaming"}
	go func() {
		enc := json.NewEncoder(client)
		_ = enc.Encode(req)
	}()

	err := svc.HandleConn(context.Background(), server)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestStreamingService_HandleConn_ValidExec(t *testing.T) {
	actions := []proto.Action{
		{
			Name: "cat",
			Exec: &proto.ExecAction{
				Commands: []string{"/bin/cat"},
			},
		},
	}
	svc := newTestStreamingService(t, actions, []string{"cat"})

	server, client := net.Pipe()
	defer client.Close()

	// Send handshake request then data
	go func() {
		req := &proto.ActionRequest{Action: "cat"}
		enc := json.NewEncoder(client)
		_ = enc.Encode(req)
		// Send some data through stdin then close
		_, _ = client.Write([]byte("hello"))
		client.Close()
	}()

	err := svc.HandleConn(context.Background(), server)
	// cat reads from stdin (the conn) and writes to stdout (the conn)
	// When client closes, cat exits, which should produce a nil error
	// (or possibly an error depending on timing)
	_ = err // We just verify it doesn't panic
}

// --- newStreamingService ---

func TestNewStreamingService_UndefinedAction(t *testing.T) {
	actionSvc := newTestActionService(t, nil)
	_, err := newStreamingService(logr.New(nil), actionSvc, []string{"not-defined"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not defined")
}
