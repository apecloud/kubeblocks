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

package oceanbase

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
	ReplicaTenant     string
	CompatibilityMode string
	Members           []dcs.Member
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
	return mgr.IsLeaderMember(ctx, cluster, nil)
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	role, err := mgr.GetReplicaRoleForMember(ctx, cluster, member)

	if err != nil {
		return false, err
	}

	if strings.EqualFold(role, PRIMARY) {
		return true, nil
	}

	return false, nil
}

func (mgr *Manager) GetCompatibilityMode(ctx context.Context) (string, error) {
	if mgr.CompatibilityMode != "" {
		return mgr.CompatibilityMode, nil
	}
	sql := fmt.Sprintf("SELECT COMPATIBILITY_MODE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&mgr.CompatibilityMode)
	if err != nil {
		return "", errors.Wrap(err, "query compatibility mode failed")
	}
	return mgr.CompatibilityMode, nil
}

func (mgr *Manager) MemberHealthyCheck(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) error {
	compatibilityMode, err := mgr.GetCompatibilityMode(ctx)
	if err != nil {
		return errors.Wrap(err, "compatibility mode unknown")
	}
	switch compatibilityMode {
	case MYSQL:
		return mgr.HealthyCheckForMySQLMode(ctx, cluster, member)
	case ORACLE:
		return mgr.HealthyCheckForOracleMode(ctx, cluster, member)
	default:
		return errors.Errorf("compatibility mode not supported: [%s]", compatibilityMode)
	}
}

func (mgr *Manager) CurrentMemberHealthyCheck(ctx context.Context, cluster *dcs.Cluster) error {
	member := cluster.GetMemberWithName(mgr.CurrentMemberName)
	return mgr.MemberHealthyCheck(ctx, cluster, member)
}

func (mgr *Manager) LeaderHealthyCheck(ctx context.Context, cluster *dcs.Cluster) error {
	members := mgr.GetMembers(ctx, cluster)
	for _, member := range members {
		if isLeader, _ := mgr.IsLeaderMember(ctx, cluster, &member); isLeader {
			return mgr.MemberHealthyCheck(ctx, cluster, &member)
		}
	}

	return errors.New("no leader found")
}

func (mgr *Manager) HealthyCheckForMySQLMode(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) error {
	isLeader, err := mgr.IsLeaderMember(ctx, cluster, member)
	if err != nil {
		return err
	}
	addr := cluster.GetMemberAddrWithPort(*member)
	db, err := mgr.GetMySQLDBConnWithAddr(addr)
	if err != nil {
		return err
	}
	if isLeader {
		err = mgr.WriteCheck(ctx, db)
		if err != nil {
			return err
		}
	}
	err = mgr.ReadCheck(ctx, db)
	if err != nil {
		return err
	}

	return nil
}

func (mgr *Manager) HealthyCheckForOracleMode(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) error {
	// there is no golang driver for oceanbase oracle mode, so we just check the root connection for healthy.
	isLeader, err := mgr.IsLeader(ctx, cluster)
	if err != nil {
		return err
	}
	if isLeader {
		return nil
	}
	return nil
}

func (mgr *Manager) GetMembers(ctx context.Context, cluster *dcs.Cluster) []dcs.Member {
	clusterCR, ok := cluster.Resource.(*appsv1alpha1.Cluster)
	if !ok {
		return nil
	}
	updateMembers := false
	for _, component := range clusterCR.Spec.ComponentSpecs {
		hasMember := false
		for _, member := range mgr.Members {
			if strings.Contains(member.Name, component.Name) {
				hasMember = true
				break
			}
		}
		if !hasMember {
			updateMembers = true
			break
		}
	}
	if !updateMembers {
		return mgr.Members
	}

	mgr.Members = []dcs.Member{}
	for _, component := range clusterCR.Spec.ComponentSpecs {
		if strings.Contains(mgr.ClusterCompName, component.Name) {
			mgr.Members = append(mgr.Members, cluster.Members...)
			continue
		}
		k8sStore, _ := dcs.NewKubernetesStore()
		k8sStore.SetCompName(component.Name)
		c, err := k8sStore.GetCluster()
		if err != nil {
			return nil
		}
		mgr.Members = append(mgr.Members, c.Members...)
	}

	return mgr.Members
}
