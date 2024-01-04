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

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

const (
	repUser      = "rep_user"
	repPassword  = "rep_user"
	normalStatus = "NORMAL"
)

func (mgr *Manager) Switchover(ctx context.Context, cluster *dcs.Cluster, primary, candidate string, force bool) error {
	if mgr.ReplicaTenant == "" {
		return errors.New("the cluster has no replica tenant set")
	}

	if force {
		return mgr.Failover(ctx, cluster, candidate)
	}

	primaryComponentName, err := getCompnentName(primary)
	if err != nil {
		return err
	}
	candidateComponentName, err := getCompnentName(candidate)
	if err != nil {
		return err
	}
	primaryStore, _ := dcs.NewKubernetesStore()
	primaryStore.SetCompName(primaryComponentName)
	candidateStore, _ := dcs.NewKubernetesStore()
	candidateStore.SetCompName(candidateComponentName)
	primaryCluster, err := primaryStore.GetCluster()
	if err != nil {
		return err
	}
	candidateCluster, err := candidateStore.GetCluster()
	if err != nil {
		return err
	}

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

	primaryAddr := fmt.Sprintf("%s:%s", primaryMember.PodIP, primaryMember.DBPort)
	primaryDB, err := config.GetDBConnWithAddr(primaryAddr)
	if err != nil {
		mgr.Logger.Info("new primarydb connection failed", "error", err)
		return err
	}
	err = mgr.standbyTenant(ctx, primaryDB)
	if err != nil {
		return err
	}

	candidateAddr := fmt.Sprintf("%s:%s", candidateMember.PodIP, candidateMember.DBPort)
	candidateDB, err := config.GetDBConnWithAddr(candidateAddr)
	if err != nil {
		mgr.Logger.Info("new candidatedb connection failed", "error", err)
		return err
	}

	err = mgr.primaryTenant(ctx, candidateDB)
	if err != nil {
		return err
	}

	tenantDB, err := mgr.getTenantConn(candidateMember)
	if err != nil {
		return errors.Wrap(err, "get DB connection failed")
	}
	err = mgr.createUser(ctx, tenantDB)
	if err != nil {
		mgr.Logger.Info("create user failed", "error", err)
		return err
	}

	err = mgr.setLogSource(ctx, primaryDB, candidateMember)
	if err != nil {
		mgr.Logger.Info("set log source failed", "error", err)
	}
	return nil
}

func (mgr *Manager) getTenantConn(member dcs.Member) (*sql.DB, error) {
	// "root@alice@tcp(10.1.0.47:2881)/oceanbase?multiStatements=true"
	dsn := fmt.Sprintf("root@%s@tcp(%s:%s)/oceanbase?multiStatements=true", mgr.ReplicaTenant, member.PodIP, member.DBPort)
	_, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS): %s", dsn)
	}

	tenantdb, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "get DB connection failed")
	}
	return tenantdb, nil
}

func (mgr *Manager) setLogSource(ctx context.Context, db *sql.DB, candidateMember dcs.Member) error {
	sourceAddr := candidateMember.PodIP + ":" + candidateMember.DBPort

	sql := fmt.Sprintf("ALTER SYSTEM SET LOG_RESTORE_SOURCE = 'SERVICE=%s USER=%s@%s PASSWORD=%s' TENANT = %s",
		sourceAddr, repUser, mgr.ReplicaTenant, repPassword, mgr.ReplicaTenant)
	_, err := db.Exec(sql)
	if err != nil {
		mgr.Logger.Info(sql+" failed", "error", err)
		return err
	}

	time.Sleep(time.Second)
	var scn int64
	queryTenant := fmt.Sprintf("SELECT RECOVERY_UNTIL_SCN FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
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
	queryUser := fmt.Sprintf("SELECT count(*) FROM mysql.user WHERE user='%s'", repUser)
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
	createUser += "SET GLOBAL ob_tcp_invited_nodes='%';"

	_, err = db.Exec(createUser)
	if err != nil {
		mgr.Logger.Info(createUser+" failed", "error", err)
		return err
	}

	return nil
}

func (mgr *Manager) primaryTenant(ctx context.Context, db *sql.DB) error {
	primaryTenant := "ALTER SYSTEM SWITCHOVER TO PRIMARY TENANT = " + mgr.ReplicaTenant
	_, err := db.Exec(primaryTenant)
	if err != nil {
		mgr.Logger.Info("primary standby tenant failed", "error", err)
		return err
	}

	var tenantRole, roleStatus string
	queryTenant := fmt.Sprintf("SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if tenantRole == PRIMARY && roleStatus == normalStatus {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}

func (mgr *Manager) standbyTenant(ctx context.Context, db *sql.DB) error {
	standbyTenant := "ALTER SYSTEM SWITCHOVER TO STANDBY TENANT = " + mgr.ReplicaTenant
	_, err := db.Exec(standbyTenant)
	if err != nil {
		mgr.Logger.Info("standby primary tenant failed", "error", err)
		return err
	}

	var tenantRole, roleStatus string
	queryTenant := fmt.Sprintf("SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if tenantRole == STANDBY && roleStatus == normalStatus {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}

func getCompnentName(memberName string) (string, error) {
	clusterName := os.Getenv(constant.KBEnvClusterName)
	componentName := strings.TrimPrefix(memberName, clusterName+"-")
	i := strings.LastIndex(componentName, "-")
	if i < 0 {
		return "", errors.Errorf("replica name is in an incorrect format %s", memberName)
	}
	componentName = componentName[:i]
	return componentName, nil
}
