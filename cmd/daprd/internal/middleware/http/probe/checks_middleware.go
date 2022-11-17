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
	"strings"
	"io"

	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
)

// NewProbeMiddleware returns a new probe middleware.
func NewProbeMiddleware(log logger.Logger) middleware.Middleware {
	return &Middleware{logger: log}
}

// Middleware is an probe middleware.
type Middleware struct {
	logger logger.Logger
}

// GetHandler returns the HTTP handler provided by the middleware.
func (m *Middleware) GetHandler(metadata middleware.Metadata) (func(next http.Handler) http.Handler, error) {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL
			if r.Method == http.MethodGet && strings.HasPrefix(url.Path, "/v1.0/bindings") {
				r.Method = http.MethodPost
				var body string
				args := url.Query()
				switch operation := args.Get("operation"); operation {
				case "statusCheck":
					body = `{"operation": "statusCheck", "metadata": {"sql" : ""}}`
					r.Body = io.NopCloser(strings.NewReader(body))
				case "runningCheck":
					body = `{"operation": "runningCheck", "metadata": {"sql" : ""}}`
					r.Body = io.NopCloser(strings.NewReader(body))
				case "roleCheck":
					body = `{"operation": "roleCheck", "metadata": {"sql" : ""}}`
					r.Body = io.NopCloser(strings.NewReader(body))
				default:
					m.logger.Infof("unknown probe operation: %v", operation)
				}
			}

			m.logger.Infof("request: %v", r)
			next.ServeHTTP(w, r)
		})
	}, nil
}
