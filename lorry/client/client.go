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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	probe2 "github.com/apecloud/kubeblocks/lorry/middleware/probe"
	. "github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/cli/exec"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	urlTemplate = "http://%s:%d/v1.0/bindings/%s"
)

type Client interface {
	// JoinMember sends a join member operation request to Lorry, located on the target pod that is about to join.
	JoinMember(ctx context.Context) error

	// LeaveMember sends a Leave member operation request to Lorry, located on the target pod that is about to leave.
	LeaveMember(ctx context.Context) error
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

func NewClient(characterType string, pod corev1.Pod) (Client, error) {
	if mockClient != nil || mockClientError != nil {
		return mockClient, mockClientError
	}
	return NewClientWithPod(&pod, characterType)
}

type OperationClient struct {
	Client           *http.Client
	Port             int32
	CharacterType    string
	URL              string
	cache            map[string]*OperationResult
	CacheTTL         time.Duration
	ReconcileTimeout time.Duration
	RequestTimeout   time.Duration
}

var _ Client = &OperationClient{}

type OperationResult struct {
	response *http.Response
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

	operationClient := &OperationClient{
		Client:           client,
		Port:             port,
		CharacterType:    characterType,
		URL:              fmt.Sprintf(urlTemplate, ip, port, characterType),
		CacheTTL:         60 * time.Second,
		RequestTimeout:   30 * time.Second,
		ReconcileTimeout: 500 * time.Millisecond,
		cache:            make(map[string]*OperationResult),
	}
	return operationClient, nil
}

func (cli *OperationClient) GetRole() (string, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(context.Background(), cli.ReconcileTimeout)
	defer cancel()

	// Http request
	url := fmt.Sprintf("%s?operation=%s", cli.URL, GetRoleOperation)

	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, url, http.MethodGet, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	result := map[string]string{}

	buf, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(buf, &result)
	if err != nil {
		return "", err
	}

	return result["role"], nil
}

// GetSystemAccounts lists all system accounts created
func (cli *OperationClient) GetSystemAccounts() ([]string, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(context.Background(), cli.ReconcileTimeout)
	defer cancel()
	url := fmt.Sprintf("%s?operation=%s", cli.URL, ListSystemAccountsOp)
	var resp *http.Response
	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, url, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	sqlResponse := SQLChannelResponse{}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &sqlResponse); err != nil {
		return nil, err
	}
	if sqlResponse.Event == RespEveFail {
		return nil, fmt.Errorf("get system accounts error: %s", sqlResponse.Message)
	}
	result := []string{}
	if err = json.Unmarshal(([]byte)(sqlResponse.Message), &result); err != nil {
		return nil, err
	}
	return result, err
}

// JoinMember sends a join member operation request to Lorry, located on the target pod that is about to join.
func (cli *OperationClient) JoinMember(ctx context.Context) error {
	_, err := cli.Request(ctx, string(JoinMemberOperation))
	return err
}

// LeaveMember sends a Leave member operation request to Lorry, located on the target pod that is about to leave.
func (cli *OperationClient) LeaveMember(ctx context.Context) error {
	_, err := cli.Request(ctx, string(LeaveMemberOperation))
	return err
}

func (cli *OperationClient) Request(ctx context.Context, operation string) (map[string]any, error) {
	ctxWithReconcileTimeout, cancel := context.WithTimeout(ctx, cli.ReconcileTimeout)
	defer cancel()

	body, err := getBodyWithOperation(operation)
	if err != nil {
		return nil, err
	}

	// Request sql channel via http request
	url := fmt.Sprintf("%s?operation=%s", cli.URL, operation)

	resp, err := cli.InvokeComponentInRoutine(ctxWithReconcileTimeout, url, http.MethodPost, body)
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

func (cli *OperationClient) InvokeComponentInRoutine(ctxWithReconcileTimeout context.Context, url, method string, body io.Reader) (*http.Response, error) {
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

func (cli *OperationClient) InvokeComponent(ctxWithReconcileTimeout context.Context, url, method string, body io.Reader, ch chan *OperationResult) {
	ctxWithRequestTimeout, cancel := context.WithTimeout(context.Background(), cli.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctxWithRequestTimeout, method, url, body)
	if err != nil || req == nil {
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
	port, err := intctrlutil.GetLorryHTTPPort(pod)
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
		// escape single quote
		data := strings.ReplaceAll(string(jsonData), "'", "\\'")
		exec.Command = []string{"sh", "-c", fmt.Sprintf("%s -d '%s'", cli.httpRequestPrefix, data)}
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

func getBodyWithOperation(operation string) (io.Reader, error) {
	meta := probe2.RequestMeta{
		Operation: operation,
		Metadata:  map[string]string{},
	}
	binary, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	body := bytes.NewReader(binary)
	return body, nil
}
