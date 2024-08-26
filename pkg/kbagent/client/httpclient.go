/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	urlTemplate = "http://%s:%d%s"
)

type httpClient struct {
	host   string
	port   int32
	client *http.Client
}

var _ Client = &httpClient{}

func (c *httpClient) Action(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
	rsp := proto.ActionResponse{}

	data, err := json.Marshal(req)
	if err != nil {
		return rsp, err
	}

	url := fmt.Sprintf(urlTemplate, c.host, c.port, proto.ServiceAction.URI)
	payload, err := c.request(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return rsp, err
	}

	defer payload.Close()
	return decode(payload, &rsp)
}

func (c *httpClient) request(ctx context.Context, method, url string, body io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	rsp, err := c.client.Do(req)
	if err != nil {
		return nil, err // http error
	}

	switch rsp.StatusCode {
	case http.StatusOK, http.StatusInternalServerError:
		return rsp.Body, nil
	default:
		return nil, fmt.Errorf("unexpected http status code: %s", rsp.Status)
	}
}

func decode[T any](body io.Reader, rsp *T) (T, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return *rsp, err
	}
	err = json.Unmarshal(data, &rsp)
	if err != nil {
		return *rsp, err
	}
	return *rsp, nil
}
