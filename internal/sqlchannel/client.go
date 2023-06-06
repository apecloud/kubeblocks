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

package sqlchannel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/cli/exec"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

type OperationClient struct {
	dapr.Client
	CharacterType    string
	cache            map[string]*OperationResult
	CacheTTL         time.Duration
	ReconcileTimeout time.Duration
	RequestTimeout   time.Duration
}

type OperationResult struct {
	response *dapr.BindingEvent
	err      error
	respTime time.Time
}

func NewClientWithPod(pod *corev1.Pod, characterType string) (*OperationClient, error) {
	if characterType == "" {
		return nil, fmt.Errorf("pod %v chacterType must be set", pod.Name)
	}

	ip := pod.Status.PodIP
	if ip == "" {
		return nil, fmt.Errorf("pod %v has no ip", pod.Name)
	}

	port, err := intctrlutil.GetProbeGRPCPort(pod)
	if err != nil {
		return nil, err
	}

	client, err := dapr.NewClientWithAddress(fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	operationClient := &OperationClient{
		Client:           client,
		CharacterType:    characterType,
		CacheTTL:         60 * time.Second,
		RequestTimeout:   30 * time.Second,
		ReconcileTimeout: 100 * time.Millisecond,
		cache:            make(map[string]*OperationResult),
	}
	return operationClient, nil
}

func (cli *OperationClient) GetRole() (string, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(context.Background(), cli.ReconcileTimeout)
	defer cancel()

	// Request sql channel via Dapr SDK
	req := &dapr.InvokeBindingRequest{
		Name:      cli.CharacterType,
		Operation: "getRole",
		Data:      []byte(""),
		Metadata:  map[string]string{},
	}

	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, req)
	if err != nil {
		return "", err
	}
	result := map[string]string{}
	err = json.Unmarshal(resp.Data, &result)
	if err != nil {
		return "", err
	}

	return result["role"], nil
}

func (cli *OperationClient) CheckStatus() (string, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(context.Background(), cli.ReconcileTimeout)
	defer cancel()

	// Request sql channel via Dapr SDK
	req := &dapr.InvokeBindingRequest{
		Name:      cli.CharacterType,
		Operation: "checkStatus",
		Data:      []byte(""),
		Metadata:  map[string]string{},
	}

	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, req)
	if err != nil {
		return "", err
	}
	result := map[string]string{}
	err = json.Unmarshal(resp.Data, &result)
	if err != nil {
		return "", err
	}

	return result["event"], nil
}

// GetSystemAccounts lists all system accounts created
func (cli *OperationClient) GetSystemAccounts() ([]string, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(context.Background(), cli.ReconcileTimeout)
	defer cancel()

	// Request sql channel via Dapr SDK
	req := &dapr.InvokeBindingRequest{
		Name:      cli.CharacterType,
		Operation: string(ListSystemAccountsOp),
	}

	if resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, req); err != nil {
		return nil, err
	} else {
		sqlResponse := SQLChannelResponse{}
		if err = json.Unmarshal(resp.Data, &sqlResponse); err != nil {
			return nil, err
		}
		if sqlResponse.Event == RespEveFail {
			return nil, fmt.Errorf("get system accounts error: %s", sqlResponse.Message)
		} else {
			result := []string{}
			if err = json.Unmarshal(([]byte)(sqlResponse.Message), &result); err != nil {
				return nil, err
			} else {
				return result, err
			}
		}
	}
}

func (cli *OperationClient) InvokeComponentInRoutine(ctxWithReconcileTimeout context.Context, req *dapr.InvokeBindingRequest) (*dapr.BindingEvent, error) {
	ch := make(chan *OperationResult, 1)
	go cli.InvokeComponent(ctxWithReconcileTimeout, req, ch)
	var resp *dapr.BindingEvent
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

func (cli *OperationClient) InvokeComponent(ctxWithReconcileTimeout context.Context, req *dapr.InvokeBindingRequest, ch chan *OperationResult) {
	ctxWithRequestTimeout, cancel := context.WithTimeout(context.Background(), cli.RequestTimeout)
	defer cancel()
	mapKey := GetMapKeyFromRequest(req)
	operationRes, ok := cli.cache[mapKey]
	if ok {
		delete(cli.cache, mapKey)
		if time.Since(operationRes.respTime) <= cli.CacheTTL {
			ch <- operationRes
			return
		}
	}

	resp, err := cli.InvokeBinding(ctxWithRequestTimeout, req)
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

func GetMapKeyFromRequest(req *dapr.InvokeBindingRequest) string {
	var buf bytes.Buffer
	buf.WriteString(req.Name)
	buf.WriteString(req.Operation)
	buf.Write(req.Data)
	keys := make([]string, 0, len(req.Metadata))
	for k := range req.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		buf.WriteString(fmt.Sprintf("%s:%s", k, req.Metadata[k]))
	}
	return buf.String()
}

// OperationHTTPClient is a mock client for operation, mainly used to hide curl command details.
type OperationHTTPClient struct {
	httpRequestPrefix string
	RequestTimeout    time.Duration
	containerName     string
	characterType     string
}

// NewHTTPClientWithChannelPod create a new OperationHTTPClient with sqlchannel container
func NewHTTPClientWithChannelPod(pod *corev1.Pod, characterType string) (*OperationHTTPClient, error) {
	var (
		err error
	)

	if characterType == "" {
		return nil, fmt.Errorf("pod %v chacterType must be set", pod.Name)
	}

	ip := pod.Status.PodIP
	if ip == "" {
		return nil, fmt.Errorf("pod %v has no ip", pod.Name)
	}
	container, err := intctrlutil.GetProbeContainerName(pod)
	if err != nil {
		return nil, err
	}
	port, err := intctrlutil.GetProbeHTTPPort(pod)
	if err != nil {
		return nil, err
	}

	client := &OperationHTTPClient{
		httpRequestPrefix: fmt.Sprintf(HTTPRequestPrefx, port, characterType),
		RequestTimeout:    10 * time.Second,
		containerName:     container,
		characterType:     characterType,
	}
	return client, nil
}

// SendRequest execs sql operation, this is a blocking operation and use pod EXEC subresource to send a http request to the probed pod
func (cli *OperationHTTPClient) SendRequest(exec *exec.ExecOptions, request SQLChannelRequest) (SQLChannelResponse, error) {
	var (
		strBuffer bytes.Buffer
		errBuffer bytes.Buffer
		err       error
		response  = SQLChannelResponse{}
	)

	if jsonData, err := json.Marshal(request); err != nil {
		return response, err
	} else {
		exec.ContainerName = cli.containerName
		exec.Command = []string{"sh", "-c", cli.httpRequestPrefix + " -d '" + string(jsonData) + "'"}
	}

	// redirect output to strBuffer to be parsed later
	if err = exec.RunWithRedirect(&strBuffer, &errBuffer); err != nil {
		return response, err
	}
	return parseResponse(strBuffer.Bytes(), request.Operation, cli.characterType)
}

type errorResponse struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

// parseResponse parses response to errorResponse or SQLChannelResponse to capture error message.
func parseResponse(data []byte, operation string, charType string) (SQLChannelResponse, error) {
	errorResponse := errorResponse{}
	response := SQLChannelResponse{}
	if err := json.Unmarshal(data, &errorResponse); err != nil {
		return response, err
	} else if len(errorResponse.ErrorCode) > 0 {
		return SQLChannelResponse{
			Event:   RespEveFail,
			Message: fmt.Sprintf("Operation `%s` on component of type `%s` is not supported yet.", operation, charType),
			Metadata: SQLChannelMeta{
				Operation: operation,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Extra:     errorResponse.Message,
			},
		}, SQLChannelError{Reason: UnsupportedOps}
	}

	// convert it to SQLChannelResponse
	err := json.Unmarshal(data, &response)
	return response, err
}
