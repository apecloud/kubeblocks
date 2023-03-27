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
	"fmt"
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
	RespTypMeta = "metadata"

	RespEveSucc = "Success"
	RespEveFail = "Failed"

	SuperUserRole  string = "superuser"
	ReadWriteRole  string = "readwrite"
	ReadOnlyRole   string = "readonly"
	NoPrivileges   string = ""
	CustomizedRole string = "customized"
	InvalidRole    string = "invalid"
)

const (
	errMsgNoSQL           = "no sql provided"
	errMsgNoUserName      = "no username provided"
	errMsgNoPassword      = "no password provided"
	errMsgNoRoleName      = "no rolename provided"
	errMsgInvalidRoleName = "invalid rolename, should be one of [superuser, readwrite, readonly]"
	errMsgNoSuchUser      = "no such user"
)

var (
	ErrNoSQL           = fmt.Errorf(errMsgNoSQL)
	ErrNoUserName      = fmt.Errorf(errMsgNoUserName)
	ErrNoPassword      = fmt.Errorf(errMsgNoPassword)
	ErrNoRoleName      = fmt.Errorf(errMsgNoRoleName)
	ErrInvalidRoleName = fmt.Errorf(errMsgInvalidRoleName)
	ErrNoSuchUser      = fmt.Errorf(errMsgNoSuchUser)
)
