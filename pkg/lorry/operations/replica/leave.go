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
	"time"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Leave struct {
	operations.Base
	dcsStore dcs.DCS
	Timeout  time.Duration
}

var leave operations.Operation = &Leave{}

func init() {
	err := operations.Register(strings.ToLower(string(util.LeaveMemberOperation)), leave)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Leave) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	s.Logger = ctrl.Log.WithName("LeaveMember")
	s.Action = constant.MemberLeaveAction
	return s.Base.Init(ctx)
}

func (s *Leave) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	cluster, err := s.dcsStore.GetCluster()
	if err != nil {
		s.Logger.Error(err, "get cluster failed")
		return nil, err
	}

	currentMember := cluster.GetMemberWithName(s.DBManager.GetCurrentMemberName())
	if !cluster.HaConfig.IsDeleting(currentMember) {
		cluster.HaConfig.AddMemberToDelete(currentMember)
		_ = s.dcsStore.UpdateHaConfig()
	}

	// remove current member from db cluster
	err = s.DBManager.LeaveMemberFromCluster(ctx, cluster, s.DBManager.GetCurrentMemberName())
	if err != nil {
		s.Logger.Error(err, "Leave member from cluster failed")
		return nil, err
	}

	if cluster.HaConfig.IsDeleting(currentMember) {
		cluster.HaConfig.FinishDeleted(currentMember)
		_ = s.dcsStore.UpdateHaConfig()
	}

	return nil, nil
}
