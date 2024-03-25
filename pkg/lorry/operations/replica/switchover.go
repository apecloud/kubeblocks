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
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Switchover struct {
	operations.Base
	dcsStore dcs.DCS
}

type SwitchoverManager interface {
	Switchover(ctx context.Context, cluster *dcs.Cluster, primary, candidate string, force bool) error
}

var switchover operations.Operation = &Switchover{}

func init() {
	err := operations.Register(strings.ToLower(string(util.SwitchoverOperation)), switchover)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Switchover) Init(_ context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	return nil
}

func (s *Switchover) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	primary := req.GetString("primary")
	candidate := req.GetString("candidate")
	if primary == "" && candidate == "" {
		return errors.New("primary or candidate must be set")
	}

	cluster, err := s.dcsStore.GetCluster()
	if cluster == nil {
		return errors.Wrap(err, "get cluster failed")
	}

	manager, err := register.GetDBManager(nil)
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}

	if cluster.HaConfig == nil || !cluster.HaConfig.IsEnable() {
		return errors.New("cluster's ha is disabled")
	}

	if primary != "" {
		leaderMember := cluster.GetMemberWithName(primary)
		if leaderMember == nil {
			message := fmt.Sprintf("primary %s not exists", primary)
			return errors.New(message)
		}

		if ok, err := manager.IsLeaderMember(ctx, cluster, leaderMember); err != nil || !ok {
			message := fmt.Sprintf("%s is not the primary", primary)
			return errors.New(message)
		}
	}

	if candidate != "" {
		candidateMember := cluster.GetMemberWithName(candidate)
		if candidateMember == nil {
			message := fmt.Sprintf("candidate %s not exists", candidate)
			return errors.New(message)
		}

		if !manager.IsMemberHealthy(ctx, cluster, candidateMember) {
			message := fmt.Sprintf("candidate %s is unhealthy", candidate)
			return errors.New(message)
		}
	} else if len(manager.HasOtherHealthyMembers(ctx, cluster, primary)) == 0 {
		return errors.New("candidate is not set and has no other healthy members")
	}

	return nil
}

func (s *Switchover) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	primary := req.GetString("primary")
	candidate := req.GetString("candidate")
	// force := req.GetBool("force")
	// if swManager, ok := manager.(SwitchoverManager); ok {
	// 	cluster, err := s.dcsStore.GetCluster()
	// 	if cluster == nil {
	// 		return nil, errors.Wrap(err, "get cluster failed")
	// 	}

	// 	err = swManager.Switchover(ctx, cluster, primary, candidate, force)
	// 	return nil, err
	// }

	err := s.dcsStore.CreateSwitchover(primary, candidate)
	if err != nil {
		message := fmt.Sprintf("Create switchover failed: %v", err)
		return nil, errors.New(message)
	}

	return nil, nil
}
