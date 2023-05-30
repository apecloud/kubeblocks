/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

// mockedRequestHandler acts like an upstream service, returns success status code 200 and a fixed response body.
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
