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
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
	"github.com/valyala/fasthttp"

	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

const (
	bindingPath  = "/v1.0/bindings"
	operationKey = "operation"

	// the key is used to bypass the dapr framework and set http status code.
	// "status-code" is the key defined by probe, but this will be changed
	// by dapr framework and http framework in the end.
	statusCodeHeader = "Metadata.status-Code"
	bodyFmt          = `{"operation": "%s", "metadata": {"sql" : ""}}`
)

type RequestMeta struct {
	Operation string            `json:"operation"`
	Metadata  map[string]string `json:"metadata"`
}

// NewProbeMiddleware returns a new probe middleware.
func NewProbeMiddleware(log logger.Logger) middleware.Middleware {
	return &Middleware{logger: log}
}

// Middleware is a probe middleware.
type Middleware struct {
	logger logger.Logger
}

var _ middleware.Middleware = &Middleware{}

// type statusCodeWriter struct {
// 	http.ResponseWriter
// 	logger logger.Logger
// }

// func (scw *statusCodeWriter) WriteHeader(statusCode int) {
// 	header := scw.ResponseWriter.Header()
// 	scw.logger.Debugf("response header: %v", header)
// 	if v, ok := header[statusCodeHeader]; ok {
// 		scw.logger.Debugf("set statusCode: %v", v)
// 		statusCode, _ = strconv.Atoi(v[0])
// 		delete(header, statusCodeHeader)
// 	}
// 	scw.ResponseWriter.WriteHeader(statusCode)
// }

// GetHandler returns the HTTP handler provided by the middleware.
func (m *Middleware) GetHandler(metadata middleware.Metadata) (func(next fasthttp.RequestHandler) fasthttp.RequestHandler, error) {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			uri := ctx.Request.URI()
			method := string(ctx.Request.Header.Method())
			if method == http.MethodGet && strings.HasPrefix(string(uri.Path()), bindingPath) {
				ctx.Request.Header.SetMethod(http.MethodPost)

				args := uri.QueryArgs()
				switch operation := args.Peek(operationKey); bindings.OperationKind(operation) {
				case CheckStatusOperation, CheckRunningOperation, CheckRoleOperation, VolumeProtection:
					body := GetRequestBody(string(operation), args)
					ctx.Request.SetBody(body)
				default:
					m.logger.Infof("unknown probe operation: %v", string(operation))
				}
			}

			m.logger.Debugf("request: %v", ctx.Request.String())
			next(ctx)
			code := ctx.Response.Header.Peek(statusCodeHeader)
			statusCode, err := strconv.Atoi(string(code))
			if err == nil {
				// header has a statusCodeHeader
				ctx.Response.Header.SetStatusCode(statusCode)
				m.logger.Debugf("response abnormal: %v", ctx.Response.String())
			} else {
				// header has no statusCodeHeader
				m.logger.Debugf("response: %v", ctx.Response.String())
			}
		}
	}, nil
}

func GetRequestBody(operation string, args *fasthttp.Args) []byte {
	metadata := make(map[string]string)
	walkFunc := func(key, value []byte) {
		if string(key) == operationKey {
			return
		}
		metadata[string(key)] = string(value)
	}
	args.VisitAll(walkFunc)

	requestMeta := RequestMeta{
		Operation: operation,
		Metadata:  metadata,
	}

	body, _ := json.Marshal(requestMeta)
	return body
}
