/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package replica

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

// AccessMode defines SVC access mode enums.
// +enum
type AccessMode string

type CheckRole struct {
	operations.Base
	dcsStore                   dcs.DCS
	OriRole                    string
	CheckRoleFailedCount       int
	FailedEventReportFrequency int
	Timeout                    time.Duration
	DBRoles                    map[string]AccessMode
}

var checkrole operations.Operation = &CheckRole{}

func init() {
	err := operations.Register(strings.ToLower(string(util.CheckRoleOperation)), checkrole)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CheckRole) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	s.Logger = ctrl.Log.WithName("checkrole")
	val := viper.GetString(constant.KBEnvServiceRoles)
	if val != "" {
		if err := json.Unmarshal([]byte(val), &s.DBRoles); err != nil {
			s.Logger.Info("KB_DB_ROLES env format error", "error", err)
		}
	}

	s.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if s.FailedEventReportFrequency < 300 {
		s.FailedEventReportFrequency = 300
	} else if s.FailedEventReportFrequency > 3600 {
		s.FailedEventReportFrequency = 3600
	}

	timeoutSeconds := util.DefaultProbeTimeoutSeconds
	if viper.IsSet(constant.KBEnvRoleProbeTimeout) {
		timeoutSeconds = viper.GetInt(constant.KBEnvRoleProbeTimeout)
	}
	// lorry utilizes the pod readiness probe to trigger role probe and 'timeoutSeconds' is directly copied from the 'probe.timeoutSeconds' field of pod.
	// here we give 80% of the total time to role probe job and leave the remaining 20% to kubelet to handle the readiness probe related tasks.
	s.Timeout = time.Duration(timeoutSeconds) * (800 * time.Millisecond)
	s.OriRole = "waitForStart"
	s.Action = constant.RoleProbeAction
	return s.Base.Init(ctx)
}

func (s *CheckRole) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *CheckRole) Do(ctx context.Context, _ *operations.OpsRequest) (*operations.OpsResponse, error) {
	resp := &operations.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.CheckRoleOperation
	resp.Data["originalRole"] = s.OriRole
	var role string
	var err error

	if !s.DBManager.IsDBStartupReady() {
		resp.Data["message"] = "db not ready"
		return resp, nil
	}

	cluster := s.dcsStore.GetClusterFromCache()

	ctx1, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	role, err = s.DBManager.GetReplicaRole(ctx1, cluster)

	if err != nil {
		s.Logger.Info("executing checkRole error", "error", err.Error())
		// do not return err, as it will cause readinessprobe to fail
		err = nil
		if s.CheckRoleFailedCount%s.FailedEventReportFrequency == 0 {
			s.Logger.Info("role checks failed continuously", "times", s.CheckRoleFailedCount)
			// if err is not nil, send event through kubelet readinessprobe
			err = util.SentEventForProbe(ctx, resp.Data)
		}
		s.CheckRoleFailedCount++
		return resp, err
	}

	s.CheckRoleFailedCount = 0
	if isValid, message := s.roleValidate(role); !isValid {
		resp.Data["message"] = message
		return resp, nil
	}

	if s.OriRole == role {
		return nil, nil
	}

	// When network partition occurs, the new primary needs to send global role change information to the controller.
	isLeader, err := s.DBManager.IsLeader(ctx, cluster)
	if err != nil {
		if err != models.ErrNotImplemented {
			return nil, err
		}
		isLeader = models.IsLikelyPrimaryRole(role)
	}

	if isLeader {
		// we need to get latest member info to build global role snapshot
		members, err := s.dcsStore.GetMembers()
		if err != nil {
			return nil, err
		}
		cluster.Members = members
		resp.Data["role"] = s.buildGlobalRoleSnapshot(cluster, role)
	} else {
		resp.Data["role"] = role
	}

	resp.Data["event"] = util.OperationSuccess
	s.OriRole = role
	err = util.SentEventForProbe(ctx, resp.Data)
	return resp, err
}

// Component may have some internal roles that needn't be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (s *CheckRole) roleValidate(role string) (bool, string) {
	if role == "" {
		// some time db replica may not have role, e.g. oceanbase
		return true, ""
	}
	// do not validate them when db roles setting is missing
	if len(s.DBRoles) == 0 {
		return true, ""
	}

	var msg string
	isValid := false
	for r := range s.DBRoles {
		if strings.EqualFold(r, role) {
			isValid = true
			break
		}
	}
	if !isValid {
		msg = fmt.Sprintf("role %s is not configured in cluster definition %v", role, s.DBRoles)
	}
	return isValid, msg
}

func (s *CheckRole) buildGlobalRoleSnapshot(cluster *dcs.Cluster, role string) string {
	currentMemberName := s.DBManager.GetCurrentMemberName()
	roleSnapshot := &common.GlobalRoleSnapshot{
		Version: strconv.FormatInt(metav1.NowMicro().UnixMicro(), 10),
		PodRoleNamePairs: []common.PodRoleNamePair{
			{
				PodName:  currentMemberName,
				RoleName: role,
				PodUID:   cluster.GetMemberWithName(currentMemberName).UID,
			},
		},
	}

	for _, member := range cluster.Members {
		s.Logger.V(1).Info("check member", "member", member.Name, "role", member.Role)
		if member.Name != currentMemberName {
			// get old primary and set it's role to none
			if strings.EqualFold(member.Role, role) {
				s.Logger.Info("there is a another leader", "member", member.Name)
				if member.IsLorryReady() {
					s.Logger.Info("another leader's lorry is online, just ignore", "member", member.Name)
					continue
				}
				s.Logger.Info("reset old leader role to none", "member", member.Name)
				roleSnapshot.PodRoleNamePairs = append(roleSnapshot.PodRoleNamePairs, common.PodRoleNamePair{
					PodName:  member.Name,
					RoleName: "",
					PodUID:   cluster.GetMemberWithName(member.Name).UID,
				})
			}
		}
	}

	b, _ := json.Marshal(roleSnapshot)
	return string(b)
}
