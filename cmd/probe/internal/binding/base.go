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
	"strconv"
	"strings"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

type Operation func(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (OpsResult, error)

type OpsResult map[string]interface{}

// AccessMode define SVC access mode enums.
// +enum
type AccessMode string

type BaseInternalOps interface {
	InternalQuery(ctx context.Context, sql string) ([]byte, error)
	InternalExec(ctx context.Context, sql string) (int64, error)
	GetLogger() logger.Logger
}

type BaseOperations struct {
	CheckRunningFailedCount    int
	CheckStatusFailedCount     int
	CheckRoleFailedCount       int
	RoleUnchangedCount         int
	FailedEventReportFrequency int
	// RoleDetectionThreshold is used to set the report duration of role event after role changed,
	// then event controller can always get rolechanged events to maintain pod label accurately
	// in cases of:
	// 1 rolechanged event lost;
	// 2 pod role label deleted or updated incorrectly.
	RoleDetectionThreshold int
	DBPort                 int
	DBAddress              string
	DBType                 string
	OriRole                string
	DBRoles                map[string]AccessMode
	Logger                 logger.Logger
	Metadata               bindings.Metadata
	InitIfNeed             func() bool
	GetRole                func(context.Context, *bindings.InvokeRequest, *bindings.InvokeResponse) (string, error)
	OperationMap           map[bindings.OperationKind]Operation
}

func init() {
	viper.SetDefault("KB_FAILED_EVENT_REPORT_FREQUENCY", defaultFailedEventReportFrequency)
	viper.SetDefault("KB_ROLE_DETECTION_THRESHOLD", defaultRoleDetectionThreshold)
}

func (ops *BaseOperations) Init(metadata bindings.Metadata) {
	ops.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if ops.FailedEventReportFrequency < 300 {
		ops.FailedEventReportFrequency = 300
	} else if ops.FailedEventReportFrequency > 3600 {
		ops.FailedEventReportFrequency = 3600
	}

	ops.RoleDetectionThreshold = viper.GetInt("KB_ROLE_DETECTION_THRESHOLD")
	if ops.RoleDetectionThreshold < 60 {
		ops.RoleDetectionThreshold = 60
	} else if ops.RoleDetectionThreshold > 300 {
		ops.RoleDetectionThreshold = 300
	}

	val := viper.GetString("KB_SERVICE_ROLES")
	if val != "" {
		if err := json.Unmarshal([]byte(val), &ops.DBRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}
	ops.Metadata = metadata
	ops.OperationMap = map[bindings.OperationKind]Operation{
		CheckRunningOperation: ops.CheckRunningOps,
		CheckRoleOperation:    ops.CheckRoleOps,
		GetRoleOperation:      ops.GetRoleOps,
	}
	ops.DBAddress = ops.getAddress()
}

func (ops *BaseOperations) RegisterOperation(opsKind bindings.OperationKind, operation Operation) {
	if ops.OperationMap == nil {
		ops.OperationMap = map[bindings.OperationKind]Operation{}
	}
	ops.OperationMap[opsKind] = operation
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

// getAddress returns component service address, if component is not listening on
// 127.0.0.1, the Operation needs to overwrite this function and set ops.DBAddress
func (ops *BaseOperations) getAddress() string {
	return "127.0.0.1"
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
	opsRes := OpsResult{}
	if !ok {
		message := fmt.Sprintf("%v operation is not implemented for %v", req.Operation, ops.DBType)
		ops.Logger.Errorf(message)
		opsRes["event"] = OperationNotImplemented
		opsRes["message"] = message
		resp.Metadata[StatusCode] = OperationNotFoundHTTPCode
		res, _ := json.Marshal(opsRes)
		resp.Data = res
		return updateRespMetadata()
	}

	if ops.InitIfNeed != nil && ops.InitIfNeed() {
		opsRes["event"] = OperationFailed
		opsRes["message"] = "db not ready"
		res, _ := json.Marshal(opsRes)
		resp.Data = res
		return updateRespMetadata()
	}

	opsRes, err := operation(ctx, req, resp)
	if err != nil {
		return nil, err
	}
	res, _ := json.Marshal(opsRes)
	resp.Data = res

	return updateRespMetadata()
}

func (ops *BaseOperations) CheckRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	opsRes["originalRole"] = ops.OriRole
	if ops.GetRole == nil {
		message := fmt.Sprintf("roleCheck operation is not implemented for %v", ops.DBType)
		ops.Logger.Errorf(message)
		opsRes["event"] = OperationNotImplemented
		opsRes["message"] = message
		resp.Metadata[StatusCode] = OperationNotFoundHTTPCode
		return opsRes, nil
	}

	role, err := ops.GetRole(ctx, req, resp)
	if err != nil {
		ops.Logger.Infof("error executing roleCheck: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if ops.CheckRoleFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Infof("role checks failed %v times continuously", ops.CheckRoleFailedCount)
			resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		ops.CheckRoleFailedCount++
		return opsRes, nil
	}

	ops.CheckRoleFailedCount = 0
	if isValid, message := ops.roleValidate(role); !isValid {
		opsRes["event"] = OperationInvalid
		opsRes["message"] = message
		return opsRes, nil
	}

	opsRes["event"] = OperationSuccess
	opsRes["role"] = role
	if ops.OriRole != role {
		ops.OriRole = role
		ops.RoleUnchangedCount = 0
	} else {
		ops.RoleUnchangedCount++
	}

	// RoleUnchangedCount is the count of consecutive role unchanged checks.
	// If the role remains unchanged consecutively in RoleDetectionThreshold checks after it has changed,
	// then the roleCheck event will be reported at roleEventReportFrequency so that the event controller
	// can always get relevant roleCheck events in order to maintain the pod label accurately, even in cases
	// of roleChanged events being lost or the pod role label being deleted or updated incorrectly.
	if ops.RoleUnchangedCount < ops.RoleDetectionThreshold && ops.RoleUnchangedCount%roleEventReportFrequency == 0 {
		resp.Metadata[StatusCode] = OperationFailedHTTPCode
	}
	return opsRes, nil
}

func (ops *BaseOperations) GetRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	if ops.GetRole == nil {
		message := fmt.Sprintf("roleCheck operation is not implemented for %v", ops.DBType)
		ops.Logger.Errorf(message)
		opsRes["event"] = OperationNotImplemented
		opsRes["message"] = message
		resp.Metadata[StatusCode] = OperationNotFoundHTTPCode
		return opsRes, nil
	}

	role, err := ops.GetRole(ctx, req, resp)
	if err != nil {
		ops.Logger.Infof("error executing roleCheck: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if ops.CheckRoleFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Infof("role checks failed %v times continuously", ops.CheckRoleFailedCount)
			resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		ops.CheckRoleFailedCount++
		return opsRes, nil
	}
	opsRes["event"] = OperationSuccess
	opsRes["role"] = role
	return opsRes, nil
}

// Component may have some internal roles that need not be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (ops *BaseOperations) roleValidate(role string) (bool, string) {
	// do not validate when db roles setting is missing
	if len(ops.DBRoles) == 0 {
		return true, ""
	}

	var msg string
	isValid := false
	for r := range ops.DBRoles {
		if strings.EqualFold(r, role) {
			isValid = true
			break
		}
	}
	if !isValid {
		msg = fmt.Sprintf("role %s is not configured in cluster definition %v", role, ops.DBRoles)
	}
	return isValid, msg
}

// CheckRunningOps checks whether the binding service is in running status,
// If check fails continuously, report an event at FailedEventReportFrequency frequency
func (ops *BaseOperations) CheckRunningOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var message string
	opsRes := OpsResult{}

	host := net.JoinHostPort(ops.DBAddress, strconv.Itoa(ops.DBPort))
	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
	if err != nil {
		message = fmt.Sprintf("running check %s error: %v", host, err)
		ops.Logger.Errorf(message)
		opsRes["event"] = OperationFailed
		opsRes["message"] = message
		if ops.CheckRunningFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Infof("running checks failed %v times continuously", ops.CheckRunningFailedCount)
			resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		ops.CheckRunningFailedCount++
		return opsRes, nil
	}
	defer conn.Close()
	ops.CheckRunningFailedCount = 0
	message = "TCP Connection Established Successfully!"
	if tcpCon, ok := conn.(*net.TCPConn); ok {
		err := tcpCon.SetLinger(0)
		ops.Logger.Infof("running check, set tcp linger failed: %v", err)
	}
	opsRes["event"] = OperationSuccess
	opsRes["message"] = message
	return opsRes, nil
}
