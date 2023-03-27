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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"golang.org/x/exp/slices"
)

type UserInfo struct {
	UserName string `json:"userName"`
	Password string `json:"password,omitempty"`
	Expired  string `json:"expired,omitempty"`
	RoleName string `json:"roleName,omitempty"`
}

type RedisEntry struct {
	Key  string `json:"key"`
	Data []byte `json:"data,omitempty"`
}

type opsMetadata struct {
	Operation bindings.OperationKind `json:"operation,omitempty"`
	StartTime string                 `json:"startTime,omitempty"`
	EndTime   string                 `json:"endTime,omitempty"`
	Extra     string                 `json:"extra,omitempty"`
}

// UserDefinedObjectType defines the interface for User Defined Objects.
type customizedObjType interface {
	UserInfo | RedisEntry
}

// CmdRender defines the interface to render a statement from given object.
type cmdRender[T customizedObjType] func(object T) string

// resultRender defines the interface to render the data from database.
type resultRender[T customizedObjType] func(interface{}) (interface{}, error)

// objectValidator defines the interface to validate the User Defined Object.
type objectValidator[T customizedObjType] func(object T) error

// objectParser defines the interface to parse the User Defined Object from request.
type objectParser[T customizedObjType] func(req *bindings.InvokeRequest, object *T) error

func ExecuteObject[T customizedObjType](ctx context.Context, ops BaseInternalOps, req *bindings.InvokeRequest,
	opsKind bindings.OperationKind, sqlTplRend cmdRender[T], msgTplRend cmdRender[T], object T) (OpsResult, error) {
	var (
		result = OpsResult{}
		err    error
	)

	metadata := opsMetadata{Operation: opsKind, StartTime: getAndFormatNow()}

	sql := sqlTplRend(object)
	metadata.Extra = sql
	ops.GetLogger().Debugf("ExecObject with cmd: %s", sql)

	if _, err = ops.InternalExec(ctx, sql); err != nil {
		return opsTerminateOnErr(result, metadata, err)
	}
	return opsTerminateOnSucc(result, metadata, msgTplRend(object))
}

func QueryObject[T customizedObjType](ctx context.Context, ops BaseInternalOps, req *bindings.InvokeRequest,
	opsKind bindings.OperationKind, sqlTplRend cmdRender[T], dataProcessor resultRender[T], object T) (OpsResult, error) {
	var (
		result = OpsResult{}
		err    error
	)

	metadata := opsMetadata{Operation: opsKind, StartTime: getAndFormatNow()}

	sql := sqlTplRend(object)
	metadata.Extra = sql
	ops.GetLogger().Debugf("QueryObject() with cmd: %s", sql)

	jsonData, err := ops.InternalQuery(ctx, sql)
	if err != nil {
		return opsTerminateOnErr(result, metadata, err)
	}

	if dataProcessor == nil {
		return opsTerminateOnSucc(result, metadata, string(jsonData))
	}

	if ret, err := dataProcessor(jsonData); err != nil {
		return opsTerminateOnErr(result, metadata, err)
	} else {
		return opsTerminateOnSucc(result, metadata, ret)
	}
}

func ParseObjFromRequest[T customizedObjType](req *bindings.InvokeRequest, parse objectParser[T], validator objectValidator[T], object *T) error {
	if req == nil {
		return fmt.Errorf("no request provided")
	}
	if parse != nil {
		if err := parse(req, object); err != nil {
			return err
		}
	}
	if validator != nil {
		if err := validator(*object); err != nil {
			return err
		}
	}
	return nil
}

func DefaultUserInfoParser(req *bindings.InvokeRequest, object *UserInfo) error {
	if req == nil || req.Metadata == nil {
		return fmt.Errorf("no metadata provided")
	} else if jsonData, err := json.Marshal(req.Metadata); err != nil {
		return err
	} else if err = json.Unmarshal(jsonData, object); err != nil {
		return err
	}
	return nil
}

func UserNameValidator(user UserInfo) error {
	if len(user.UserName) == 0 {
		return ErrNoUserName
	}
	return nil
}

func UserNameAndPasswdValidator(user UserInfo) error {
	if len(user.UserName) == 0 {
		return ErrNoUserName
	}
	if len(user.Password) == 0 {
		return ErrNoPassword
	}
	return nil
}

func UserNameAndRoleValidator(user UserInfo) error {
	if len(user.UserName) == 0 {
		return ErrNoUserName
	}
	if len(user.RoleName) == 0 {
		return ErrNoRoleName
	}
	roles := []string{ReadOnlyRole, ReadWriteRole, SuperUserRole}
	if !slices.Contains(roles, strings.ToLower(user.RoleName)) {
		return ErrInvalidRoleName
	}
	return nil
}

func getAndFormatNow() string {
	return time.Now().Format(time.RFC3339Nano)
}

func opsTerminateOnSucc(result OpsResult, metadata opsMetadata, msg interface{}) (OpsResult, error) {
	metadata.EndTime = getAndFormatNow()
	result[RespTypEve] = RespEveSucc
	result[RespTypMsg] = msg
	result[RespTypMeta] = metadata
	return result, nil
}

func opsTerminateOnErr(result OpsResult, metadata opsMetadata, err error) (OpsResult, error) {
	metadata.EndTime = getAndFormatNow()
	result[RespTypEve] = RespEveFail
	result[RespTypMsg] = err.Error()
	result[RespTypMeta] = metadata
	return result, nil
}
