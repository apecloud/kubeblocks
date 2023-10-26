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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/cli/exec"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	. "github.com/apecloud/kubeblocks/pkg/lorry/util"
)

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
	container, err := intctrlutil.GetLorryContainerName(pod)
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

// SendRequest execs sql operation, this is a blocking operation and use pod EXEC subresource to send a http request to the lorry pod
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
