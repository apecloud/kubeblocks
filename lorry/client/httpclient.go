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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/lorry/httpserver"
	. "github.com/apecloud/kubeblocks/lorry/util"
)

const (
	urlTemplate = "http://%s:%d/v1.0/"
)

type HttpClient struct {
	Client           *http.Client
	Port             int32
	URL              string
	cache            map[string]*OperationResult
	CacheTTL         time.Duration
	ReconcileTimeout time.Duration
	RequestTimeout   time.Duration
}

var _ Client = &HttpClient{}

type OperationResult struct {
	response *http.Response
	err      error
	respTime time.Time
}

func NewClientWithPod(pod *corev1.Pod) (*HttpClient, error) {
	ip := pod.Status.PodIP
	if ip == "" {
		return nil, fmt.Errorf("pod %v has no ip", pod.Name)
	}

	port, err := intctrlutil.GetLorryHTTPPort(pod)
	if err != nil {
		// not lorry in the pod, just return nil without error
		return nil, nil
	}

	// don't use default http-client
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	netTransport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}

	operationClient := &HttpClient{
		Client:           client,
		Port:             port,
		URL:              fmt.Sprintf(urlTemplate, ip, port),
		CacheTTL:         60 * time.Second,
		RequestTimeout:   30 * time.Second,
		ReconcileTimeout: 500 * time.Millisecond,
		cache:            make(map[string]*OperationResult),
	}
	return operationClient, nil
}

func (cli *HttpClient) GetRole(ctx context.Context) (string, error) {
	resp, err := cli.Request(ctx, string(GetRoleOperation), http.MethodGet, nil)
	if err != nil {
		return "", err
	}

	return resp["role"].(string), nil
}

// GetSystemAccounts lists all system accounts created
func (cli *HttpClient) GetSystemAccounts(ctx context.Context) ([]map[string]any, error) {
	resp, err := cli.Request(ctx, string(ListSystemAccountsOp), http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	return resp["users"].([]map[string]any), nil
}

// JoinMember sends a join member operation request to Lorry, located on the target pod that is about to join.
func (cli *HttpClient) JoinMember(ctx context.Context) error {
	_, err := cli.Request(ctx, string(JoinMemberOperation), http.MethodPost, nil)
	return err
}

// LeaveMember sends a Leave member operation request to Lorry, located on the target pod that is about to leave.
func (cli *HttpClient) LeaveMember(ctx context.Context) error {
	_, err := cli.Request(ctx, string(LeaveMemberOperation), http.MethodPost, nil)
	return err
}

func (cli *HttpClient) Request(ctx context.Context, operation, method string, req *httpserver.Request) (map[string]any, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(ctx, cli.ReconcileTimeout)
	defer cancel()

	// Request sql channel via http request
	url := fmt.Sprintf("%s%s", cli.URL, operation)

	var body []byte
	var err error
	if req != nil {
		body, err = json.Marshal(req)
		if err != nil {
			return nil, errors.Wrap(err, "request encode failed")
		}
	}

	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, url, method, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (cli *HttpClient) InvokeComponentInRoutine(ctxWithReconcileTimeout context.Context, url, method string, body io.Reader) (*http.Response, error) {
	ch := make(chan *OperationResult, 1)
	go cli.InvokeComponent(ctxWithReconcileTimeout, url, method, body, ch)
	var resp *http.Response
	var err error
	select {
	case <-ctxWithReconcileTimeout.Done():
		err = fmt.Errorf("invoke error : %v", ctxWithReconcileTimeout.Err())
	case result := <-ch:
		resp = result.response
		err = result.err
	}
	return resp, err
}

func (cli *HttpClient) InvokeComponent(ctxWithReconcileTimeout context.Context, url, method string, body io.Reader, ch chan *OperationResult) {
	ctxWithRequestTimeout, cancel := context.WithTimeout(context.Background(), cli.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctxWithRequestTimeout, method, url, body)
	if err != nil {
		operationRes := &OperationResult{
			response: nil,
			err:      err,
			respTime: time.Now(),
		}
		ch <- operationRes
		return
	}

	mapKey := GetMapKeyFromRequest(req)
	operationRes, ok := cli.cache[mapKey]
	if ok {
		delete(cli.cache, mapKey)
		if time.Since(operationRes.respTime) <= cli.CacheTTL {
			ch <- operationRes
			return
		}
	}

	resp, err := cli.Client.Do(req)
	operationRes = &OperationResult{
		response: resp,
		err:      err,
		respTime: time.Now(),
	}
	select {
	case <-ctxWithReconcileTimeout.Done():
		cli.cache[mapKey] = operationRes
	default:
		ch <- operationRes
	}
}

func GetMapKeyFromRequest(req *http.Request) string {
	var buf bytes.Buffer
	buf.WriteString(req.URL.String())

	if req.Body != nil {
		all, err := io.ReadAll(req.Body)
		if err != nil {
			return ""
		}
		buf.Write(all)
	}
	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		buf.WriteString(fmt.Sprintf("%s:%s", k, req.Header[k]))
	}

	return buf.String()
}
