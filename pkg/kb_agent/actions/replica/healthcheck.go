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
	"strings"

	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type CheckStatus struct {
	actions.Base
	LeaderFailedCount          int
	FailureThreshold           int
	checkFailedCount           int
	failedEventReportFrequency int
}

var checkstatus actions.Action = &CheckStatus{}

func init() {
	err := actions.Register(strings.ToLower(string(util.HealthyCheckOperation)), checkstatus)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CheckStatus) Init(ctx context.Context) error {
	s.failedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if s.failedEventReportFrequency < 300 {
		s.Logger.Info("failedEventReportFrequency is too small, set to 300")
		s.failedEventReportFrequency = 300
	} else if s.failedEventReportFrequency > 3600 {
		s.Logger.Info("failedEventReportFrequency is too large, set to 3600")
		s.failedEventReportFrequency = 3600
	}

	s.FailureThreshold = 3
	s.Logger = ctrl.Log.WithName("CheckStatus")
	s.Action = constant.CheckHealthyAction
	return s.Base.Init(ctx)
}

func (s *CheckStatus) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *CheckStatus) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	resp := &actions.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.HealthyCheckOperation
	err := s.Handler.HealthyCheck(ctx)
	if err != nil {
		return s.handlerError(ctx, err)
	}

	s.checkFailedCount = 0
	resp.Data["event"] = util.OperationSuccess
	return resp, nil
}

func (s *CheckStatus) handlerError(ctx context.Context, err error) (*actions.OpsResponse, error) {
	resp := &actions.OpsResponse{
		Data: map[string]any{},
	}
	message := err.Error()
	s.Logger.Info("healthy checks failed", "error", message)
	resp.Data["event"] = util.OperationFailed
	resp.Data["message"] = message
	if s.checkFailedCount%s.failedEventReportFrequency == 0 {
		s.Logger.Info("healthy checks were failling continuously", "times", s.checkFailedCount)
		_ = util.SentEventForProbe(ctx, resp.Data)
	}
	s.checkFailedCount++
	return resp, err
}