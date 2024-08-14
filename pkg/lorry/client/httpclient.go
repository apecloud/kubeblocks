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
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	urlTemplate = "http://%s:%d/v1.0/"
)

var NotImplemented = errors.New("NotImplemented")

type HTTPClient struct {
	lorryClient
	Client           *http.Client
	URL              string
	CacheTTL         time.Duration
	ReconcileTimeout time.Duration
	RequestTimeout   time.Duration
	logger           logr.Logger
}

var _ Client = &HTTPClient{}

type OperationResult struct {
	response *http.Response
	err      error
	reqTime  time.Time
	respTime time.Time
}

var cache map[string]*OperationResult = make(map[string]*OperationResult)

func NewHTTPClientWithPod(pod *corev1.Pod) (*HTTPClient, error) {
	logger := ctrl.Log.WithName("Lorry HTTP client")
	port, err := intctrlutil.GetLorryHTTPPort(pod)
	if err != nil {
		logger.Info("not lorry in the pod, just return nil without error")
		return nil, nil
	}

	ip := pod.Status.PodIP
	if ip == "" {
		return nil, fmt.Errorf("pod %v has no ip", pod.Name)
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

	operationClient := &HTTPClient{
		Client:           client,
		URL:              fmt.Sprintf(urlTemplate, ip, port),
		CacheTTL:         1800 * time.Second,
		RequestTimeout:   300 * time.Second,
		ReconcileTimeout: 500 * time.Millisecond,
		logger:           ctrl.Log.WithName("Lorry HTTP client"),
	}
	operationClient.lorryClient = lorryClient{requester: operationClient}
	return operationClient, nil
}

func NewHTTPClientWithURL(url string) (*HTTPClient, error) {
	if url == "" {
		return nil, fmt.Errorf("no url")
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

	operationClient := &HTTPClient{
		Client:           client,
		URL:              url,
		CacheTTL:         1800 * time.Second,
		RequestTimeout:   300 * time.Second,
		ReconcileTimeout: 500 * time.Millisecond,
	}
	operationClient.lorryClient = lorryClient{requester: operationClient}
	return operationClient, nil
}

func (cli *HTTPClient) Request(ctx context.Context, operation, method string, req map[string]any) (map[string]any, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(ctx, cli.ReconcileTimeout)
	defer cancel()

	// Request sql channel via http request
	url := fmt.Sprintf("%s%s", cli.URL, strings.ToLower(operation))

	var reader io.Reader = nil
	if req != nil {
		body, err := json.Marshal(req)
		if err != nil {
			return nil, errors.Wrap(err, "request encode failed")
		}
		reader = bytes.NewReader(body)
	}

	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, url, method, reader)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusUnavailableForLegalReasons:
		result, err := parseBody(resp.Body)
		if err != nil {
			return nil, err
		}
		if event, ok := result["event"]; ok && event.(string) == "NotImplemented" {
			return nil, NotImplemented
		}
		return result, nil

	case http.StatusNoContent:
		return nil, nil
	case http.StatusNotImplemented, http.StatusInternalServerError:
		fallthrough
	default:
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", string(msg))
	}
}

func (cli *HTTPClient) InvokeComponentInRoutine(ctxWithReconcileTimeout context.Context, url, method string, body io.Reader) (*http.Response, error) {
	ch := make(chan *OperationResult, 1)
	go cli.InvokeComponent(ctxWithReconcileTimeout, url, method, body, ch)
	var resp *http.Response
	var err error
	select {
	case <-ctxWithReconcileTimeout.Done():
		err = fmt.Errorf("request timeout: %v", ctxWithReconcileTimeout.Err())
	case result := <-ch:
		resp = result.response
		err = result.err
	}
	return resp, err
}

func (cli *HTTPClient) InvokeComponent(ctxWithReconcileTimeout context.Context, url, method string, body io.Reader, ch chan *OperationResult) {
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
	operationRes, ok := cache[mapKey]
	if ok {
		if operationRes.response != nil {
			// if the response is not nil, it means the request has been sent and the response is cached
			delete(cache, mapKey)
			if time.Since(operationRes.respTime) <= cli.CacheTTL {
				ch <- operationRes
				return
			}
			cli.logger.Info("cache expired", "url", url, "method", method, "cacheTTL", cli.CacheTTL, "since", time.Since(operationRes.respTime))
		} else {
			// if the response is nil, it means the request has been sent and not finished yet
			if time.Since(operationRes.reqTime) >= 2*cli.RequestTimeout {
				// if the request has been sent for less than 2 times of cacheTTL, and there is no the response, clean the cache and send the request again
				delete(cache, mapKey)
				cli.logger.Info("request timeout, and try again", "url", url, "method", method, "timeout", 2*cli.RequestTimeout, "since", time.Since(operationRes.reqTime))
			} else {
				ch <- operationRes
				return
			}
		}
	}
	operationRes = &OperationResult{
		err:     fmt.Errorf("request not finished"),
		reqTime: time.Now(),
	}

	cache[mapKey] = operationRes
	resp, err := cli.Client.Do(req)
	operationRes.response = resp
	operationRes.err = err
	operationRes.respTime = time.Now()
	select {
	case <-ctxWithReconcileTimeout.Done():
	default:
		delete(cache, mapKey)
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
		req.Body = io.NopCloser(bytes.NewReader(all))
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

func parseBody(body io.Reader) (map[string]any, error) {
	result := map[string]any{}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, errors.Wrap(err, "decode body failed")
	}

	return result, nil
}

func convertToArrayOfMap(value any) ([]map[string]any, error) {
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("resp errors: %v", value)
	}

	result := make([]map[string]any, 0, len(array))
	for _, v := range array {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("resp errors: %v", value)
		}
		result = append(result, m)
	}
	return result, nil
}
