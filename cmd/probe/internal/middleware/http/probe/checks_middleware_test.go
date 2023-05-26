/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package probe

import (
	"net/http"
	"testing"

	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

const checkFailedHTTPCode = "451"

// mockedRequestHandler acts like an upstream service returns success status code 200 and a fixed response body.
func mockedRequestHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.SetStatusCode(http.StatusOK)
	ctx.Response.SetBodyString("mock response")
}

func TestRequestHandlerWithIllegalRouterRule(t *testing.T) {
	meta := middleware.Metadata{
		Base: metadata.Base{
			Properties: map[string]string{},
		},
	}
	log := logger.NewLogger("probemiddleware.test")
	middleware := NewProbeMiddleware(log)
	handler, err := middleware.GetHandler(meta)
	assert.Nil(t, err)

	t.Run("hit: status check request", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetHost("localhost:3501")
		ctx.Request.SetRequestURI("/v1.0/bindings/probe?operation=statusCheck")
		ctx.Request.Header.SetHost("localhost:3501")
		ctx.Request.Header.SetMethod("GET")

		handler(mockedRequestHandler)(&ctx)
		assert.Equal(t, http.StatusOK, ctx.Response.Header.StatusCode())
		assert.Equal(t, http.MethodPost, string(ctx.Request.Header.Method()))
	})

	t.Run("hit: status code handler", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetHost("localhost:3501")
		ctx.Request.SetRequestURI("/v1.0/bindings/probe?operation=statusCheck")
		ctx.Request.Header.SetHost("localhost:3501")
		ctx.Request.Header.SetMethod("GET")
		ctx.Response.Header.Add(statusCodeHeader, checkFailedHTTPCode)
		handler(mockedRequestHandler)(&ctx)
		assert.Equal(t, 451, ctx.Response.Header.StatusCode())
		assert.Equal(t, http.MethodPost, string(ctx.Request.Header.Method()))
	})
}
