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

package oceanbase

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	Role          = "ROLE"
	CurrentLeader = "CURRENT_LEADER"
	PRIMARY       = "PRIMARY"
	STANDBY       = "STANDBY"
)

type Manager struct {
	mysql.Manager
	ReplicaTenant string
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("Oceanbase")
	_, err := NewConfig(properties)
	if err != nil {
		return nil, err
	}

	mysqlMgr, err := mysql.NewManager(properties)
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		Manager: *mysqlMgr.(*mysql.Manager),
	}

	mgr.SetLogger(logger)
	mgr.ReplicaTenant = viperx.GetString("TENANT_NAME")
	return mgr, nil
}

func (mgr *Manager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	role, err := mgr.GetReplicaRole(ctx, cluster)

	if err != nil {
		return false, err
	}

	if strings.EqualFold(role, PRIMARY) {
		return true, nil
	}

	return false, nil
}
