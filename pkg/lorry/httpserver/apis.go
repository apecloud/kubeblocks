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

package httpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

const (
	jsonContentTypeHeader = "application/json"
	version               = "v1.0"
)

type option = func(ctx *fasthttp.RequestCtx)

type OperationAPI interface {
	Endpoints() []Endpoint
	RegisterOperations(map[string]operations.Operation)
}

type api struct {
	endpoints []Endpoint
	ready     bool
}

func (a *api) Endpoints() []Endpoint {
	return a.endpoints
}

func (a *api) RegisterOperations(ops map[string]operations.Operation) {
	endpoints := make([]Endpoint, 0, len(ops))

	for key, op := range ops {
		err := op.Init(context.Background())
		if err != nil {
			logger.Error(err, "operation init failed", "operation", key)
			continue
		}

		endpoint := Endpoint{
			Version: version,
		}

		if op.IsReadonly(context.Background()) {
			endpoint.Method = fasthttp.MethodGet
		} else {
			endpoint.Method = fasthttp.MethodPost
		}

		// opType := reflect.TypeOf(op)
		endpoint.Route = key
		endpoint.Handler = OperationWrapper(op)
		endpoints = append(endpoints, endpoint)
	}
	a.endpoints = endpoints
	a.ready = true
}

func OperationWrapper(op operations.Operation) fasthttp.RequestHandler {
	return func(reqCtx *fasthttp.RequestCtx) {
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

		b, err := json.Marshal(req.Data)
		if err != nil {
			msg := NewErrorResponse("ERR_MALFORMED_REQUEST_DATA", fmt.Sprintf("marshal request data field: %v", err))
			respond(reqCtx, withError(fasthttp.StatusInternalServerError, msg))
			logger.Error(err, "marshal request data field")
			return
		}
		opsReq := &operations.OpsRequest{
			Parameters: req.Parameters,
			Data:       b,
		}

		if err := op.PreCheck(ctx, opsReq); err != nil {
			msg := NewErrorResponse("ERR_PRECHECK_FAILED", fmt.Sprintf("operation precheck failed: %v", err))
			respond(reqCtx, withError(fasthttp.StatusInternalServerError, msg))
			logger.Error(err, "operation precheck failed")
			return
		}

		resp, err := op.Do(ctx, opsReq)

		statusCode := fasthttp.StatusOK
		if err != nil {
			if ok := errors.As(err, &util.ProbeError{}); ok {
				statusCode = fasthttp.StatusUnavailableForLegalReasons
			} else {
				if errors.Is(err, models.ErrNoImplemented) {
					statusCode = fasthttp.StatusNotImplemented
				} else {
					statusCode = fasthttp.StatusInternalServerError
					logger.Error(err, "operation exec failed")
				}
				msg := NewErrorResponse("ERR_OPERATION_FAILED", fmt.Sprintf("operation exec failed: %v", err))
				respond(reqCtx, withError(statusCode, msg))
				return
			}
		}

		if resp == nil {
			respond(reqCtx, withEmpty())
		} else {
			body, _ = json.Marshal(resp.Data)
			respond(reqCtx, withMetadata(resp.Metadata), withJSON(statusCode, body))
		}
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

func withMetadata(metadata map[string]string) option {
	return func(ctx *fasthttp.RequestCtx) {
		for k, v := range metadata {
			ctx.Response.Header.Set("KB."+k, v)
		}
	}
}
func respond(ctx *fasthttp.RequestCtx, options ...option) {
	for _, option := range options {
		option(ctx)
	}
}
