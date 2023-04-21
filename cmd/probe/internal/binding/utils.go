/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package binding

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dapr/components-contrib/bindings"
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
	roles := []RoleType{ReadOnlyRole, ReadWriteRole, SuperUserRole}
	for _, role := range roles {
		if role.EqualTo(user.RoleName) {
			return nil
		}
	}
	return ErrInvalidRoleName
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

func SortRoleByWeight(r1, r2 RoleType) bool {
	return int(r1.GetWeight()) > int(r2.GetWeight())
}

func String2RoleType(roleName string) RoleType {
	if SuperUserRole.EqualTo(roleName) {
		return SuperUserRole
	}
	if ReadWriteRole.EqualTo(roleName) {
		return ReadWriteRole
	}
	if ReadOnlyRole.EqualTo(roleName) {
		return ReadOnlyRole
	}
	if NoPrivileges.EqualTo(roleName) {
		return NoPrivileges
	}
	return CustomizedRole
}
