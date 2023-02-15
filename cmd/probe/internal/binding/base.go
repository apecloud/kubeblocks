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
	"net"
	"strings"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal"
)

const (
	CheckRunningOperation bindings.OperationKind = "checkRunning"
	CheckStatusOperation  bindings.OperationKind = "checkStatus"
	CheckRoleOperation    bindings.OperationKind = "checkRole"
	GetRoleOperation      bindings.OperationKind = "getRole"

	// CommandSQLKey keys from request's metadata.
	CommandSQLKey = "sql"

	roleEventRecordQPS            = 1. / 60.
	roleEventRecordFrequency      = int(1 / roleEventRecordQPS)
	defaultCheckFailedThreshold   = 1800
	defaultRoleDetectionThreshold = 300
)

type Operation func(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) ([]byte, error)

type BaseOperations struct {
	oriRole                 string
	dbRoles                 map[string]AccessMode
	runningCheckFailedCount int
	roleCheckFailedCount    int
	roleUnchangedCount      int
	checkFailedThreshold    int
	// roleDetectionThreshold is used to set the report duration of role event after role changed,
	// then event controller can always get rolechanged events to maintain pod label accurately
	// in cases of:
	// 1 rolechanged event lost;
	// 2 pod role label deleted or updated incorrectly.
	roleDetectionThreshold int
	DbPort                 int
	DBType                 string
	Logger                 logger.Logger
	Metadata               bindings.Metadata
	InitIfNeed             func() bool
	GetRole                func(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error)
	OperationMap           map[bindings.OperationKind]Operation
}

// ProbeOperation abstracts the interfaces a binding implementation needs to support.
// these interfaces together providing probing service: CheckRunning, statusCheck, roleCheck
type ProbeOperation interface {
	// InitIfNeed binding initiates
	InitIfNeed() error
	// GetRunningPort get binding service port.
	// runningCheck will run its check on this port
	GetRunningPort() int
	// StatusCheck TODO proposal TBD
	StatusCheck(context.Context, string, *bindings.InvokeResponse) ([]byte, error)
	// GetRole get consensus role name of the binding service
	// roleCheck will call this interface when necessary
	// return role name of the binding service
	GetRole(context.Context, string) (string, error)
}

func init() {
	viper.SetDefault("KB_CHECK_FAILED_THRESHOLD", defaultCheckFailedThreshold)
	viper.SetDefault("KB_ROLE_DETECTION_THRESHOLD", defaultRoleDetectionThreshold)
}

func (ops *BaseOperations) Init(metadata bindings.Metadata) {
	ops.checkFailedThreshold = viper.GetInt("KB_CHECK_FAILED_THRESHOLD")
	if ops.checkFailedThreshold < 300 {
		ops.checkFailedThreshold = 300
	} else if ops.checkFailedThreshold > 3600 {
		ops.checkFailedThreshold = 3600
	}

	ops.roleDetectionThreshold = viper.GetInt("KB_ROLE_DETECTION_THRESHOLD")
	if ops.roleDetectionThreshold < 60 {
		ops.roleDetectionThreshold = 60
	} else if ops.roleDetectionThreshold > 300 {
		ops.roleDetectionThreshold = 300
	}

	val := viper.GetString("KB_SERVICE_ROLES")
	if val != "" {
		if err := json.Unmarshal([]byte(val), &ops.dbRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}
	ops.Metadata = metadata
	ops.OperationMap = map[bindings.OperationKind]Operation{
		CheckRunningOperation: ops.CheckRunning,
		CheckRoleOperation:    ops.CheckRole,
	}

}

// Operations returns list of operations supported by the binding.
func (ops *BaseOperations) Operations() []bindings.OperationKind {
	opsKinds := make([]bindings.OperationKind, len(ops.OperationMap))
	i := 0
	for opsKind := range ops.OperationMap {
		opsKinds[i] = opsKind
		i++
	}
	return opsKinds
}

// Invoke handles all invoke operations.
func (ops *BaseOperations) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	if req == nil {
		return nil, errors.Errorf("invoke request required")
	}

	ops.Logger.Debugf("request operation: %v", req.Operation)
	startTime := time.Now()
	resp := &bindings.InvokeResponse{
		Metadata: map[string]string{
			RespOpKey:        string(req.Operation),
			RespSQLKey:       "test",
			RespStartTimeKey: startTime.Format(time.RFC3339Nano),
		},
	}

	updateRespMetadata := func() (*bindings.InvokeResponse, error) {
		endTime := time.Now()
		resp.Metadata[RespEndTimeKey] = endTime.Format(time.RFC3339Nano)
		resp.Metadata[RespDurationKey] = endTime.Sub(startTime).String()
		return resp, nil
	}

	operation, ok := ops.OperationMap[req.Operation]
	if !ok {
		message := fmt.Sprintf("%v operation is not implemented for %v", req.Operation, ops.DBType)
		ops.Logger.Errorf(message)
		result := &ProbeMessage{}
		result.Event = "OperationNotImplemented"
		result.Message = message
		resp.Metadata[StatusCode] = OperationFailedHTTPCode
		res, _ := json.Marshal(result)
		resp.Data = res
		return updateRespMetadata()
	}

	if ops.InitIfNeed != nil && ops.InitIfNeed() {
		resp.Data = []byte("db not ready")
		return updateRespMetadata()
	}

	d, err := operation(ctx, req, resp)
	if err != nil {
		return nil, err
	}
	resp.Data = d

	return updateRespMetadata()
}

func (ops *BaseOperations) CheckRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) ([]byte, error) {
	result := &ProbeMessage{}
	result.OriginalRole = ops.oriRole
	if ops.GetRole == nil {
		message := fmt.Sprintf("roleCheck operation is not implemented for %v", ops.DBType)
		ops.Logger.Errorf(message)
		result.Event = "OperationNotImplemented"
		result.Message = message
		response.Metadata[StatusCode] = OperationFailedHTTPCode
		res, _ := json.Marshal(result)
		return res, nil
	}

	role, err := ops.GetRole(ctx, request, response)
	if err != nil {
		ops.Logger.Infof("error executing roleCheck: %v", err)
		result.Event = "roleCheckFailed"
		result.Message = err.Error()
		if ops.roleCheckFailedCount%ops.checkFailedThreshold == 0 {
			ops.Logger.Infof("role checks failed %v times continuously", ops.roleCheckFailedCount)
			response.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		ops.roleCheckFailedCount++
		res, _ := json.Marshal(result)
		return res, nil
	}

	ops.roleCheckFailedCount = 0
	if isValid, message := ops.roleValidate(role); !isValid {
		result.Event = "roleInvalid"
		result.Message = message
		res, _ := json.Marshal(result)
		return res, nil
	}

	result.Role = role
	if ops.oriRole != role {
		result.Event = "roleChanged"
		ops.oriRole = role
		ops.roleUnchangedCount = 0
	} else {
		result.Event = "roleUnchanged"
		ops.roleUnchangedCount++
	}

	// roleUnchangedCount is the count of consecutive role unchanged checks.
	// if observed role unchanged consecutively in roleDetectionThreshold times after role changed,
	// we emit the current role again，then event controller can always get
	// roleChanged events to maintain pod label accurately in cases of:
	// 1 roleChanged event loss;
	// 2 pod role label deleted or updated incorrectly.
	if ops.roleUnchangedCount < ops.roleDetectionThreshold && ops.roleUnchangedCount%roleEventRecordFrequency == 0 {
		response.Metadata[StatusCode] = OperationFailedHTTPCode
	}
	res, _ := json.Marshal(result)
	ops.Logger.Infof(string(res))
	return res, nil
}

// DB may have some internal roles that need not be exposed to end user,
// and not configured in cluster definition, e.g. apecloud-mysql's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (ops *BaseOperations) roleValidate(role string) (bool, string) {
	// do not validate when db roles setting is missing
	if len(ops.dbRoles) == 0 {
		return true, ""
	}

	var msg string
	isValid := false
	for r := range ops.dbRoles {
		if strings.EqualFold(r, role) {
			isValid = true
			break
		}
	}
	if !isValid {
		msg = fmt.Sprintf("role %s is not configured in cluster definition %v", role, ops.dbRoles)
	}
	return isValid, msg
}

// CheckRunning checks whether the binding service is in running status:
// the port is open or is close consecutively in checkFailedThreshold times
func (ops *BaseOperations) CheckRunning(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) ([]byte, error) {
	var message string
	result := ProbeMessage{}
	marshalResult := func() ([]byte, error) {
		result.Message = message
		return json.Marshal(result)
	}

	host := fmt.Sprintf("127.0.0.1:%d", ops.DbPort)
	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
	if err != nil {
		message = fmt.Sprintf("running check %s error: %v", host, err)
		result.Event = "runningCheckFailed"
		ops.Logger.Errorf(message)
		if ops.runningCheckFailedCount++; ops.runningCheckFailedCount%ops.checkFailedThreshold == 0 {
			ops.Logger.Infof("running checks failed %v times continuously", ops.runningCheckFailedCount)
			resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		return marshalResult()
	}
	defer conn.Close()
	ops.runningCheckFailedCount = 0
	message = "TCP Connection Established Successfully!"
	if tcpCon, ok := conn.(*net.TCPConn); ok {
		err := tcpCon.SetLinger(0)
		ops.Logger.Infof("running check, set tcp linger failed: %v", err)
	}
	return marshalResult()
}
