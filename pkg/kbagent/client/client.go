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
	"context"
	"net"
	"net/http"
	"time"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	defaultConnectTimeout = 5 * time.Second
)

type Client interface {
	Action(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error)
}

// HACK: for unit test only.
var mockClient Client
var mockClientError error

func SetMockClient(cli Client, err error) {
	mockClient = cli
	mockClientError = err
}

func UnsetMockClient() {
	mockClient = nil
	mockClientError = nil
}

func GetMockClient() Client {
	return mockClient
}

func NewClient(host string, port int32) (Client, error) {
	if mockClient != nil || mockClientError != nil {
		return mockClient, mockClientError
	}

	if host == "" && port == 0 {
		return nil, nil
	}

	// don't use default http-client
	dialer := &net.Dialer{
		Timeout: defaultConnectTimeout,
	}
	transport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: defaultConnectTimeout,
	}
	cli := &http.Client{
		// don't set timeout at client level
		// Timeout:   time.Second * 30,
		Transport: transport,
	}
	return &httpClient{
		host:   host,
		port:   port,
		client: cli,
	}, nil
}
