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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type CheckStatus struct {
	operations.Base
	dcsStore                   dcs.DCS
	dbManager                  engines.DBManager
	checkFailedCount           int
	failedEventReportFrequency int
	logger                     logr.Logger
}

var checkstatus operations.Operation = &CheckStatus{}

func init() {
	err := operations.Register("checkstatus", checkstatus)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CheckStatus) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	dbManager, err := register.GetDBManager()
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	s.dbManager = dbManager

	s.failedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if s.failedEventReportFrequency < 300 {
		s.failedEventReportFrequency = 300
	} else if s.failedEventReportFrequency > 3600 {
		s.failedEventReportFrequency = 3600
	}

	s.logger = ctrl.Log.WithName("checkstatus")
	return nil
}

func (s *CheckStatus) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *CheckStatus) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	resp := &operations.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.CheckStatusOperation
	var message string

	k8sStore := s.dcsStore.(*dcs.KubernetesStore)
	cluster := k8sStore.GetClusterFromCache()
	isHealthy := s.dbManager.IsCurrentMemberHealthy(ctx, cluster)
	if !isHealthy {
		message = "status check failed"
		s.logger.Info(message)
		resp.Data["event"] = util.OperationFailed
		resp.Data["message"] = message
		var err error
		if s.checkFailedCount%s.failedEventReportFrequency == 0 {
			s.logger.Info("status checks failed continuously", "times", s.checkFailedCount)
			err = util.SentEventForProbe(ctx, resp.Data)
		}
		s.checkFailedCount++
		return resp, err
	}
	s.checkFailedCount = 0
	resp.Data["event"] = util.OperationSuccess
	return resp, nil
}
