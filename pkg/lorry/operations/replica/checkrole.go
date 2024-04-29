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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

// AccessMode defines SVC access mode enums.
// +enum
type AccessMode string

type CheckRole struct {
	operations.Base
	logger                     logr.Logger
	dcsStore                   dcs.DCS
	OriRole                    *string
	CheckRoleFailedCount       int
	FailedEventReportFrequency int
	Timeout                    time.Duration
	DBRoles                    map[string]AccessMode
	Command                    []string
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

	s.logger = ctrl.Log.WithName("checkrole")
	val := viper.GetString(constant.KBEnvServiceRoles)
	if val != "" {
		if err := json.Unmarshal([]byte(val), &s.DBRoles); err != nil {
			s.logger.Info("KB_DB_ROLES env format error", "error", err)
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
	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		roleProbeCmd, ok := actionCommands[constant.RoleProbeAction]
		if ok && len(roleProbeCmd) > 0 {
			s.Command = roleProbeCmd
		}
	}

	manager, err := register.GetDBManager(s.Command)
	if err != nil {
		return err
	}
	// In addition to active probing, if the database engine supports it, we also support passive subscription to role changes.
	go manager.SubscribeRoleChange(ctx, s.OriRole)

	return nil
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

	manager, err1 := register.GetDBManager(s.Command)
	if err1 != nil {
		return nil, errors.Wrap(err1, "get manager failed")
	}

	if !manager.IsDBStartupReady() {
		resp.Data["message"] = "db not ready"
		return resp, nil
	}

	cluster := s.dcsStore.GetClusterFromCache()

	ctx1, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	role, err = manager.GetReplicaRole(ctx1, cluster)

	if err != nil {
		s.logger.Info("executing checkRole error", "error", err.Error())
		// do not return err, as it will cause readinessprobe to fail
		err = nil
		if s.CheckRoleFailedCount%s.FailedEventReportFrequency == 0 {
			s.logger.Info("role checks failed continuously", "times", s.CheckRoleFailedCount)
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

	resp.Data["role"] = role
	if *s.OriRole == role {
		return nil, nil
	}
	resp.Data["event"] = util.OperationSuccess
	*s.OriRole = role
	err = util.SentEventForProbe(ctx, resp.Data)
	return resp, err
}

// Component may have some internal roles that needn't be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (s *CheckRole) roleValidate(role string) (bool, string) {
	if role == "" {
		return false, "role is none"
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
