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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"
)

const checkFailedHTTPCode = "451"

// mockedRequestHandler acts like an upstream service returns success status code 200 and a fixed response body.
func mockedRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("mock response"))
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
		r := httptest.NewRequest(http.MethodGet,
			"http://localhost:3501/v1.0/bindings/probe?operation=statuscheck", nil)
		w := httptest.NewRecorder()

		handler(http.HandlerFunc(mockedRequestHandler)).ServeHTTP(w, r)
		//r.Body.{io.(strings.Reader).UnreadRune()
		_, err := io.ReadAll(r.Body)
		assert.Nil(t, err)
		result := w.Result()
		assert.Equal(t, http.StatusOK, result.StatusCode)
		assert.Equal(t, http.MethodPost, r.Method)
		result.Body.Close()
	})

	t.Run("hit: status code handler", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet,
			"http://localhost:3501/v1.0/bindings/probe?operation=statuscheck", nil)
		w := httptest.NewRecorder()
		header := w.Header()
		header.Add(statusCodeHeader, checkFailedHTTPCode)
		handler(http.HandlerFunc(mockedRequestHandler)).ServeHTTP(w, r)
		//r.Body.{io.(strings.Reader).UnreadRune()
		_, err := io.ReadAll(r.Body)
		assert.Nil(t, err)
		result := w.Result()
		assert.Equal(t, 451, result.StatusCode)
		assert.Equal(t, http.MethodPost, r.Method)
		result.Body.Close()
	})
}
