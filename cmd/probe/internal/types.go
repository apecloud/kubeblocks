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

package internal

type ProbeMessage struct {
	Event        string `json:"event,omitempty"`
	OriginalRole string `json:"originalRole,omitempty"`
	Role         string `json:"role,omitempty"`
	Message      string `json:"message,omitempty"`
}

type OpsResult map[string]interface{}
// AccessMode define SVC access mode enums.
// +enum
type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

const (
	// keys from response's metadata.
	RespOpKey           = "operation"
	RespSQLKey          = "sql"
	RespStartTimeKey    = "start-time"
	RespRowsAffectedKey = "rows-affected"
	RespEndTimeKey      = "end-time"
	RespDurationKey     = "duration"
	StatusCode          = "status-code"
	// 451 Unavailable For Legal Reasons, used to indicate check failed and trigger kubelet events
	OperationFailedHTTPCode = "451"
)
