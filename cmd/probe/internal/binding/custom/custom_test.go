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

package custom

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	bindingHttp "github.com/dapr/components-contrib/bindings/http"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperations(t *testing.T) {
	opers := (*bindingHttp.HTTPSource)(nil).Operations()
	assert.Equal(t, []bindings.OperationKind{
		bindings.CreateOperation,
		"get",
		"head",
		"post",
		"put",
		"patch",
		"delete",
		"options",
		"trace",
	}, opers)
}

func TestInit(t *testing.T) {
	var path string

	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			path = req.URL.Path
			input := req.Method
			if req.Body != nil {
				defer req.Body.Close()
				b, _ := io.ReadAll(req.Body)
				if len(b) > 0 {
					input = string(b)
				}
			}
			inputFromHeader := req.Header.Get("X-Input")
			if inputFromHeader != "" {
				input = inputFromHeader
			}
			w.Header().Set("Content-Type", "text/plain")
			if input == "internal server error" {
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.Write([]byte(strings.ToUpper(input)))
		}),
	)
	defer s.Close()

	m := bindings.Metadata{Base: metadata.Base{
		Properties: map[string]string{
			"url": s.URL,
		},
	}}
	hs := bindingHttp.NewHTTP(logger.NewLogger("test"))
	err := hs.Init(m)
	require.NoError(t, err)

	tests := map[string]struct {
		input     string
		operation string
		metadata  map[string]string
		path      string
		err       string
	}{
		"get": {
			input:     "GET",
			operation: "get",
			metadata:  nil,
			path:      "/",
			err:       "",
		},
		"request headers": {
			input:     "OVERRIDE",
			operation: "get",
			metadata:  map[string]string{"X-Input": "override"},
			path:      "/",
			err:       "",
		},
		"post": {
			input:     "expected",
			operation: "post",
			metadata:  map[string]string{"path": "/test"},
			path:      "/test",
			err:       "",
		},
		"put": {
			input:     "expected",
			operation: "put",
			metadata:  map[string]string{"path": "/test"},
			path:      "/test",
			err:       "",
		},
		"patch": {
			input:     "expected",
			operation: "patch",
			metadata:  map[string]string{"path": "/test"},
			path:      "/test",
			err:       "",
		},
		"delete": {
			input:     "DELETE",
			operation: "delete",
			metadata:  nil,
			path:      "/",
			err:       "",
		},
		"options": {
			input:     "OPTIONS",
			operation: "options",
			metadata:  nil,
			path:      "/",
			err:       "",
		},
		"trace": {
			input:     "TRACE",
			operation: "trace",
			metadata:  nil,
			path:      "/",
			err:       "",
		},
		"backward compatibility": {
			input:     "expected",
			operation: "create",
			metadata:  map[string]string{"path": "/test"},
			path:      "/test",
			err:       "",
		},
		"invalid path": {
			input:     "expected",
			operation: "POST",
			metadata:  map[string]string{"path": "/../test"},
			path:      "",
			err:       "invalid path: /../test",
		},
		"invalid operation": {
			input:     "notvalid",
			operation: "notvalid",
			metadata:  map[string]string{"path": "/test"},
			path:      "/test",
			err:       "invalid operation: notvalid",
		},
		"internal server error": {
			input:     "internal server error",
			operation: "post",
			metadata:  map[string]string{"path": "/"},
			path:      "/",
			err:       "received status code 500",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			response, err := hs.Invoke(context.TODO(), &bindings.InvokeRequest{
				Data:      []byte(tc.input),
				Metadata:  tc.metadata,
				Operation: bindings.OperationKind(tc.operation),
			})
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.path, path)
				assert.Equal(t, strings.ToUpper(tc.input), string(response.Data))
				assert.Equal(t, "text/plain", response.Metadata["Content-Type"])
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.err, err.Error())
			}
		})
	}
}
