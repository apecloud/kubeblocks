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
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/godror/godror"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

const (
	repUser      = "rep_user"
	repPassword  = "rep_user"
	normalStatus = "NORMAL"
	MYSQL        = "MYSQL"
	ORACLE       = "ORACLE"
)

func (mgr *Manager) Switchover(ctx context.Context, cluster *dcs.Cluster, primary, candidate string, force bool) error {
	if mgr.ReplicaTenant == "" {
		return errors.New("the cluster has no replica tenant set")
	}

	if force {
		return mgr.Failover(ctx, cluster, candidate)
	}

	primaryMember, err := getMember(primary)
	if err != nil {
		return errors.Wrapf(err, "get primary %s failed", primary)
	}
	candidateMember, err := getMember(candidate)
	if err != nil {
		return errors.Wrapf(err, "get candidate %s failed", candidate)
	}

	primaryAddr := fmt.Sprintf("%s:%s", primaryMember.PodIP, primaryMember.DBPort)
	primaryDB, err := config.GetDBConnWithAddr(primaryAddr)
	if err != nil {
		mgr.Logger.Info("new primarydb connection failed", "error", err)
		return err
	}

	candidateAddr := fmt.Sprintf("%s:%s", candidateMember.PodIP, candidateMember.DBPort)
	candidateDB, err := config.GetDBConnWithAddr(candidateAddr)
	if err != nil {
		mgr.Logger.Info("new candidatedb connection failed", "error", err)
		return err
	}

	compatibilityMode, err := mgr.validateAndGetCompatibilityMode(ctx, primaryDB, candidateDB)
	if err != nil {
		return err
	}

	opTimestamp, err := mgr.GetMemberOpTimestamp(ctx, cluster, primaryMember)
	if err != nil {
		return errors.Wrap(err, "get primary op timestamp failed")
	}
	mgr.OpTimestamp = opTimestamp
	isLagging, _ := mgr.IsMemberLagging(ctx, cluster, candidateMember)
	if isLagging {
		return errors.New("candidate member is lagging")
	}

	err = mgr.standbyTenant(ctx, primaryDB)
	if err != nil {
		return err
	}

	err = mgr.primaryTenant(ctx, candidateDB)
	if err != nil {
		return err
	}

	err = mgr.createUser(ctx, candidateMember, compatibilityMode)
	if err != nil {
		mgr.Logger.Info("create user failed", "error", err)
		return err
	}

	err = mgr.setLogSource(ctx, primaryDB, *candidateMember)
	if err != nil {
		mgr.Logger.Info("set log source failed", "error", err)
	}
	return nil
}

func (mgr *Manager) validateAndGetCompatibilityMode(ctx context.Context, primaryDB, candidateDB *sql.DB) (string, error) {
	var primaryMode, candidateMode, role string
	queryTenant := fmt.Sprintf("SELECT COMPATIBILITY_MODE, TENANT_ROLE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	err := primaryDB.QueryRowContext(ctx, queryTenant).Scan(&primaryMode, &role)
	if err != nil {
		mgr.Logger.Info("query primary tenant info failed", "error", err.Error())
		return "", err
	}
	if role != PRIMARY {
		err = errors.Errorf("the primary role is not PRIMARY: %s", role)
		mgr.Logger.Info(err.Error())
		return "", err
	}

	err = candidateDB.QueryRowContext(ctx, queryTenant).Scan(&candidateMode, &role)
	if err != nil {
		mgr.Logger.Info("query candidate tenant info failed", "error", err.Error())
		return "", err
	}

	if role != STANDBY {
		err = errors.Errorf("the candidate role is not STANDBY: %s", role)
		mgr.Logger.Info(err.Error())
		return "", err
	}

	if primaryMode != candidateMode {
		err = errors.Errorf("the compatibility modes of primary and candidate are different: %s, %s", primaryMode, candidateMode)
		mgr.Logger.Info(err.Error())
		return "", err
	}

	return primaryMode, nil
}

func (mgr *Manager) setLogSource(ctx context.Context, db *sql.DB, candidateMember dcs.Member) error {
	sourceAddr := candidateMember.PodIP + ":" + candidateMember.DBPort

	sql := fmt.Sprintf("ALTER SYSTEM SET LOG_RESTORE_SOURCE = 'SERVICE=%s USER=%s@%s PASSWORD=%s' TENANT = %s",
		sourceAddr, repUser, mgr.ReplicaTenant, repPassword, mgr.ReplicaTenant)
	_, err := db.Exec(sql)
	if err != nil {
		mgr.Logger.Info(sql+" failed", "error", err) //nolint:goconst
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

func (mgr *Manager) createUser(ctx context.Context, member *dcs.Member, compatibilityMode string) error {
	tenantDB, err := mgr.getConnWithMode(member, compatibilityMode)
	if err != nil {
		return errors.Wrap(err, "get DB connection failed")
	}
	switch compatibilityMode {
	case MYSQL:
		return mgr.createUserForMySQLMode(ctx, tenantDB)
	case ORACLE:
		// rep user is auto synced for standby
		return nil
	default:
		return errors.Errorf("the compatibility mode is invalid: %s", compatibilityMode)
	}
}

// // mysql sdk driver does not support oracle tenant:
// // Error 1235 (0A000): Oracle tenant for current client driver is not supported
// func (mgr *Manager) createUserForOracleMode(ctx context.Context, db *sql.DB) error {
// 	queryUser := fmt.Sprintf("SELECT count(*) FROM ALL_USERS WHERE user='%s'", repUser)
// 	var userCount int
// 	err := db.QueryRowContext(ctx, queryUser).Scan(&userCount)
// 	if err != nil {
// 		mgr.Logger.Info(queryUser+" failed", "error", err)
// 		return err
// 	}
// 	if userCount > 0 {
// 		return nil
// 	}
// 	createUser := fmt.Sprintf("CREATE USER %s IDENTIFIED BY %s;", repUser, repPassword)
// 	createUser += fmt.Sprintf("GRANT CONNECT TO %s;", repUser)
// 	createUser += fmt.Sprintf("GRANT SELECT on SYS.GV$OB_LOG_STAT to %s;", repUser)
// 	createUser += fmt.Sprintf("GRANT SELECT on SYS.GV$OB_UNITS to %s;", repUser)
// 	createUser += fmt.Sprintf("GRANT SELECT on SYS.GV$OB_PARAMETERS to %s;", repUser)
// 	createUser += fmt.Sprintf("GRANT SELECT on SYS.DBA_OB_ACCESS_POINT to %s;", repUser)
// 	createUser += fmt.Sprintf("GRANT SELECT on SYS.DBA_OB_TENANTS to %s;", repUser)
// 	createUser += fmt.Sprintf("GRANT SELECT on SYS.DBA_OB_LS to %s;", repUser)
// 	createUser += "SET GLOBAL ob_tcp_invited_nodes='%';"
// 	_, err = db.Exec(createUser)
// 	if err != nil {
// 		mgr.Logger.Info(createUser+" failed", "error", err)
// 		return err
// 	}
//
// 	return nil
// }

func (mgr *Manager) createUserForMySQLMode(ctx context.Context, db *sql.DB) error {
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

func (mgr *Manager) getConnWithMode(member *dcs.Member, compatibilityMode string) (*sql.DB, error) {
	var user, dsn string
	var tenantDB *sql.DB
	var err error
	switch compatibilityMode {
	case MYSQL:
		user = "root"
		// "root@alice@tcp(10.1.0.47:2881)/oceanbase?multiStatements=true"
		dsn = fmt.Sprintf("%s@%s@tcp(%s:%s)/oceanbase?multiStatements=true", user, mgr.ReplicaTenant, member.PodIP, member.DBPort)
		_, err := mysql.ParseDSN(dsn)
		if err != nil {
			return nil, errors.Wrapf(err, "illegal Data Source Name (DNS): %s", dsn)
		}

		tenantDB, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, errors.Wrap(err, "get DB connection failed")
		}
	case ORACLE:
		user = "SYS"
		dsn = fmt.Sprintf("%s@%s@tcp(%s:%s)/SYS?multiStatements=true", user, mgr.ReplicaTenant, member.PodIP, member.DBPort)
		tenantDB, err = sql.Open("godror", dsn)
		if err != nil {
			return nil, errors.Wrap(err, "get DB connection failed")
		}
	default:
		err := errors.Errorf("the compatibility mode is invalid: %s", compatibilityMode)
		return nil, err
	}

	return tenantDB, nil
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

func getDCSCluster(memberName string) (*dcs.Cluster, error) {
	componentName, err := getCompnentName(memberName)
	if err != nil {
		return nil, err
	}
	k8sStore, _ := dcs.NewKubernetesStore()
	k8sStore.SetCompName(componentName)
	cluster, err := k8sStore.GetCluster()
	if err != nil {
		return nil, errors.Wrapf(err, "get cluster %s failed", componentName)
	}

	return cluster, nil
}

func getMember(memberName string) (*dcs.Member, error) {
	cluster, err := getDCSCluster(memberName)
	if err != nil {
		return nil, err
	}
	if len(cluster.Members) != 1 {
		return nil, errors.Errorf("component has %d replicas, "+
			"the replicas count need to be 1", len(cluster.Members))
	}
	return &cluster.Members[0], nil
}
