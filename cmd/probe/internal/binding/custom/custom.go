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
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/mitchellh/mapstructure"
)

// HTTPSource is a binding for an http url endpoint invocation
//
//revive:disable-next-line
type HTTPSource struct {
	metadata httpMetadata
	client   *http.Client

	logger logger.Logger
}

type httpMetadata struct {
	URL string `mapstructure:"url"`
}

// NewHTTP returns a new HTTPSource.
func NewHTTP(logger logger.Logger) bindings.OutputBinding {
	return &HTTPSource{logger: logger}
}

// Init performs metadata parsing.
func (h *HTTPSource) Init(metadata bindings.Metadata) error {
	if err := mapstructure.Decode(metadata.Properties, &h.metadata); err != nil {
		return err
	}

	// See guidance on proper HTTP client settings here:
	// https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	netTransport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	h.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}

	return nil
}

// Operations returns the supported operations for this binding.
func (h *HTTPSource) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
		bindings.CreateOperation, // For backward compatibility
		"get",
		"head",
		"post",
		"put",
		"patch",
		"delete",
		"options",
		"trace",
	}
}

// Invoke performs an HTTP request to the configured HTTP endpoint.
func (h *HTTPSource) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	u := h.metadata.URL
	if req.Metadata != nil {
		if path, ok := req.Metadata["path"]; ok {
			// Simplicity and no "../../.." type exploits.
			u = fmt.Sprintf("%s/%s", strings.TrimRight(u, "/"), strings.TrimLeft(path, "/"))
			if strings.Contains(u, "..") {
				return nil, fmt.Errorf("invalid path: %s", path)
			}
		}
	}

	var body io.Reader
	method := strings.ToUpper(string(req.Operation))
	// For backward compatibility
	if method == "CREATE" {
		method = "POST"
	}
	switch method {
	case "PUT", "POST", "PATCH":
		body = bytes.NewBuffer(req.Data)
	case "GET", "HEAD", "DELETE", "OPTIONS", "TRACE":
	default:
		return nil, fmt.Errorf("invalid operation: %s", req.Operation)
	}

	request, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}

	// Set default values for Content-Type and Accept headers.
	if body != nil {
		if _, ok := req.Metadata["Content-Type"]; !ok {
			request.Header.Set("Content-Type", "application/json; charset=utf-8")
		}
	}
	if _, ok := req.Metadata["Accept"]; !ok {
		request.Header.Set("Accept", "application/json; charset=utf-8")
	}

	// Any metadata keys that start with a capital letter
	// are treated as request headers
	for mdKey, mdValue := range req.Metadata {
		keyAsRunes := []rune(mdKey)
		if len(keyAsRunes) > 0 && unicode.IsUpper(keyAsRunes[0]) {
			request.Header.Set(mdKey, mdValue)
		}
	}

	// Send the question
	resp, err := h.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body. For empty responses (e.g. 204 No Content)
	// `b` will be an empty slice.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	metadata := make(map[string]string, len(resp.Header)+2)
	// Include status code & desc
	metadata["statusCode"] = strconv.Itoa(resp.StatusCode)
	metadata["status"] = resp.Status

	// Response headers are mapped from `map[string][]string` to `map[string]string`
	// where headers with multiple values are delimited with ", ".
	for key, values := range resp.Header {
		metadata[key] = strings.Join(values, ", ")
	}

	// Create an error for non-200 status codes.
	if resp.StatusCode/100 != 2 {
		err = fmt.Errorf("received status code %d", resp.StatusCode)
	}

	return &bindings.InvokeResponse{
		Data:     b,
		Metadata: metadata,
	}, err
}
