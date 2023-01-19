/*
Copyright ApeCloud Inc.

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
	"strconv"

	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
	"github.com/valyala/fasthttp"
)

const (
	statusCheckOperation  = "statusCheck"
	runningCheckOperation = "runningCheck"
	roleCheckOperation    = "roleCheck"
	bindingPath           = "/v1.0/bindings"

	// the key is used to bypass the dapr framework and set http status code.
	// "status-code" is the key defined by probe, but this will changed like this
	// by dapr framework and http framework in the end.
	statusCodeHeader = "Metadata.status-Code"
)

type statusCodeWriter struct {
	http.ResponseWriter
	logger logger.Logger
}

// Middleware is an probe middleware.
type Middleware struct {
	logger logger.Logger
}

var _ middleware.Middleware = &Middleware{}

// NewProbeMiddleware returns a new probe middleware.
func NewProbeMiddleware(log logger.Logger) middleware.Middleware {
	return &Middleware{logger: log}
}

// GetHandler returns the HTTP handler provided by the middleware.
func (m *Middleware) GetHandler(metadata Metadata) (func(h fasthttp.RequestHandler) fasthttp.RequestHandler, error) {
	return nil, nil
}
func (scw *statusCodeWriter) WriteHeader(statusCode int) {
	header := scw.ResponseWriter.Header()
	scw.logger.Debugf("response header: %v", header)
	if v, ok := header[statusCodeHeader]; ok {
		scw.logger.Debugf("set statusCode: %v", v)
		statusCode, _ = strconv.Atoi(v[0])
		delete(header, statusCodeHeader)
	}
	scw.ResponseWriter.WriteHeader(statusCode)
}
