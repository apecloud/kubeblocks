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

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
)

const (
	jsonContentTypeHeader = "application/json"
	version               = "v1.0"
)

type option = func(ctx *fasthttp.RequestCtx)

func Endpoints() []Endpoint {
	return []Endpoint{
		{
			Route:   "/action",
			Method:  fasthttp.MethodPost,
			Version: version,
			Handler: actionHandler,
		},
	}

}

func actionHandler(reqCtx *fasthttp.RequestCtx) {
	ctx := context.Background()
	body := reqCtx.PostBody()

	var req Request
	if len(body) > 0 {
		err := json.Unmarshal(body, &req)
		if err != nil {
			msg := NewErrorResponse("ERR_MALFORMED_REQUEST", fmt.Sprintf("unmarshal HTTP body failed: %v", err))
			respond(reqCtx, withError(fasthttp.StatusBadRequest, msg))
			return
		}
	}

	_, err := json.Marshal(req.Data)
	if err != nil {
		msg := NewErrorResponse("ERR_MALFORMED_REQUEST_DATA", fmt.Sprintf("marshal request data field: %v", err))
		respond(reqCtx, withError(fasthttp.StatusInternalServerError, msg))
		logger.Info("marshal request data field", "error", err.Error())
		return
	}

	if req.Action == "" {
		msg := NewErrorResponse("ERR_MALFORMED_REQUEST_DATA", "no action in request")
		respond(reqCtx, withError(fasthttp.StatusBadRequest, msg))
		return
	}

	resp, err := handlers.Do(ctx, req.Action, req.Parameters)
	statusCode := fasthttp.StatusOK
	if err != nil {
		if errors.Is(err, handlers.ErrNotImplemented) {
			statusCode = fasthttp.StatusNotImplemented
		} else {
			statusCode = fasthttp.StatusInternalServerError
			logger.Info("action exec failed", "action", req.Action, "error", err.Error())
		}
		msg := NewErrorResponse("ERR_ACTION_FAILED", fmt.Sprintf("action exec failed: %s", err.Error()))
		respond(reqCtx, withError(statusCode, msg))
		return
	}

	if resp == nil {
		respond(reqCtx, withEmpty())
	} else {
		body, _ = json.Marshal(resp)
		respond(reqCtx, withJSON(statusCode, body))
	}
}

// withJSON overrides the content-type with application/json.
func withJSON(code int, obj []byte) option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetStatusCode(code)
		ctx.Response.SetBody(obj)
		ctx.Response.Header.SetContentType(jsonContentTypeHeader)
	}
}

// withError sets error code and jsonify error message.
func withError(code int, resp ErrorResponse) option {
	b, _ := json.Marshal(&resp)
	return withJSON(code, b)
}

func withEmpty() option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetBody(nil)
		ctx.Response.SetStatusCode(fasthttp.StatusNoContent)
	}
}

func respond(ctx *fasthttp.RequestCtx, options ...option) {
	for _, option := range options {
		option(ctx)
	}
}
