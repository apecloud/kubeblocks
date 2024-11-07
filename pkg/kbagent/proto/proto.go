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
	Output   []byte `json:"output,omitempty"`  // output of the probe on success, or latest succeed output on failure
	Message  string `json:"message,omitempty"` // message of the probe on failure
}

type Task struct {
	Instance            string          `json:"instance"`
	Task                string          `json:"task"`
	UID                 string          `json:"UID"`                           // the unique identifier of the task
	Replicas            string          `json:"replicas"`                      // target replicas to run the task
	Payload             string          `json:"payload"`                       // the payload for the specific action
	NotifyAtFinish      bool            `json:"notifyAtFinish,omitempty"`      // whether to notify the controller when the task is finished
	ReportPeriodSeconds int32           `json:"reportPeriodSeconds,omitempty"` // the period to report the progress of the task
	NewReplica          *NewReplicaTask `json:"newReplica,omitempty"`
}

type TaskEvent struct {
	Instance  string    `json:"instance"`
	Task      string    `json:"task"`
	UID       string    `json:"UID"`
	Replica   string    `json:"replica"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Code      int32     `json:"code"`
	Output    []byte    `json:"output,omitempty"`  // output of the task on success
	Message   string    `json:"message,omitempty"` // message of the task on failure
}

type NewReplicaTask struct {
	Remote         string            `json:"remote"` // the remote address of the data source
	Port           int32             `json:"port"`
	Replicas       string            `json:"replicas"`             // replicas to load the data
	Parameters     map[string]string `json:"parameters,omitempty"` // parameters for data dump and load
	TimeoutSeconds *int32            `json:"timeoutSeconds,omitempty"`
}
