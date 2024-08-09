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

	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	urlTemplate      = "http://%s:%d/%s"
	actionServiceURI = "/v1.0/action"
)

type httpClient struct {
	host   string
	port   int32
	client *http.Client
}

var _ Client = &httpClient{}

func (c *httpClient) CallAction(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
	url := fmt.Sprintf(urlTemplate, c.host, c.port, actionServiceURI)

	data, err := json.Marshal(req)
	if err != nil {
		return proto.ActionResponse{}, err
	}

	payload, err := c.request(ctx, fasthttp.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return proto.ActionResponse{}, err
	}
	if payload == nil {
		return proto.ActionResponse{}, nil
	}
	return c.decode(payload)
}

func (c *httpClient) request(ctx context.Context, method, url string, body io.Reader) (io.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	rsp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	switch rsp.StatusCode {
	case http.StatusOK, http.StatusUnavailableForLegalReasons:
		return rsp.Body, nil
	case http.StatusNoContent:
		return nil, nil
	case http.StatusNotImplemented, http.StatusInternalServerError:
		fallthrough
	default:
		msg, err := io.ReadAll(rsp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf(string(msg))
	}
}

func (c *httpClient) decode(body io.Reader) (proto.ActionResponse, error) {
	rsp := proto.ActionResponse{}
	data, err := io.ReadAll(body)
	if err != nil {
		return rsp, err
	}
	err = json.Unmarshal(data, &rsp)
	if err != nil {
		return rsp, err
	}
	return rsp, nil
}
