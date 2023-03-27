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
	"time"

	"github.com/dapr/components-contrib/bindings"
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
	StartTime string                 `json:"startTime,omitempty"`
	EndTime   string                 `json:"endTime,omitempty"`
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

func GetAndFormatNow() string {
	return time.Now().Format(time.RFC3339Nano)
}

func OpsTerminateOnSucc(result OpsResult, metadata OpsMetadata, msg interface{}) (OpsResult, error) {
	metadata.EndTime = GetAndFormatNow()
	result[RespTypEve] = RespEveSucc
	result[RespTypMsg] = msg
	result[RespTypMeta] = metadata
	return result, nil
}

func OpsTerminateOnErr(result OpsResult, metadata OpsMetadata, err error) (OpsResult, error) {
	metadata.EndTime = GetAndFormatNow()
	result[RespTypEve] = RespEveFail
	result[RespTypMsg] = err.Error()
	result[RespTypMeta] = metadata
	return result, nil
}

func ExecuteObject[T UserDefinedObjectType](ctx context.Context, ops BaseInternalOps, req *bindings.InvokeRequest,
	opsKind bindings.OperationKind,
	validFn UserDefinedObjectValidator[T],
	sqlTplRend SQLRender[T], msgTplRend SQLRender[T]) (OpsResult, error) {
	var (
		result   = OpsResult{}
		userInfo = T{}
		err      error
	)

	metadata := OpsMetadata{Operation: opsKind, StartTime: GetAndFormatNow()}
	// parser userinfo from metadata
	if err = ParseObjectFromMetadata(req.Metadata, &userInfo, validFn); err != nil {
		return OpsTerminateOnErr(result, metadata, err)
	}

	sql := sqlTplRend(userInfo)
	metadata.Extra = sql
	ops.GetLogger().Debugf("MysqlOperations.execUser() with sql: %s", sql)
	if _, err = ops.InternalExec(ctx, sql); err != nil {
		return OpsTerminateOnErr(result, metadata, err)
	}
	return OpsTerminateOnSucc(result, metadata, msgTplRend(userInfo))
}

func QueryObject[T UserDefinedObjectType](ctx context.Context, ops BaseInternalOps, req *bindings.InvokeRequest,
	opsKind bindings.OperationKind,
	validFn UserDefinedObjectValidator[T],
	sqlTplRend SQLRender[T],
	dataProcessor DataRender) (OpsResult, error) {
	var (
		result   = OpsResult{}
		userInfo = T{}
		err      error
	)

	metadata := OpsMetadata{Operation: opsKind, StartTime: GetAndFormatNow()}
	// parser userinfo from metadata
	if err := ParseObjectFromMetadata(req.Metadata, &userInfo, validFn); err != nil {
		return OpsTerminateOnErr(result, metadata, err)
	}

	sql := sqlTplRend(userInfo)
	metadata.Extra = sql
	ops.GetLogger().Debugf("MysqlOperations.queryUser() with sql: %s", sql)

	jsonData, err := ops.InternalQuery(ctx, sql)
	if err != nil {
		return OpsTerminateOnErr(result, metadata, err)
	}

	if dataProcessor == nil {
		return OpsTerminateOnSucc(result, metadata, string(jsonData))
	}
	if ret, err := dataProcessor(jsonData); err != nil {
		return OpsTerminateOnErr(result, metadata, err)
	} else {
		return OpsTerminateOnSucc(result, metadata, ret)
	}
}
