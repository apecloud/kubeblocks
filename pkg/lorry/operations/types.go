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

package operations

import (
	"time"

	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

// OpsRequest is the request for Operation
type OpsRequest struct {
	Data       []byte         `json:"data,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

// OpsResponse is the response for Operation
type OpsResponse struct {
	Data     map[string]any    `json:"data,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type OpsMetadata struct {
	Operation util.OperationKind `json:"operation,omitempty"`
	StartTime string             `json:"startTime,omitempty"`
	EndTime   string             `json:"endTime,omitempty"`
	Extra     string             `json:"extra,omitempty"`
}

func getAndFormatNow() string {
	return time.Now().Format(time.RFC3339Nano)
}

func NewOpsResponse(operation util.OperationKind) *OpsResponse {
	resp := &OpsResponse{
		Data:     map[string]any{},
		Metadata: map[string]string{},
	}

	resp.Metadata["startTime"] = getAndFormatNow()
	resp.Metadata["operation"] = string(operation)
	return resp
}

func (resp *OpsResponse) WithSuccess(msg string) (*OpsResponse, error) {
	resp.Metadata["endTime"] = getAndFormatNow()
	resp.Data[util.RespFieldEvent] = util.OperationSuccess
	if msg != "" {
		resp.Data[util.RespFieldMessage] = msg
	}

	return resp, nil
}

func (resp *OpsResponse) WithError(err error) (*OpsResponse, error) {
	resp.Metadata["endTime"] = getAndFormatNow()
	resp.Data[util.RespFieldEvent] = util.OperationFailed
	resp.Data[util.RespFieldMessage] = err.Error()
	return resp, err
}
