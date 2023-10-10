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

	"github.com/apecloud/kubeblocks/internal/contant"
	"github.com/apecloud/kubeblocks/lorry/dcs"
	"github.com/apecloud/kubeblocks/lorry/engines/register"
	"github.com/apecloud/kubeblocks/lorry/operations"
)

// AccessMode defines SVC access mode enums.
// +enum
type AccessMode string

type CheckRole struct {
	operations.Base
	logger                     logr.Logger
	dcsStore                   dcs.DCS
	OriRole                    string
	CheckRoleFailedCount       int
	FailedEventReportFrequency int
	DBRoles                    map[string]AccessMode
}

var defaultRoleProbeTimeoutSeconds int = 2
var checkrole operations.Operation = &CheckRole{}

func init() {
	err := operations.Register("checkrole", checkrole)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CheckRole) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	val := viper.GetString("KB_SERVICE_ROLES")
	if val != "" {
		if err := json.Unmarshal([]byte(val), &s.DBRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}

	s.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if s.FailedEventReportFrequency < 300 {
		s.FailedEventReportFrequency = 300
	} else if s.FailedEventReportFrequency > 3600 {
		s.FailedEventReportFrequency = 3600
	}

	s.logger = ctrl.Log.WithName("checkrole")
	return nil
}

func (s *CheckRole) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	manager, err := register.GetOrCreateManager()
	if err != nil {
		return nil, errors.Wrap(err, "get manager failed")
	}

	timeoutSeconds := defaultRoleProbeTimeoutSeconds
	if viper.IsSet(contant.KBEnvRoleProbeTimeout) {
		timeoutSeconds = viper.GetInt(contant.KBEnvRoleProbeTimeout)
	}
	// lorry utilizes the pod readiness probe to trigger role probe and 'timeoutSeconds' is directly copied from the 'probe.timeoutSeconds' field of pod.
	// here we give 80% of the total time to role probe job and leave the remaining 20% to kubelet to handle the readiness probe related tasks.
	timeout := time.Duration(timeoutSeconds) * (800 * time.Millisecond)
	ctx1, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	role, err := manager.GetReplicaRole(ctx1)
	if err != nil {
		s.logger.Error(err, "executing checkRole error")
		if s.CheckRoleFailedCount%s.FailedEventReportFrequency == 0 {
			s.logger.Info("role checks failed continuously", "times", s.CheckRoleFailedCount)
			SentProbeEvent(ctx, opsRes, resp, s.logger)
			return nil, err
		}
		s.CheckRoleFailedCount++
		return nil, nil
	}

	resp := operations.OpsResponse{}
	s.CheckRoleFailedCount = 0
	if isValid, message := s.roleValidate(role); !isValid {
		resp.Metadata["message"] = message
		return &resp, nil
	}

	resp["role"] = role
	if ops.OriRole != role {
		ops.OriRole = role
		SentProbeEvent(ctx, opsRes, resp, ops.logger)
	}
	return nil, nil
}

// Component may have some internal roles that needn't be exposed to end user,
// and not configured in cluster definition, e.g. ETCD's Candidate.
// roleValidate is used to filter the internal roles and decrease the number
// of report events to reduce the possibility of event conflicts.
func (s *CheckRole) roleValidate(role string) (bool, string) {
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
