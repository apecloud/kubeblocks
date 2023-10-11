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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
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

var logger logr.Logger

func init() {
	logger = ctrl.Log.WithName("middleware")
}

func getRequestBody(operation string, args map[string][]string) []byte {
	metadata := make(map[string]string)
	walkFunc := func(key string, value []string) {
		if key == operationKey {
			return
		}
		if len(value) == 1 {
			metadata[key] = value[0]
		} else {
			marshal, err := json.Marshal(value)
			if err != nil {
				logger.Error(err, "getRequestBody marshal json error")
				return
			}
			metadata[key] = string(marshal)
		}
	}

	for k, v := range args {
		walkFunc(k, v)
	}

	requestMeta := RequestMeta{
		Operation: operation,
		Metadata:  metadata,
	}

	body, _ := json.Marshal(requestMeta)
	return body
}

func SetMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		uri := request.URL
		method := request.Method

		if method == http.MethodGet && strings.HasPrefix(uri.Path, bindingPath) {
			request.Method = http.MethodPost

			operation := uri.Query().Get(operationKey)
			if strings.HasPrefix(operation, "get") || strings.HasPrefix(operation, "check") || strings.HasPrefix(operation, "list") {
				body := getRequestBody(operation, uri.Query())
				request.Body = io.NopCloser(bytes.NewReader(body))
			} else {
				logger.Info("unknown probe operation", "operation", operation)
			}
		}

		logger.Info("receive request", "request", request.RequestURI)
		next(writer, request)
		code := writer.Header().Get(statusCodeHeader)
		statusCode, err := strconv.Atoi(code)
		if err == nil {
			// header has a statusCodeHeader
			writer.WriteHeader(statusCode)
			logger.Info("write response with header", "statusCode", statusCode)
		} else {
			// header has no statusCodeHeader
			logger.Info("response has no statusCodeHeader")
		}
	}
}
