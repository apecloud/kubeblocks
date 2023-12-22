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
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

const (
	tenantName  = "alice"
	repUser     = "rep-user"
	repPassword = "rep-user"
)

func (mgr *Manager) Switchover(ctx context.Context, cluster *dcs.Cluster, primary, candidate string) error {
	primaryComponentName := getCompnentName(primary)
	candidateComponentName := getCompnentName(candidate)
	primaryStore, _ := dcs.NewKubernetesStore()
	primaryStore.SetCompName(primaryComponentName)
	candidateStore, _ := dcs.NewKubernetesStore()
	candidateStore.SetCompName(candidateComponentName)
	primaryCluster, _ := primaryStore.GetCluster()
	candidateCluster, _ := candidateStore.GetCluster()

	if len(primaryCluster.Members) != 1 || len(candidateCluster.Members) != 1 {
		return errors.Errorf("primary component has %d replicas, candidate component has %d replicas, "+
			"the replicas count need to be 1", len(primaryCluster.Members), len(candidateCluster.Members))
	}

	primaryMember := primaryCluster.Members[0]
	candidateMember := candidateCluster.Members[0]
	if !strings.EqualFold(primaryMember.Role, PRIMARY) {
		return errors.Errorf("primary member's role is %s, not %s", primaryCluster.Members[0].Role, PRIMARY)
	}

	if !strings.EqualFold(candidateMember.Role, STANDBY) {
		return errors.Errorf("candidate member's role is %s, not %s", candidateCluster.Members[0].Role, STANDBY)
	}

	primaryAddr := primaryCluster.GetMemberAddrWithPort(primaryMember)
	primarydb, err := config.GetDBConnWithAddr(primaryAddr)
	if err != nil {
		mgr.Logger.Info("new primarydb connection failed", "error", err)
		return err
	}
	mgr.standbyTenant(ctx, primarydb)

	candidateAddr := candidateCluster.GetMemberAddrWithPort(candidateMember)
	candidatedb, err := config.GetDBConnWithAddr(candidateAddr)
	if err != nil {
		mgr.Logger.Info("new candidatedb connection failed", "error", err)
		return err
	}

	err = mgr.primaryTenant(ctx, candidatedb)
	if err != nil {
		return err
	}

	err = mgr.createUser(ctx, candidatedb)
	if err != nil {
		return err
	}

	mgr.setLogSource(ctx, primarydb, candidateMember)
	return nil
}

func (mgr *Manager) setLogSource(ctx context.Context, db *sql.DB, candidateMember dcs.Member) error {
	sourceAddr := candidateMember.PodIP + ":2882"

	sql := fmt.Sprintf("ALTER SYSTEM SET LOG_RESTORE_SOURCE = 'SERVICE=%s USER=%s@%s PASSWORD=%s';", sourceAddr, repUser, tenantName, repPassword)
	_, err := db.Exec(sql)
	if err != nil {
		mgr.Logger.Info(sql+" failed", "error", err)
		return err
	}

	time.Sleep(time.Second)
	var scn int64
	queryTenant := "SELECT RECOVERY_UNTIL_SCN FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME=" + tenantName
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&scn)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if scn == 4611686018427387903 {
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (mgr *Manager) createUser(ctx context.Context, db *sql.DB) error {
	queryUser := "SELECT count(*) FROM mysql.user WHERE user= " + repUser
	var userCount int
	err := db.QueryRowContext(ctx, queryUser).Scan(&userCount)
	if err != nil {
		mgr.Logger.Info(queryUser+" failed", "error", err)
		return err
	}
	if userCount > 0 {
		return nil
	}

	createUser := fmt.Sprintf("CREATE USER %s IDENTIFIED BY '%s';"+
		"GRANT SELECT ON oceanbase.* TO %s;", repUser, repPassword, repUser)
	createUser = createUser + "SET GLOBAL ob_tcp_invited_nodes='%';"

	_, err = db.Exec(createUser)
	if err != nil {
		mgr.Logger.Info(createUser+" failed", "error", err)
		return err
	}

	return nil
}

func (mgr *Manager) primaryTenant(ctx context.Context, db *sql.DB) error {
	primaryTenant := "ALTER SYSTEM SWITCHOVER TO PRIMARY TENANT = " + tenantName
	_, err := db.Exec(primaryTenant)
	if err != nil {
		mgr.Logger.Info("primary standby tenant failed", "error", err)
		return err
	}

	var tenantRole, roleStatus string
	queryTenant := "SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME=" + tenantName
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if tenantRole == PRIMARY && roleStatus == "NORMAL" {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}

func (mgr *Manager) standbyTenant(ctx context.Context, db *sql.DB) error {
	standbyTenant := "ALTER SYSTEM SWITCHOVER TO STANDBY TENANT = " + tenantName
	_, err := db.Exec(standbyTenant)
	if err != nil {
		mgr.Logger.Info("standby primary tenant failed", "error", err)
		return err
	}

	var tenantRole, roleStatus string
	queryTenant := "SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME=" + tenantName
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if tenantRole == STANDBY && roleStatus == "NORMAL" {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}

func getCompnentName(memberName string) string {
	clusterName := os.Getenv(constant.KBEnvClusterName)
	componentName := strings.TrimPrefix(memberName, clusterName+"-")
	componentName = componentName[:strings.LastIndex(componentName, "-")]
	return componentName
}
