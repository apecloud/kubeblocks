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
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
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

// NewProbeMiddleware returns a new probe middleware.
func NewProbeMiddleware(log logger.Logger) middleware.Middleware {
	return &Middleware{logger: log}
}

// Middleware is an probe middleware.
type Middleware struct {
	logger logger.Logger
}

type statusCodeWriter struct {
	http.ResponseWriter
	logger logger.Logger
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

// GetHandler returns the HTTP handler provided by the middleware.
func (m *Middleware) GetHandler(metadata middleware.Metadata) (func(next http.Handler) http.Handler, error) {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL
			if r.Method == http.MethodGet && strings.HasPrefix(url.Path, bindingPath) {
				var body string
				r.Method = http.MethodPost
				r.Body.Close()
				switch operation := url.Query().Get("operation"); operation {
				case statusCheckOperation:
					body = `{"operation": "statusCheck", "metadata": {"sql" : ""}}`
					r.Body = io.NopCloser(strings.NewReader(body))
				case runningCheckOperation:
					body = `{"operation": "runningCheck", "metadata": {"sql" : ""}}`
					r.Body = io.NopCloser(strings.NewReader(body))
				case roleCheckOperation:
					body = `{"operation": "roleCheck", "metadata": {"sql" : ""}}`
					r.Body = io.NopCloser(strings.NewReader(body))
				default:
					m.logger.Infof("unknown probe operation: %v", operation)
				}
			}

			m.logger.Infof("request: %v", r)
			scw := &statusCodeWriter{ResponseWriter: w, logger: m.logger}
			next.ServeHTTP(scw, r)
		})
	}, nil
}
