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

package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dapr "github.com/dapr/go-sdk/client"
	corev1 "k8s.io/api/core/v1"
)

type OperationClient struct {
	dapr.Client
	CharacterType string
}

type Order struct {
	OrderID  int     `json:"orderid"`
	Customer string  `json:"customer"`
	Price    float64 `json:"price"`
}

func NewClientWithPod(pod *corev1.Pod, characterType string) (*OperationClient, error) {
	if characterType == "" {
		return nil, fmt.Errorf("pod %v chacterType must be set", pod)
	}

	ip := pod.Status.PodIP
	if ip == "" {
		return nil, fmt.Errorf("pod %v has no ip", pod)
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
		Client:        client,
		CharacterType: characterType,
	}
	return operationClient, nil
}

func (cli *OperationClient) GetRole() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Request sql channel via Dapr SDK
	in := &dapr.InvokeBindingRequest{
		Name:      cli.CharacterType,
		Operation: "getRole",
		Data:      []byte(""),
		Metadata:  map[string]string{},
	}
	resp, err := cli.InvokeBinding(ctx, in)
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
