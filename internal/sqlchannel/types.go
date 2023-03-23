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
	"time"

	"github.com/dapr/components-contrib/bindings"
)

const (
	RespTypEve  = "event"
	RespTypMsg  = "message"
	RespTypData = "data"
	RespTypMeta = "metadata"

	RespEveSucc = "Success"
	RespEveFail = "Failed"

	SuperUserRole string = "superuser"
	ReadWriteRole string = "readwrite"
	ReadOnlyRole  string = "readonly"
	InvalidRole   string = "invalid"

	// actions for cluster accounts management
	ListUsersOp      bindings.OperationKind = "listUsers"
	CreateUserOp     bindings.OperationKind = "createUser"
	DeleteUserOp     bindings.OperationKind = "deleteUser"
	DescribeUserOp   bindings.OperationKind = "describeUser"
	GrantUserRoleOp  bindings.OperationKind = "grantUserRole"
	RevokeUserRoleOp bindings.OperationKind = "revokeUserRole"

	HTTPRequestPrefx string = "curl -X POST -H 'Content-Type: application/json' http://localhost:%d/v1.0/bindings/%s"
)

// UserInfo is the user information for account management
type UserInfo struct {
	UserName string        `json:"userName"`
	Password string        `json:"password,omitempty"`
	Expired  string        `json:"expired,omitempty"`
	ExpireAt time.Duration `json:"expireAt,omitempty"`
	RoleName string        `json:"roleName,omitempty"`
}

// SQLChannelRequest is the request for sqlchannel
type SQLChannelRequest struct {
	Operation string                 `json:"operation"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SQLChannelResponse is the response for sqlchannel
type SQLChannelResponse struct {
	Event    string         `json:"event,omitempty"`
	Message  string         `json:"message,omitempty"`
	Data     string         `json:"data,omitempty"`
	Metadata SQLChannelMeta `json:"metadata,omitempty"`
}

// SQLChannelMeta is the metadata for sqlchannel
type SQLChannelMeta struct {
	Operation string    `json:"operation,omitempty"`
	StartTime time.Time `json:"startTime,omitempty"`
	EndTime   time.Time `json:"endTime,omitempty"`
	Extra     string    `json:"extra,omitempty"`
}
