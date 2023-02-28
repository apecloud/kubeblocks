/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlchannel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dapr "github.com/dapr/go-sdk/client"
	corev1 "k8s.io/api/core/v1"
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

type Order struct {
	OrderID  int     `json:"orderid"`
	Customer string  `json:"customer"`
	Price    float64 `json:"price"`
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
