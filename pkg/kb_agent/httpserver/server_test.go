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

package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

func mockServer(t *testing.T) *server {
	fakeOps := map[string]actions.Action{
		"fake-1": actions.NewFakeAction(actions.FakeDefault, nil),
		"fake-2": actions.NewFakeAction(actions.FakePreCheck, func(ctx context.Context, request *actions.OpsRequest) error {
			return fmt.Errorf("fake pre check error")
		}),
		"fake-3": actions.NewFakeAction(actions.FakeDo, func(ctx context.Context, request *actions.OpsRequest) (*actions.OpsResponse, error) {
			return nil, models.ErrNotImplemented
		}),
		"fake-4": actions.NewFakeAction(actions.FakeDo, func(ctx context.Context, request *actions.OpsRequest) (*actions.OpsResponse, error) {
			return nil, util.NewProbeError("fake probe error")
		}),
		"fake-5": actions.NewFakeAction(actions.FakeDo, func(ctx context.Context, request *actions.OpsRequest) (*actions.OpsResponse, error) {
			return nil, fmt.Errorf("fake do error")
		}),
		"fake-6": actions.NewFakeAction(actions.FakeDo, func(ctx context.Context, request *actions.OpsRequest) (*actions.OpsResponse, error) {
			return &actions.OpsResponse{
				Data: map[string]any{
					"data": request.Data,
				},
				Metadata: map[string]string{
					"fake-meta": "fake",
				},
			}, nil
		}),
	}

	s := NewServer(fakeOps)
	assert.NotNil(t, s)
	fakeServer, ok := s.(*server)
	assert.True(t, ok)

	return fakeServer
}

func mockHTTPRequest(url string, method string, body string) *fasthttp.RequestCtx {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI(url)
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetBodyString(body)
	ctx.Request.Header.Set("User-Agent", "fake-agent")

	return ctx
}

func parseErrorResponse(t *testing.T, rawErrorResponse []byte) *ErrorResponse {
	resp := &ErrorResponse{}
	err := json.Unmarshal(rawErrorResponse, resp)
	assert.Nil(t, err)

	return resp
}

func TestRouter(t *testing.T) {
	fakeServer := mockServer(t)

	handler := fakeServer.Router()
	assert.NotNil(t, handler)
	fakeRouterHandler := fakeServer.apiLogger(handler)

	t.Run("unmarshal HTTP body failed", func(t *testing.T) {
		ctx := mockHTTPRequest("/v1.0/fake-1", fasthttp.MethodPost, `test`)
		fakeRouterHandler(ctx)

		response := parseErrorResponse(t, ctx.Response.Body())
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
		assert.Equal(t, "ERR_MALFORMED_REQUEST", response.ErrorCode)
		assert.Equal(t, "unmarshal HTTP body failed: invalid character 'e' in literal true (expecting 'r')", response.Message)
	})

	t.Run("pre check failed", func(t *testing.T) {
		ctx := mockHTTPRequest("/v1.0/fake-2", fasthttp.MethodPost, `{"data": "test"}`)
		fakeRouterHandler(ctx)

		response := parseErrorResponse(t, ctx.Response.Body())
		assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())
		assert.Equal(t, "ERR_PRECHECK_FAILED", response.ErrorCode)
		assert.Equal(t, "operation precheck failed: fake pre check error", response.Message)
	})

	t.Run("do check not implemented", func(t *testing.T) {
		ctx := mockHTTPRequest("/v1.0/fake-3", fasthttp.MethodPost, `{"data": "test"}`)
		fakeRouterHandler(ctx)

		response := parseErrorResponse(t, ctx.Response.Body())
		assert.Equal(t, fasthttp.StatusNotImplemented, ctx.Response.StatusCode())
		assert.Equal(t, "ERR_OPERATION_FAILED", response.ErrorCode)
		assert.Equal(t, "operation exec failed: not implemented", response.Message)
	})

	t.Run("do check probe error", func(t *testing.T) {
		ctx := mockHTTPRequest("/v1.0/fake-4", fasthttp.MethodPost, `{"data": "test"}`)
		fakeRouterHandler(ctx)

		assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())
		assert.Empty(t, ctx.Response.Body())
	})

	t.Run("do check failed", func(t *testing.T) {
		ctx := mockHTTPRequest("/v1.0/fake-5", fasthttp.MethodPost, `{"data": "test"}`)
		fakeRouterHandler(ctx)

		response := parseErrorResponse(t, ctx.Response.Body())
		assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())
		assert.Equal(t, "ERR_OPERATION_FAILED", response.ErrorCode)
		assert.Equal(t, "operation exec failed: fake do error", response.Message)
	})

	t.Run("return meta data", func(t *testing.T) {
		ctx := mockHTTPRequest("/v1.0/fake-6", fasthttp.MethodPost, `{"data": "test"}`)
		fakeRouterHandler(ctx)

		assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
		assert.Equal(t, string(ctx.Response.Body()), `{"data":"InRlc3Qi"}`)
		assert.Equal(t, []byte("fake"), ctx.Response.Header.Peek("KB.fake-meta"))
	})
}

func TestStartNonBlocking(t *testing.T) {
	fakeServer := mockServer(t)

	err := fakeServer.StartNonBlocking()
	assert.Nil(t, err)
	err = fakeServer.Close()
	assert.Nil(t, err)
}
