/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type actionCall struct {
	requestFingerprint string
	running            bool
	result             *actionResult
}

type actionResult struct {
	output []byte
	err    error
}

func newActionResult(output []byte, err error) *actionResult {
	return &actionResult{
		output: append([]byte(nil), output...),
		err:    err,
	}
}

func (r *actionResult) response() ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	if r.err != nil {
		return nil, r.err
	}
	return append([]byte(nil), r.output...), nil
}

type actionRequestFingerprintInput struct {
	Action         string                       `json:"action"`
	Parameters     map[string]string            `json:"parameters"`
	Arguments      [][]string                   `json:"arguments"`
	TimeoutSeconds int32                        `json:"timeoutSeconds"`
	RetryPolicy    *retryPolicyFingerprintInput `json:"retryPolicy"`
}

type retryPolicyFingerprintInput struct {
	MaxRetries    int   `json:"maxRetries"`
	RetryInterval int64 `json:"retryInterval"`
}

func fingerprintActionRequest(req *proto.ActionRequest, timeout *int32, retryPolicy *proto.RetryPolicy) (string, error) {
	parameters := req.Parameters
	if parameters == nil {
		parameters = map[string]string{}
	}
	arguments := req.Arguments
	if len(arguments) == 0 {
		arguments = [][]string{}
	} else {
		arguments = append([][]string(nil), arguments...)
		for i, argument := range arguments {
			if argument == nil {
				arguments[i] = []string{}
			}
		}
	}
	var retry *retryPolicyFingerprintInput
	if retryPolicy != nil && retryPolicy.MaxRetries > 0 {
		retry = &retryPolicyFingerprintInput{
			MaxRetries:    retryPolicy.MaxRetries,
			RetryInterval: int64(max(retryPolicy.RetryInterval, 0)),
		}
	}
	input := actionRequestFingerprintInput{
		Action:         req.Action,
		Parameters:     parameters,
		Arguments:      arguments,
		TimeoutSeconds: effectiveTimeoutSeconds(timeout),
		RetryPolicy:    retry,
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "", errors.Wrap(err, "marshal Action request fingerprint input")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func effectiveTimeoutSeconds(timeout *int32) int32 {
	if timeout == nil || *timeout == 0 {
		return int32(defaultActionCallTimeout / time.Second)
	}
	if *timeout < 0 {
		return -1
	}
	return *timeout
}
