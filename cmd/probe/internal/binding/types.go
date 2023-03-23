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

package binding

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dapr/components-contrib/bindings"
)

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"

	// keys from response's metadata.
	RespOpKey           = "operation"
	RespStartTimeKey    = "start-time"
	RespRowsAffectedKey = "rows-affected"
	RespEndTimeKey      = "end-time"
	RespDurationKey     = "duration"
	StatusCode          = "status-code"
	// 451 Unavailable For Legal Reasons, used to indicate operation failed.
	OperationFailedHTTPCode = "451"
	// 404 Not Found, used to indicate operation not found.
	OperationNotFoundHTTPCode = "404"

	// CommandSQLKey keys from request's metadata.
	CommandSQLKey = "sql"

	roleEventRecordQPS                = 1. / 60.
	roleEventReportFrequency          = int(1 / roleEventRecordQPS)
	defaultFailedEventReportFrequency = 1800
	defaultRoleDetectionThreshold     = 300
)

const (
	// types for probe
	CheckRunningType int = iota
	CheckStatusType
	CheckRoleChangedType
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
)

var (
	errMsgNoSQL           = "no sql provided"
	errMsgNoUserName      = "no username provided"
	errMsgNoPassword      = "no password provided"
	errMsgNoRoleName      = "no rolename provided"
	errMsgInvalidRoleName = "invalid rolename, should be one of [superuser, readwrite, readonly]"
	errMsgNoUserFound     = "no user found"

	ErrNoSQL           = fmt.Errorf(errMsgNoSQL)
	ErrNoUserName      = fmt.Errorf(errMsgNoUserName)
	ErrNoPassword      = fmt.Errorf(errMsgNoPassword)
	ErrNoRoleName      = fmt.Errorf(errMsgNoRoleName)
	ErrInvalidRoleName = fmt.Errorf(errMsgInvalidRoleName)
	ErrNoUserFound     = fmt.Errorf(errMsgNoUserFound)
)

type UserInfo struct {
	UserName string        `json:"userName"`
	Password string        `json:"password,omitempty"`
	Expired  string        `json:"expired,omitempty"`
	ExpireAt time.Duration `json:"expireAt,omitempty"`
	RoleName string        `json:"roleName,omitempty"`
}

type OpsMetadata struct {
	Operation bindings.OperationKind `json:"operation,omitempty"`
	StartTime time.Time              `json:"startTime,omitempty"`
	EndTime   time.Time              `json:"endTime,omitempty"`
	Extra     string                 `json:"extra,omitempty"`
}

// UserDefinedObjectType defines the interface for User Defined Objects.
type UserDefinedObjectType interface {
	UserInfo
}

// SQLRender defines the interface to render a SQL statement for given object.
type SQLRender[T UserDefinedObjectType] func(object T) string

// SQLPostProcessor defines what to do after retrieving results from database.
type SQLPostProcessor[T UserDefinedObjectType] func(object T) error

// UserDefinedObjectValidator defines the interface to validate the User Defined Object.
type UserDefinedObjectValidator[T UserDefinedObjectType] func(object T) error

// DataRender defines the interface to render the data from database.
type DataRender func([]byte) (interface{}, error)

func ParseObjectFromMetadata[T UserDefinedObjectType](metadata map[string]string, object *T, fn UserDefinedObjectValidator[T]) error {
	if metadata == nil {
		return fmt.Errorf("no metadata provided")
	} else if jsonData, err := json.Marshal(metadata); err != nil {
		return err
	} else if err = json.Unmarshal(jsonData, object); err != nil {
		return err
	} else if fn != nil {
		return fn(*object)
	}
	return nil
}
