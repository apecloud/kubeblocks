/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/dcs"
	. "github.com/apecloud/kubeblocks/lorry/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type Operation func(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error)

type OpsResult map[string]interface{}

// AccessMode defines SVC access mode enums.
// +enum
type AccessMode string

type BaseInternalOps interface {
	InternalQuery(ctx context.Context, sql string) ([]byte, error)
	InternalExec(ctx context.Context, sql string) (int64, error)
	GetLogger() logr.Logger
	GetRunningPort() int
	Invoke(ctx context.Context, req *ProbeRequest) (*ProbeResponse, error)
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
	Logger                 logr.Logger
	Metadata               map[string]string
	InitIfNeed             func() bool
	Manager                component.DBManager
	GetRole                func(context.Context, *ProbeRequest, *ProbeResponse) (string, error)

	OperationsMap map[OperationKind]Operation
}

func init() {
	viper.SetDefault("KB_FAILED_EVENT_REPORT_FREQUENCY", defaultFailedEventReportFrequency)
	viper.SetDefault("KB_ROLE_DETECTION_THRESHOLD", defaultRoleDetectionThreshold)
}

func (ops *BaseOperations) Init(properties component.Properties) {
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
	ops.Metadata = properties
	ops.OperationsMap = map[OperationKind]Operation{
		CheckRunningOperation: ops.CheckRunningOps,
		CheckRoleOperation:    ops.CheckRoleOps,
		GetRoleOperation:      ops.GetRoleOps,
		VolumeProtection:      ops.VolumeProtectionOps,
		SwitchoverOperation:   ops.SwitchoverOps,
		LockOperation:         ops.LockOps,
		UnlockOperation:       ops.UnlockOps,
		JoinMemberOperation:   ops.JoinMemberOps,
		LeaveMemberOperation:  ops.LeaveMemberOps,
	}

	ops.DBAddress = ops.getAddress()
}

func (ops *BaseOperations) RegisterOperation(opsKind OperationKind, operation Operation) {
	if ops.OperationsMap == nil {
		ops.OperationsMap = map[OperationKind]Operation{}
	}
	ops.OperationsMap[opsKind] = operation
}

func (ops *BaseOperations) RegisterOperationOnDBReady(opsKind OperationKind, operation Operation, manager component.DBManager) {
	ops.RegisterOperation(opsKind, StartupCheckWraper(manager, operation))
}

// Operations returns list of operations supported by the binding.
func (ops *BaseOperations) Operations() []OperationKind {
	opsKinds := make([]OperationKind, len(ops.OperationsMap))
	i := 0
	for opsKind := range ops.OperationsMap {
		opsKinds[i] = opsKind
		i++
	}
	return opsKinds
}

// getAddress returns component service address, if component is not listening on
// 127.0.0.1, the LegacyOperation needs to overwrite this function and set ops.DBAddress
func (ops *BaseOperations) getAddress() string {
	return "127.0.0.1"
}

// Invoke handles all invoke operations.
func (ops *BaseOperations) Invoke(ctx context.Context, req *ProbeRequest) (*ProbeResponse, error) {
	if req == nil {
		return nil, errors.Errorf("invoke request required")
	}

	startTime := time.Now()
	resp := &ProbeResponse{
		Metadata: map[string]string{
			RespOpKey:        string(req.Operation),
			RespStartTimeKey: startTime.Format(time.RFC3339Nano),
		},
	}

	updateRespMetadata := func() (*ProbeResponse, error) {
		endTime := time.Now()
		resp.Metadata[RespEndTimeKey] = endTime.Format(time.RFC3339Nano)
		resp.Metadata[RespDurationKey] = endTime.Sub(startTime).String()
		return resp, nil
	}

	operation, ok := ops.OperationsMap[req.Operation]
	opsRes := OpsResult{}
	if !ok {
		message := fmt.Sprintf("%v operation is not implemented for %v", req.Operation, ops.DBType)
		ops.Logger.Error(nil, message)
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
	ops.Logger.Info("operation called", "operation", req.Operation, "result", opsRes)
	if opsRes != nil {
		res, _ := json.Marshal(opsRes)
		resp.Data = res
	}

	return updateRespMetadata()
}

func (ops *BaseOperations) CheckRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	opsRes["operation"] = CheckRoleOperation
	opsRes["originalRole"] = ops.OriRole
	if ops.GetRole == nil {
		message := fmt.Sprintf("checkRole operation is not implemented for %v", ops.DBType)
		ops.Logger.Info(message)
		opsRes["event"] = OperationNotImplemented
		opsRes["message"] = message
		resp.Metadata[StatusCode] = OperationNotFoundHTTPCode
		return opsRes, nil
	}

	timeoutSeconds := defaultRoleProbeTimeoutSeconds
	if viper.IsSet(roleProbeTimeoutVarName) {
		timeoutSeconds = viper.GetInt(roleProbeTimeoutVarName)
	}
	// lorry utilizes the pod readiness probe to trigger role probe and 'timeoutSeconds' is directly copied from the 'probe.timeoutSeconds' field of pod.
	// here we give 80% of the total time to role probe job and leave the remaining 20% to kubelet to handle the readiness probe related tasks.
	timeout := time.Duration(timeoutSeconds) * (800 * time.Millisecond)
	ctx1, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	role, err := ops.GetRole(ctx1, req, resp)
	if err != nil {
		ops.Logger.Error(err, "executing checkRole error")
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if ops.CheckRoleFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Info("role checks failed continuously", "times", ops.CheckRoleFailedCount)
			go SentProbeEvent(ctx, opsRes, resp, ops.Logger)
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
		if role != "" {
			go SentProbeEvent(ctx, opsRes, resp, ops.Logger)
		}
	}

	// RoleUnchangedCount is the count of consecutive role unchanged checks.
	// If the role remains unchanged consecutively in RoleDetectionThreshold checks after it has changed,
	// then the roleCheck event will be reported at roleEventReportFrequency so that the event controller
	// can always get relevant roleCheck events in order to maintain the pod label accurately, even in cases
	// of roleChanged events being lost or the pod role label being deleted or updated incorrectly.
	// if ops.RoleUnchangedCount < ops.RoleDetectionThreshold && ops.RoleUnchangedCount%roleEventReportFrequency == 0 {
	// 	resp.Metadata[StatusCode] = OperationFailedHTTPCode
	// }
	return opsRes, nil
}

func (ops *BaseOperations) GetRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	if ops.GetRole == nil {
		message := fmt.Sprintf("getRole operation is not implemented for %v", ops.DBType)
		ops.Logger.Error(fmt.Errorf("not implemented"), message)
		opsRes["event"] = OperationNotImplemented
		opsRes["message"] = message
		resp.Metadata[StatusCode] = OperationNotFoundHTTPCode
		return opsRes, nil
	}

	role, err := ops.GetRole(ctx, req, resp)
	if err != nil {
		ops.Logger.Error(err, "error executing getRole")
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if ops.CheckRoleFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Info("getRole failed continuously", "failed times", ops.CheckRoleFailedCount)
			// resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		ops.CheckRoleFailedCount++
		return opsRes, nil
	}
	opsRes["event"] = OperationSuccess
	opsRes["role"] = role
	return opsRes, nil
}

// Component may have some internal roles that needn't be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (ops *BaseOperations) roleValidate(role string) (bool, string) {
	// do not validate them when db roles setting is missing
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
func (ops *BaseOperations) CheckRunningOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var message string
	opsRes := OpsResult{}
	opsRes["operation"] = CheckRunningOperation

	host := net.JoinHostPort(ops.DBAddress, strconv.Itoa(ops.DBPort))
	// sql exec timeout needs to be less than httpget's timeout which by default 1s.
	conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
	if err != nil {
		message = fmt.Sprintf("running check %s error", host)
		ops.Logger.Error(err, message)
		opsRes["event"] = OperationFailed
		opsRes["message"] = message
		if ops.CheckRunningFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Info("running checks failed continuously", "times", ops.CheckRunningFailedCount)
			// resp.Metadata[StatusCode] = OperationFailedHTTPCode
			SentProbeEvent(ctx, opsRes, resp, ops.Logger)
		}
		ops.CheckRunningFailedCount++
		return opsRes, nil
	}
	defer conn.Close()
	ops.CheckRunningFailedCount = 0
	message = "TCP Connection Established Successfully!"
	if tcpCon, ok := conn.(*net.TCPConn); ok {
		err := tcpCon.SetLinger(0)
		ops.Logger.Error(err, "running check, set tcp linger failed")
	}
	opsRes["event"] = OperationSuccess
	opsRes["message"] = message
	return opsRes, nil
}

func (ops *BaseOperations) SwitchoverOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	leader := req.Metadata["leader"]
	candidate := req.Metadata["candidate"]
	if leader == "" && candidate == "" {
		opsRes["event"] = OperationFailed
		opsRes["message"] = "Leader or Candidate must be set"
		return opsRes, nil
	}

	dcsStore := dcs.GetStore()
	if dcsStore == nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = "DCS store init failed"
		return opsRes, nil
	}
	cluster, err := dcsStore.GetCluster()
	if cluster == nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = fmt.Sprintf("Get Cluster %s error: %v.", dcsStore.GetClusterName(), err)
		return opsRes, nil
	}

	manager, err := component.GetDefaultManager()
	if err != nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		return opsRes, nil
	}

	if leader != "" {
		leaderMember := cluster.GetMemberWithName(leader)
		if leaderMember == nil {
			opsRes["event"] = OperationFailed
			opsRes["message"] = fmt.Sprintf("leader %s not exists", leader)
			return opsRes, nil
		}
		if ok, err := manager.IsLeaderMember(ctx, cluster, leaderMember); err != nil || !ok {
			opsRes["event"] = OperationFailed
			opsRes["message"] = fmt.Sprintf("%s is not the leader", leader)
			return opsRes, nil
		}
	}
	if candidate != "" {
		candidateMember := cluster.GetMemberWithName(candidate)
		if candidateMember == nil {
			opsRes["event"] = OperationFailed
			opsRes["message"] = fmt.Sprintf("candidate %s not exists", candidate)
			return opsRes, nil
		}
		if !manager.IsMemberHealthy(ctx, cluster, candidateMember) {
			opsRes["event"] = OperationFailed
			opsRes["message"] = fmt.Sprintf("candidate %s is unhealthy", candidate)
			return opsRes, nil
		}
	} else if len(manager.HasOtherHealthyMembers(ctx, cluster, leader)) == 0 {
		opsRes["event"] = OperationFailed
		opsRes["message"] = "candidate is not set and has no other healthy members"
		return opsRes, nil
	}

	err = dcsStore.CreateSwitchover(leader, candidate)
	if err != nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = fmt.Sprintf("Create switchover failed: %v", err)
		return opsRes, nil
	}

	opsRes["event"] = OperationSuccess
	return opsRes, nil
}

// JoinMemberOps is used to join the current member into the DB cluster.
// If OpsResult["event"] == "success" and err == nil, it indicates that the member has successfully Joined.
// In any other situation, it signifies a failure.
func (ops *BaseOperations) JoinMemberOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	manager, err := component.GetDefaultManager()
	if manager == nil {
		// manager for the DB is not supported, just return
		opsRes["event"] = OperationSuccess
		opsRes["message"] = err.Error()
		return opsRes, nil
	}

	dcsStore := dcs.GetStore()
	var cluster *dcs.Cluster
	cluster, err = dcsStore.GetCluster()
	if err != nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = fmt.Sprintf("get cluster from dcs failed: %v", err)
		return opsRes, err
	}

	// join current member to db cluster
	err = manager.JoinCurrentMemberToCluster(ctx, cluster)
	if err != nil {
		message := fmt.Sprintf("Join member to cluster failed: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = message
		return opsRes, err
	}

	opsRes["event"] = OperationSuccess
	opsRes["message"] = "Join of the current member is complete"
	return opsRes, nil
}

// LeaveMemberOps is used to remove the current member from the DB cluster.
// If OpsResult["event"] == "success" and err == nil, it indicates that the member has successfully left.
// In any other situation, it signifies a failure.
// - "error": is used to indicate if the leave operation has failed. If error is nil, it signifies a successful leave, and if it is not nil, it indicates a failure.
// - "OpsResult": provides additional detailed messages regarding the operation.
//   - "OpsResult['event']" can hold either "fail" or "success" based on the outcome of the leave operation.
//   - "OpsResult['message']" provides a specific reason explaining the event.
func (ops *BaseOperations) LeaveMemberOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	manager, err := component.GetDefaultManager()
	if manager == nil {
		// manager for the DB is not supported, just return
		opsRes["event"] = OperationSuccess
		opsRes["message"] = err.Error()
		return opsRes, nil
	}

	dcsStore := dcs.GetStore()
	var cluster *dcs.Cluster
	cluster, err = dcsStore.GetCluster()
	if err != nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = fmt.Sprintf("get cluster from dcs failed: %v", err)
		return opsRes, err
	}

	currentMember := cluster.GetMemberWithName(manager.GetCurrentMemberName())
	if !cluster.HaConfig.IsDeleting(currentMember) {
		cluster.HaConfig.AddMemberToDelete(currentMember)
		_ = dcsStore.UpdateHaConfig()
	}

	// remove current member from db cluster
	err = manager.LeaveMemberFromCluster(ctx, cluster, manager.GetCurrentMemberName())
	if err != nil {
		message := fmt.Sprintf("Leave member form cluster failed: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = message
		return opsRes, err
	}

	opsRes["event"] = OperationSuccess
	opsRes["message"] = "left of the current member is complete"
	return opsRes, nil
}
