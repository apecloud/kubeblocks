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

import (
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
)

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

const (
	ProbeEventFieldPath           = "spec.containers{kbagent}"
	ProbeEventReportingController = "kbagent"
	ProbeEventSourceComponent     = "kbagent"
)

type Probe struct {
	Instance            string `json:"instance"`
	Action              string `json:"action"`
	InitialDelaySeconds int32  `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32  `json:"periodSeconds,omitempty"`
	SuccessThreshold    int32  `json:"successThreshold,omitempty"`
	FailureThreshold    int32  `json:"failureThreshold,omitempty"`
	ReportPeriodSeconds int32  `json:"reportPeriodSeconds,omitempty"`
}

type ProbeEvent struct {
	Instance string `json:"instance"`
	Probe    string `json:"probe"`
	Code     int32  `json:"code"`

	// output of the probe on success, or latest succeed output on failure
	Output []byte `json:"output,omitempty"`

	// message of the probe on failure
	Message string `json:"message,omitempty"`
}

type Task struct {
	// the unique identifier of the task
	UID string `json:"UID"`

	// whether to notify the controller when the task is finished
	NotifyAtFinish *bool `json:"notifyAtFinish,omitempty"`

	// the period to report the progress of the task
	ReportPeriodSeconds *int32 `json:"reportPeriodSeconds,omitempty"`

	DataLoad *DataLoadTask `json:"dataLoad,omitempty"`
}

type TaskEvent struct {
	Instance string `json:"instance"`

	Code  int32  `json:"code"`
	Error string `json:"error,omitempty"`

	// output of the task on success
	Output []byte `json:"output,omitempty"`

	// message of the task on failure
	Message string `json:"message,omitempty"`

	DataLoad *DataLoadEvent `json:"dataLoad,omitempty"`
}

type DataLoadTask struct {
	// the remote address of the data source
	Remote string `json:"remote"`

	Port *int32 `json:"port,omitempty"`

	// replicas to load the data
	Replicas string `json:"replicas"`

	// parameters for data dump and load
	Parameters     map[string]string `json:"parameters,omitempty"`
	TimeoutSeconds *int32            `json:"timeoutSeconds,omitempty"`
}

type DataLoadEvent struct {
	UID       string             `json:"UID"`
	StartTime time.Time          `json:"startTime"`
	EndTime   time.Time          `json:"endTime"`
	Progress  intstr.IntOrString `json:"progress"`
}
