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
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func newTestActionService(t *testing.T, actions []proto.Action) *actionService {
	t.Helper()
	svc, err := newActionService(logr.New(nil), actions)
	require.NoError(t, err)
	return svc
}

// --- URI / Start / HandleConn ---

func TestActionService_URI(t *testing.T) {
	svc := newTestActionService(t, nil)
	assert.Equal(t, proto.ServiceAction.URI, svc.URI())
}

func TestActionService_Start(t *testing.T) {
	svc := newTestActionService(t, nil)
	assert.NoError(t, svc.Start())
}

func TestActionService_HandleConn(t *testing.T) {
	svc := newTestActionService(t, nil)
	assert.NoError(t, svc.HandleConn(context.Background(), nil))
}

// --- decode ---

func TestActionService_Decode_Valid(t *testing.T) {
	svc := newTestActionService(t, nil)
	payload, _ := json.Marshal(&proto.ActionRequest{Action: "test"})
	req, err := svc.decode(payload)
	require.NoError(t, err)
	assert.Equal(t, "test", req.Action)
}

func TestActionService_Decode_InvalidJSON(t *testing.T) {
	svc := newTestActionService(t, nil)
	_, err := svc.decode([]byte("not-json"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, proto.ErrBadRequest))
}

// --- encode ---

func TestActionService_Encode_Success(t *testing.T) {
	svc := newTestActionService(t, nil)
	data := svc.encode([]byte("output"), nil)
	var rsp proto.ActionResponse
	require.NoError(t, json.Unmarshal(data, &rsp))
	assert.Equal(t, []byte("output"), rsp.Output)
	assert.Empty(t, rsp.Error)
}

func TestActionService_Encode_Error(t *testing.T) {
	svc := newTestActionService(t, nil)
	data := svc.encode(nil, proto.ErrFailed)
	var rsp proto.ActionResponse
	require.NoError(t, json.Unmarshal(data, &rsp))
	assert.Equal(t, "failed", rsp.Error)
	assert.Nil(t, rsp.Output)
}

// --- HandleRequest ---

func TestActionService_HandleRequest_ValidExec(t *testing.T) {
	actions := []proto.Action{
		{
			Name: "echo",
			Exec: &proto.ExecAction{
				Commands: []string{"/bin/echo", "-n", "hello"},
			},
		},
	}
	svc := newTestActionService(t, actions)
	payload, _ := json.Marshal(&proto.ActionRequest{Action: "echo"})
	data, err := svc.HandleRequest(context.Background(), payload)
	require.NoError(t, err) // HandleRequest always returns nil error, wraps in response
	require.NotNil(t, data)

	var rsp proto.ActionResponse
	require.NoError(t, json.Unmarshal(data, &rsp))
	assert.Empty(t, rsp.Error)
	assert.Equal(t, []byte("hello"), rsp.Output)
}

func TestActionService_HandleRequest_BadJSON(t *testing.T) {
	svc := newTestActionService(t, nil)
	data, err := svc.HandleRequest(context.Background(), []byte("{bad"))
	require.NoError(t, err)

	var rsp proto.ActionResponse
	require.NoError(t, json.Unmarshal(data, &rsp))
	assert.Equal(t, "badRequest", rsp.Error)
}

func TestActionService_HandleRequest_ActionNotFound(t *testing.T) {
	svc := newTestActionService(t, nil)
	payload, _ := json.Marshal(&proto.ActionRequest{Action: "missing"})
	data, err := svc.HandleRequest(context.Background(), payload)
	require.NoError(t, err)

	var rsp proto.ActionResponse
	require.NoError(t, json.Unmarshal(data, &rsp))
	assert.Equal(t, "notDefined", rsp.Error)
}

func TestActionService_HandleRequest_InvalidAction(t *testing.T) {
	// Action exists but has no Exec/HTTP/GRPC
	actions := []proto.Action{{Name: "empty"}}
	svc := newTestActionService(t, actions)
	payload, _ := json.Marshal(&proto.ActionRequest{Action: "empty"})
	data, err := svc.HandleRequest(context.Background(), payload)
	require.NoError(t, err)

	var rsp proto.ActionResponse
	require.NoError(t, json.Unmarshal(data, &rsp))
	assert.Equal(t, "badRequest", rsp.Error)
}

// --- handleRequestNonBlocking ---

func TestActionService_HandleRequestNonBlocking_Exec(t *testing.T) {
	actions := []proto.Action{
		{
			Name: "echo",
			Exec: &proto.ExecAction{
				Commands: []string{"/bin/echo", "-n", "nb-result"},
			},
		},
	}
	svc := newTestActionService(t, actions)
	nb := true
	payload, _ := json.Marshal(&proto.ActionRequest{Action: "echo", NonBlocking: &nb})
	ctx := context.Background()

	// First call starts the action — may get inProgress or done depending on timing
	data1, err := svc.HandleRequest(ctx, payload)
	require.NoError(t, err)

	// Give time for the exec to complete
	time.Sleep(200 * time.Millisecond)

	// Second call should return the result
	data2, err := svc.HandleRequest(ctx, payload)
	require.NoError(t, err)

	// At least one of the calls should have the result
	var rsp1, rsp2 proto.ActionResponse
	_ = json.Unmarshal(data1, &rsp1)
	_ = json.Unmarshal(data2, &rsp2)

	// The result should eventually be returned
	if rsp1.Error == "" && string(rsp1.Output) == "nb-result" {
		return // got result on first call
	}
	assert.Empty(t, rsp2.Error, "expected no error in final response")
	assert.Equal(t, []byte("nb-result"), rsp2.Output)
}

// --- resolveTimeout ---

func TestResolveTimeout_RequestTakesPrecedence(t *testing.T) {
	var action, req int32 = 10, 30
	result := resolveTimeout(&action, &req)
	assert.Equal(t, &req, result)
}

func TestResolveTimeout_FallbackToAction(t *testing.T) {
	var action int32 = 10
	result := resolveTimeout(&action, nil)
	assert.Equal(t, &action, result)
}

func TestResolveTimeout_BothNil(t *testing.T) {
	result := resolveTimeout(nil, nil)
	assert.Nil(t, result)
}

// --- callActionWithRetry ---

func TestCallActionWithRetry_SuccessNoRetry(t *testing.T) {
	action := &proto.Action{
		Name: "echo",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/echo", "-n", "ok"},
		},
	}
	output, err := callActionWithRetry(context.Background(), action, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), output)
}

func TestCallActionWithRetry_FailNoRetryPolicy(t *testing.T) {
	action := &proto.Action{
		Name: "fail",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "exit 1"},
		},
	}
	_, err := callActionWithRetry(context.Background(), action, nil, nil, nil)
	require.Error(t, err)
}

func TestCallActionWithRetry_RetryAndEventuallyFail(t *testing.T) {
	action := &proto.Action{
		Name: "fail",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "exit 1"},
		},
	}
	policy := &proto.RetryPolicy{
		MaxRetries:    2,
		RetryInterval: 10 * time.Millisecond,
	}
	_, err := callActionWithRetry(context.Background(), action, nil, nil, policy)
	require.Error(t, err)
}

func TestCallActionWithRetry_RetryZeroMaxRetries(t *testing.T) {
	action := &proto.Action{
		Name: "fail",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "exit 1"},
		},
	}
	policy := &proto.RetryPolicy{
		MaxRetries: 0,
	}
	_, err := callActionWithRetry(context.Background(), action, nil, nil, policy)
	require.Error(t, err)
}

func TestCallActionWithRetry_ContextCancelled(t *testing.T) {
	action := &proto.Action{
		Name: "sleep",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "exit 1"},
		},
	}
	policy := &proto.RetryPolicy{
		MaxRetries:    10,
		RetryInterval: 5 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := callActionWithRetry(ctx, action, nil, nil, policy)
	require.Error(t, err)
}
