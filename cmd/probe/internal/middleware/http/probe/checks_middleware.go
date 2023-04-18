/*
Copyright ApeCloud, Inc.

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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
	"github.com/valyala/fasthttp"

	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

const (
	bindingPath = "/v1.0/bindings"

	// the key is used to bypass the dapr framework and set http status code.
	// "status-code" is the key defined by probe, but this will changed like this
	// by dapr framework and http framework in the end.
	statusCodeHeader = "Metadata.status-Code"
	bodyFmt          = `{"operation": "%s", "metadata": {"sql" : ""}}`
)

// NewProbeMiddleware returns a new probe middleware.
func NewProbeMiddleware(log logger.Logger) middleware.Middleware {
	return &Middleware{logger: log}
}

// Middleware is an probe middleware.
type Middleware struct {
	logger logger.Logger
}

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
				var body string
				ctx.Request.Header.SetMethod(http.MethodPost)

				switch operation := uri.QueryArgs().Peek("operation"); bindings.OperationKind(operation) {
				case CheckStatusOperation:
					body = fmt.Sprintf(bodyFmt, CheckStatusOperation)
					ctx.Request.SetBody([]byte(body))
				case CheckRunningOperation:
					body = fmt.Sprintf(bodyFmt, CheckRunningOperation)
					ctx.Request.SetBody([]byte(body))
				case CheckRoleOperation:
					body = fmt.Sprintf(bodyFmt, CheckRoleOperation)
					ctx.Request.SetBody([]byte(body))
				default:
					m.logger.Infof("unknown probe operation: %v", string(operation))
				}
			}

			m.logger.Infof("request: %v", ctx.Request.String())
			next(ctx)
			code := ctx.Response.Header.Peek(statusCodeHeader)
			statusCode, err := strconv.Atoi(string(code))
			if err == nil {
				// header has a statusCodeHeader
				ctx.Response.Header.SetStatusCode(statusCode)
				m.logger.Infof("response abnormal: %v", ctx.Response.String())
			} else {
				// header has no statusCodeHeader
				m.logger.Infof("response: %v", ctx.Response.String())
			}
		}
	}, nil
}
