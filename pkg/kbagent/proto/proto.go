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

package proto

import "time"

type Action struct {
	Name           string       `json:"name"`
	Exec           *ExecAction  `json:"exec,omitempty"`
	TimeoutSeconds int32        `json:"timeoutSeconds,omitempty"`
	RetryPolicy    *RetryPolicy `json:"retryPolicy,omitempty"`
}

type ExecAction struct {
	Commands []string `json:"command,omitempty"`
	Args     []string `json:"args,omitempty"`
}

type RetryPolicy struct {
	MaxRetries    int           `json:"maxRetries,omitempty"`
	RetryInterval time.Duration `json:"retryInterval,omitempty"`
}

type ActionRequest struct {
	Action         string            `json:"action"`
	Parameters     map[string]string `json:"parameters,omitempty"`
	NonBlocking    *bool             `json:"nonBlocking,omitempty"`
	TimeoutSeconds *int32            `json:"timeoutSeconds,omitempty"`
	RetryPolicy    *RetryPolicy      `json:"retryPolicy,omitempty"`
}

type ActionResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Output  []byte `json:"output,omitempty"`
}

// TODO: define the event spec for probe or async action

type Probe struct {
	Instance            string `json:"instance"`
	Action              string `json:"action"`
	InitialDelaySeconds int32  `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32  `json:"periodSeconds,omitempty"`
	SuccessThreshold    int32  `json:"successThreshold,omitempty"`
	FailureThreshold    int32  `json:"failureThreshold,omitempty"`
	ReportPeriodSeconds *int32 `json:"reportPeriodSeconds,omitempty"`
}

type ProbeEvent struct {
	Instance string `json:"instance"`
	Probe    string `json:"probe"`
	Code     int32  `json:"code"`
	Output   []byte `json:"output,omitempty"`  // output of the probe on success, or latest succeed output on failure
	Message  string `json:"message,omitempty"` // message of the probe on failure
}

const (
	ProbeEventFieldPath           = "spec.containers{kbagent}"
	ProbeEventReportingController = "kbagent"
	ProbeEventSourceComponent     = "kbagent"
)
