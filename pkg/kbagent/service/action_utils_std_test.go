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
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

// --- gather ---

func TestGather_EmptyChannel(t *testing.T) {
	ch := make(chan int, 1)
	result := gather(ch)
	assert.Nil(t, result)
}

func TestGather_ValueAvailable(t *testing.T) {
	ch := make(chan int, 1)
	ch <- 42
	result := gather(ch)
	require.NotNil(t, result)
	assert.Equal(t, 42, *result)
}

func TestGather_ClosedChannel(t *testing.T) {
	ch := make(chan int)
	close(ch)
	result := gather(ch)
	assert.Nil(t, result)
}

// --- actionCallTimeoutContext ---

func TestActionCallTimeoutContext_NilTimeout(t *testing.T) {
	ctx, cancel := actionCallTimeoutContext(context.Background(), nil)
	defer cancel()
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(defaultActionCallTimeout), deadline, 2*time.Second)
}

func TestActionCallTimeoutContext_ZeroTimeout(t *testing.T) {
	timeout := int32(0)
	ctx, cancel := actionCallTimeoutContext(context.Background(), &timeout)
	defer cancel()
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(defaultActionCallTimeout), deadline, 2*time.Second)
}

func TestActionCallTimeoutContext_PositiveTimeout(t *testing.T) {
	timeout := int32(10)
	ctx, cancel := actionCallTimeoutContext(context.Background(), &timeout)
	defer cancel()
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(10*time.Second), deadline, 2*time.Second)
}

func TestActionCallTimeoutContext_ExceedsMax(t *testing.T) {
	timeout := int32(120) // exceeds maxActionCallTimeout (60s)
	ctx, cancel := actionCallTimeoutContext(context.Background(), &timeout)
	defer cancel()
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(maxActionCallTimeout), deadline, 2*time.Second)
}

func TestActionCallTimeoutContext_NegativeTimeout(t *testing.T) {
	timeout := int32(-1)
	ctx, cancel := actionCallTimeoutContext(context.Background(), &timeout)
	defer cancel()
	_, ok := ctx.Deadline()
	assert.False(t, ok, "negative timeout means no deadline")
}

// --- httpActionMethodNURL ---

func TestHttpActionMethodNURL_Defaults(t *testing.T) {
	action := &proto.HTTPAction{Port: "8080"}
	method, url := httpActionMethodNURL(action)
	assert.Equal(t, "GET", method)
	assert.Equal(t, "HTTP://127.0.0.1:8080/", url)
}

func TestHttpActionMethodNURL_Custom(t *testing.T) {
	action := &proto.HTTPAction{
		Port:   "9090",
		Host:   "myhost",
		Scheme: "HTTPS",
		Path:   "/health",
		Method: "POST",
	}
	method, url := httpActionMethodNURL(action)
	assert.Equal(t, "POST", method)
	assert.Equal(t, "HTTPS://myhost:9090/health", url)
}

// --- renderTemplateData ---

func TestRenderTemplateData_EmptyData(t *testing.T) {
	result, err := renderTemplateData("test", nil, "")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestRenderTemplateData_NoTemplate(t *testing.T) {
	result, err := renderTemplateData("test", nil, "plain text")
	require.NoError(t, err)
	assert.Equal(t, "plain text", result)
}

func TestRenderTemplateData_WithParameters(t *testing.T) {
	params := map[string]string{"name": "world"}
	result, err := renderTemplateData("test", params, "hello {{.name}}")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestRenderTemplateData_InvalidTemplate(t *testing.T) {
	_, err := renderTemplateData("test", nil, "{{.missing}}")
	require.Error(t, err)
}

func TestRenderTemplateData_ParseError(t *testing.T) {
	_, err := renderTemplateData("test", nil, "{{bad template")
	require.Error(t, err)
}

// --- mergeEnvWith ---

func TestMergeEnvWith_EmptyParams(t *testing.T) {
	result := mergeEnvWith(nil)
	assert.NotNil(t, result)
	// Should contain OS env vars
	assert.NotEmpty(t, result)
}

func TestMergeEnvWith_ParamsTakePrecedence(t *testing.T) {
	params := map[string]string{"PATH": "custom-path"}
	result := mergeEnvWith(params)
	assert.Equal(t, "custom-path", result["PATH"])
}

// --- blockingCallAction ---

func TestBlockingCallAction_ExecSuccess(t *testing.T) {
	action := &proto.Action{
		Name: "echo",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/echo", "-n", "hello"},
		},
	}
	output, err := blockingCallAction(context.Background(), action, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), output)
}

func TestBlockingCallAction_ExecFail(t *testing.T) {
	action := &proto.Action{
		Name: "fail",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "exit 1"},
		},
	}
	_, err := blockingCallAction(context.Background(), action, nil, nil)
	require.Error(t, err)
}

func TestBlockingCallAction_ExecWithStderr(t *testing.T) {
	action := &proto.Action{
		Name: "stderr",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "echo -n errout >&2; exit 1"},
		},
	}
	_, err := blockingCallAction(context.Background(), action, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "errout")
}

// --- nonBlockingCallActionX with invalid action type ---

func TestNonBlockingCallActionX_InvalidType(t *testing.T) {
	action := &proto.Action{Name: "invalid"}
	_, err := nonBlockingCallActionX(context.Background(), action, nil, nil, nil, nil, nil)
	require.Error(t, err)
}

// --- httpClient ---

func TestHttpClient_Cached(t *testing.T) {
	c1 := httpClient()
	c2 := httpClient()
	assert.Same(t, c1, c2)
}

// --- safeClose / safeCloseF ---

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

func TestSafeClose(t *testing.T) {
	safeClose(nopCloser{})
}

func TestSafeCloseF(t *testing.T) {
	safeCloseF(func() error { return nil })
}

// --- nonBlockingCallAction ---

func TestNonBlockingCallAction_Success(t *testing.T) {
	action := &proto.Action{
		Name: "echo",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/echo", "-n", "async-out"},
		},
	}
	ch, err := nonBlockingCallAction(context.Background(), action, nil, nil)
	require.NoError(t, err)
	result := <-ch
	require.NoError(t, result.err)
	assert.Equal(t, "async-out", result.stdout.String())
}

// --- execActionCallX with parameters as env ---

func TestExecActionCallX_WithParameters(t *testing.T) {
	action := &proto.Action{
		Name: "env-echo",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sh", "-c", "printf '%s' $MY_VAR"},
		},
	}
	params := map[string]string{"MY_VAR": "test-value"}
	output, err := blockingCallAction(context.Background(), action, params, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("test-value"), output)
}

// --- blockingCallAction with timeout ---

func TestBlockingCallAction_Timeout(t *testing.T) {
	action := &proto.Action{
		Name: "sleep",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/sleep", "10"},
		},
	}
	timeout := int32(1)
	_, err := blockingCallAction(context.Background(), action, nil, &timeout)
	require.Error(t, err)
	assert.ErrorIs(t, err, proto.ErrTimedOut)
}

// --- nonBlockingCallActionX with negative timeout (no deadline) ---

func TestNonBlockingCallActionX_NegativeTimeout(t *testing.T) {
	action := &proto.Action{
		Name: "echo",
		Exec: &proto.ExecAction{
			Commands: []string{"/bin/echo", "-n", "no-deadline"},
		},
	}
	timeout := ptr.To(int32(-1))
	ch, err := nonBlockingCallActionX(context.Background(), action, nil, timeout, nil, nil, nil)
	require.NoError(t, err)
	execErr := <-ch
	assert.NoError(t, execErr)
}

// --- httpActionCallX with httptest ---

func TestHttpActionCallX_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	port := fmt.Sprintf("%d", srv.Listener.Addr().(*net.TCPAddr).Port)
	action := &proto.HTTPAction{
		Port:   port,
		Host:   "127.0.0.1",
		Scheme: "HTTP",
		Method: "GET",
		Path:   "/health",
	}
	protoAction := &proto.Action{
		Name: "http-test",
		HTTP: action,
	}

	output, err := blockingCallAction(context.Background(), protoAction, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), output)
}

func TestHttpActionCallX_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	action := &proto.HTTPAction{
		Port:   fmt.Sprintf("%d", srv.Listener.Addr().(*net.TCPAddr).Port),
		Host:   "127.0.0.1",
		Scheme: "HTTP",
		Method: "GET",
		Path:   "/fail",
	}
	protoAction := &proto.Action{
		Name: "http-err",
		HTTP: action,
	}
	_, err := blockingCallAction(context.Background(), protoAction, nil, nil)
	require.Error(t, err)
}

func TestHttpActionCallX_WithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		w.WriteHeader(200)
		fmt.Fprintf(w, "received: %s", buf.String())
	}))
	defer srv.Close()

	action := &proto.HTTPAction{
		Port:   fmt.Sprintf("%d", srv.Listener.Addr().(*net.TCPAddr).Port),
		Host:   "127.0.0.1",
		Scheme: "HTTP",
		Method: "POST",
		Path:   "/data",
		Body:   "hello {{.name}}",
	}
	protoAction := &proto.Action{
		Name: "http-body",
		HTTP: action,
	}
	params := map[string]string{"name": "world"}
	output, err := blockingCallAction(context.Background(), protoAction, params, nil)
	require.NoError(t, err)
	assert.Contains(t, string(output), "received: hello world")
}

func TestHttpActionCallX_WithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, r.Header.Get("X-Custom"))
	}))
	defer srv.Close()

	action := &proto.HTTPAction{
		Port:   fmt.Sprintf("%d", srv.Listener.Addr().(*net.TCPAddr).Port),
		Host:   "127.0.0.1",
		Scheme: "HTTP",
		Method: "GET",
		Headers: []proto.HTTPHeader{
			{Name: "X-Custom", Value: "my-val-{{.key}}"},
		},
	}
	protoAction := &proto.Action{
		Name: "http-headers",
		HTTP: action,
	}
	params := map[string]string{"key": "123"}
	output, err := blockingCallAction(context.Background(), protoAction, params, nil)
	require.NoError(t, err)
	assert.Equal(t, "my-val-123", string(output))
}

// --- httpActionMethodNURL with partial custom fields ---

func TestHttpActionMethodNURL_PartialCustom(t *testing.T) {
	action := &proto.HTTPAction{
		Port: "3000",
		Host: "custom-host",
	}
	method, url := httpActionMethodNURL(action)
	assert.Equal(t, "GET", method)
	assert.Equal(t, "HTTP://custom-host:3000/", url)
}

func TestHttpActionMethodNURL_CustomSchemeAndPath(t *testing.T) {
	action := &proto.HTTPAction{
		Port:   "443",
		Scheme: "HTTPS",
		Path:   "/api/v1/check",
	}
	method, url := httpActionMethodNURL(action)
	assert.Equal(t, "GET", method)
	assert.Equal(t, "HTTPS://127.0.0.1:443/api/v1/check", url)
}
