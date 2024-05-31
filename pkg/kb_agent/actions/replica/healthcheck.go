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

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type CheckStatus struct {
	actions.Base
	LeaderFailedCount          int
	FailureThreshold           int
	dcsStore                   dcs.DCS
	checkFailedCount           int
	failedEventReportFrequency int
}

type FailoverManager interface {
	Failover(ctx context.Context, cluster *dcs.Cluster, candidate string) error
}

var checkstatus actions.Action = &CheckStatus{}

func init() {
	err := actions.Register(strings.ToLower(string(util.HealthyCheckOperation)), checkstatus)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CheckStatus) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	s.failedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if s.failedEventReportFrequency < 300 {
		s.failedEventReportFrequency = 300
	} else if s.failedEventReportFrequency > 3600 {
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

	k8sStore := s.dcsStore.(*dcs.KubernetesStore)
	cluster := k8sStore.GetClusterFromCache()

	err := s.Handler.CurrentMemberHealthyCheck(ctx, cluster)
	if err != nil {
		return s.handlerError(ctx, err)
	}

	isLeader, err := s.Handler.IsLeader(ctx, cluster)
	if err != nil {
		return s.handlerError(ctx, err)
	}

	if isLeader {
		s.LeaderFailedCount = 0
		s.checkFailedCount = 0
		resp.Data["event"] = util.OperationSuccess
		return resp, nil
	}
	err = s.Handler.LeaderHealthyCheck(ctx, cluster)
	if err != nil {
		s.LeaderFailedCount++
		if s.LeaderFailedCount > s.FailureThreshold {
			err = s.failover(ctx, cluster)
			if err != nil {
				return s.handlerError(ctx, err)
			}
		}
		return s.handlerError(ctx, err)
	}
	s.LeaderFailedCount = 0
	s.checkFailedCount = 0
	resp.Data["event"] = util.OperationSuccess
	return resp, nil
}

func (s *CheckStatus) failover(ctx context.Context, cluster *dcs.Cluster) error {
	failoverManger, ok := s.Handler.(FailoverManager)
	if !ok {
		return errors.New("failover manager not found")
	}
	err := failoverManger.Failover(ctx, cluster, s.Handler.GetCurrentMemberName())
	if err != nil {
		return errors.Wrap(err, "failover failed")
	}
	return nil
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
		s.Logger.Info("healthy checks failed continuously", "times", s.checkFailedCount)
		_ = util.SentEventForProbe(ctx, resp.Data)
	}
	s.checkFailedCount++
	return resp, err
}
