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
	"net/http/httptest"
	"strconv"
	"testing"

	json "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestGetRequestBody(t *testing.T) {
	mock := make(map[string][]string)
	mock["sql"] = []string{"dd"}
	operation := "exec"
	body := getRequestBody(operation, mock)

	meta := RequestMeta{
		Operation: operation,
		Metadata:  map[string]string{},
	}
	meta.Metadata["sql"] = "dd"
	marshal, err := json.Marshal(meta)
	assert.Nil(t, err)
	assert.Equal(t, marshal, body)
}

func TestSetMiddleware(t *testing.T) {
	t.Run("Rewrite Get", func(t *testing.T) {
		mockHandler := func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, request.Method, http.MethodPost)
		}

		request := httptest.NewRequest("GET", "/v1.0/bindings", nil)
		recorder := httptest.NewRecorder()

		middleware := SetMiddleware(mockHandler)
		middleware(recorder, request)
	})

	t.Run("Rewrite StatusCode", func(t *testing.T) {

		mockHandler := func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Add(statusCodeHeader, strconv.Itoa(http.StatusNotFound))
		}

		request := httptest.NewRequest("Post", "/v1.0/bindings", nil)
		recorder := httptest.NewRecorder()

		middleware := SetMiddleware(mockHandler)
		middleware(recorder, request)

		code := recorder.Code
		assert.Equal(t, http.StatusNotFound, code)
	})

}
