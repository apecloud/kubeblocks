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
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

type LegacyOperation func(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error)

type OpsResult map[string]interface{}

type GlobalInfo struct {
	Event     string            `json:"event,omitempty"`
	Term      int               `json:"term,omitempty"`
	Addr2Role map[string]string `json:"map,omitempty"`
	Message   string            `json:"message,omitempty"`
}

type Operation interface {
	Kind() OperationKind
	Init(metadata component.Properties) error
	Invoke(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) error
}

// AccessMode defines SVC access mode enums.
// +enum
type AccessMode string

type BaseInternalOps interface {
	InternalQuery(ctx context.Context, sql string) ([]byte, error)
	InternalExec(ctx context.Context, sql string) (int64, error)
	GetLogger() logr.Logger
	GetRunningPort() int
	Dispatch(ctx context.Context, req *ProbeRequest) (*ProbeResponse, error)
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
	OriGlobalInfo          *GlobalInfo
	DBRoles                map[string]AccessMode
	Logger                 logr.Logger
	Metadata               map[string]string
	InitIfNeed             func() bool
	GetRole                func(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (string, error)
	GetGlobalInfo          func(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (GlobalInfo, error)
	// TODO: need a better way to support the extension for engines.
	LockInstance     func(ctx context.Context) error
	UnlockInstance   func(ctx context.Context) error
	LegacyOperations map[OperationKind]LegacyOperation
	Ops              map[OperationKind]Operation
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
	ops.LegacyOperations = map[OperationKind]LegacyOperation{
		CheckRunningOperation:  ops.CheckRunningOps,
		CheckRoleOperation:     ops.CheckRoleOps,
		GetRoleOperation:       ops.GetRoleOps,
		VolumeProtection:       ops.volumeProtection,
		SwitchoverOperation:    ops.SwitchoverOps,
		GetGlobalInfoOperation: ops.GetGlobalInfoOps,
	}

	// ops.Ops = map[OperationKind]Operation{
	//	VolumeProtection: newVolumeProtectionOperation(ops.Logger, ops),
	//}
	ops.DBAddress = ops.getAddress()

	// for kind, op := range ops.Ops {
	//	if err := op.Init(properties); err != nil {
	//		ops.Logger.Error(err, fmt.Sprintf("init operation %s error", kind))
	//		// panic(fmt.Sprintf("init operation %s error: %s", kind, err.Error()))
	//	}
	//}
}

func (ops *BaseOperations) RegisterOperation(opsKind OperationKind, operation LegacyOperation) {
	if ops.LegacyOperations == nil {
		ops.LegacyOperations = map[OperationKind]LegacyOperation{}
	}
	ops.LegacyOperations[opsKind] = operation
}

func (ops *BaseOperations) RegisterOperationOnDBReady(opsKind OperationKind, operation LegacyOperation, manager component.DBManager) {
	ops.RegisterOperation(opsKind, StartupCheckWraper(manager, operation))
}

// Operations returns list of operations supported by the binding.
func (ops *BaseOperations) Operations() []OperationKind {
	opsKinds := make([]OperationKind, len(ops.LegacyOperations))
	i := 0
	for opsKind := range ops.LegacyOperations {
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

// Dispatch handles all invoke operations.
func (ops *BaseOperations) Dispatch(ctx context.Context, req *ProbeRequest) (*ProbeResponse, error) {
	if req == nil {
		return nil, errors.Errorf("invoke request required")
	}

	startTime := time.Now()
	resp := &ProbeResponse{Metadata: map[string]string{}}

	updateRespMetadata := func() (*ProbeResponse, error) {
		endTime := time.Now()
		resp.Metadata[RespEndTimeKey] = endTime.Format(time.RFC3339Nano)
		resp.Metadata[RespDurationKey] = endTime.Sub(startTime).String()
		return resp, nil
	}

	operation, ok := ops.LegacyOperations[req.Operation]
	opsRes := OpsResult{}
	if !ok {
		message := fmt.Sprintf("%v operation is not implemented for %v", req.Operation, ops.DBType)
		ops.Logger.Info(message)
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

	// sql exec timeout needs to be less than httpget's timeout which by default 1s.
	ctx1, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	role, err := ops.GetRole(ctx1, req, resp)
	if err != nil {
		ops.Logger.Error(err, "executing checkRole error")
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if ops.CheckRoleFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Info("role checks failed continuously", "times", ops.CheckRoleFailedCount)
			SentProbeEvent(ctx, opsRes, ops.Logger)
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

	if ops.OriRole != role {
		ops.OriRole = role
		SentProbeEvent(ctx, opsRes, ops.Logger)
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

func (ops *BaseOperations) GetGlobalInfoOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	opsRes["operation"] = GetGlobalInfoOperation
	if ops.GetGlobalInfo == nil {
		message := fmt.Sprintf("getGlobalInfo operation is not implemented for %v", ops.DBType)
		ops.Logger.Error(fmt.Errorf("not implemented"), message)
		opsRes["event"] = OperationNotImplemented
		opsRes["message"] = message
		resp.Metadata[StatusCode] = OperationNotFoundHTTPCode
		return opsRes, nil
	}

	globalInfo, err := ops.GetGlobalInfo(ctx, req, resp)
	if err != nil {
		ops.Logger.Error(err, "error executing GlobalInfo")
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if ops.CheckRoleFailedCount%ops.FailedEventReportFrequency == 0 {
			ops.Logger.Info("getRole failed continuously", "failed times", ops.CheckRoleFailedCount)
			SentProbeEvent(ctx, opsRes, ops.Logger)
		}
		// todo: just reuse the checkRoleFailCount temporarily
		ops.CheckRoleFailedCount++
		return opsRes, nil
	}

	ops.CheckRoleFailedCount = 0

	for _, role := range globalInfo.Addr2Role {
		if isValid, message := ops.roleValidate(role); !isValid {
			opsRes["event"] = OperationInvalid
			opsRes["message"] = message
			return opsRes, nil
		}
	}

	if ops.OriGlobalInfo == nil || globalInfo.ShouldUpdate(*ops.OriGlobalInfo) {
		ops.OriGlobalInfo = &globalInfo
		globalInfo.Transform(opsRes)
		SentProbeEvent(ctx, opsRes, ops.Logger)
	}

	return opsRes, nil
}

func (ops *BaseOperations) volumeProtection(ctx context.Context, req *ProbeRequest,
	rsp *ProbeResponse) (OpsResult, error) {
	return ops.fwdLegacyOperationCall(VolumeProtection, ctx, req, rsp)
}

func (ops *BaseOperations) fwdLegacyOperationCall(kind OperationKind, ctx context.Context,
	req *ProbeRequest, rsp *ProbeResponse) (OpsResult, error) {
	op, ok := ops.Ops[kind]
	if !ok {
		panic(fmt.Sprintf("unknown operation kind: %s", kind))
	}
	// since the rsp.Data has been set properly, it doesn't need to return a OpsResult here.
	return nil, op.Invoke(ctx, req, rsp)
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
			SentProbeEvent(ctx, opsRes, ops.Logger)
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

	characterType := viper.GetString("KB_SERVICE_CHARACTER_TYPE")
	if characterType == "" {
		opsRes["event"] = OperationFailed
		opsRes["message"] = "KB_SERVICE_CHARACTER_TYPE not set"
		return opsRes, nil
	}

	manager := component.GetManager(characterType)
	if manager == nil {
		opsRes["event"] = OperationFailed
		opsRes["message"] = fmt.Sprintf("No DB Manager for character type %s", characterType)
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
		if !manager.IsMemberHealthy(cluster, candidateMember) {
			opsRes["event"] = OperationFailed
			opsRes["message"] = fmt.Sprintf("candidate %s is unhealthy", candidate)
			return opsRes, nil
		}
	} else if len(manager.HasOtherHealthyMembers(cluster, leader)) == 0 {
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

func (g *GlobalInfo) ShouldUpdate(another GlobalInfo) bool {
	if g.Term != another.Term {
		return g.Term < another.Term
	}
	if g.Message != another.Message || g.Event != another.Event {
		return true
	}
	for k, v := range g.Addr2Role {
		if s, ok := another.Addr2Role[k]; ok {
			if s != v {
				return true
			}
		} else {
			return true
		}
	}
	return false
}

func (g *GlobalInfo) Transform(result OpsResult) {
	result["event"] = g.Event
	result["term"] = g.Term
	result["message"] = g.Message
	result["map"] = g.Addr2Role
}
