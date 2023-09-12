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

package officalpostgres

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cast"
	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/postgres"
	"github.com/apecloud/kubeblocks/lorry/dcs"
	"github.com/apecloud/kubeblocks/lorry/util"
)

type Manager struct {
	postgres.Manager
	syncStandbys   *postgres.PGStandby
	recoveryParams map[string]map[string]string
}

var Mgr *Manager

var fs = afero.NewOsFs()

func NewManager(logger logger.Logger) (*Manager, error) {
	Mgr = &Manager{}

	baseManager, err := postgres.NewManager(logger)
	if err != nil {
		return nil, errors.Errorf("new base manager failed, err: %v", err)
	}

	Mgr.Manager = *baseManager
	component.RegisterManager("postgresql", util.Replication, Mgr)

	return Mgr, nil
}

func (mgr *Manager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) LeaveMemberFromCluster(context.Context, *dcs.Cluster, string) error {
	return nil
}

func (mgr *Manager) IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error) {
	// for replication, the setup script imposes a constraint where the successful startup of the primary database (db0)
	// is a prerequisite for the successful launch of the remaining databases.
	return mgr.IsDBStartupReady(), nil
}

func (mgr *Manager) cleanDBState() {
	mgr.UnsetIsLeader()
	mgr.recoveryParams = nil
	mgr.syncStandbys = nil
	mgr.DBState = &dcs.DBState{
		Extra: map[string]string{},
	}
}

func (mgr *Manager) GetDBState(ctx context.Context, cluster *dcs.Cluster) *dcs.DBState {
	mgr.cleanDBState()

	isLeader, err := mgr.IsLeader(ctx, cluster)
	if err != nil {
		mgr.Logger.Errorf("check is leader failed, err:%v", err)
		return nil
	}
	mgr.SetIsLeader(isLeader)

	replicationMode, err := mgr.getReplicationMode(ctx)
	if err != nil {
		mgr.Logger.Errorf("get replication mode failed, err:%v", err)
		return nil
	}
	mgr.DBState.Extra[postgres.ReplicationMode] = replicationMode

	if replicationMode == postgres.Synchronous && cluster.Leader != nil && cluster.Leader.Name == mgr.CurrentMemberName {
		syncStandbys := mgr.getSyncStandbys(ctx)
		if syncStandbys != nil {
			mgr.syncStandbys = syncStandbys
			mgr.DBState.Extra[postgres.SyncStandBys] = strings.Join(syncStandbys.Members.ToSlice(), ",")
		}
	}

	walPosition, err := mgr.getWalPositionWithHost(ctx, "")
	if err != nil {
		mgr.Logger.Errorf("get wal position failed, err:%v", err)
		return nil
	}
	mgr.DBState.OpTimestamp = walPosition

	var timeLine int64
	if isLeader {
		timeLine = mgr.getCurrentTimeLine(ctx)
	} else {
		timeLine = mgr.getReceivedTimeLine(ctx)
	}
	if timeLine == 0 {
		mgr.Logger.Errorf("get received timeLine failed, err:%v", err)
		return nil
	}
	mgr.DBState.Extra[postgres.TimeLine] = strconv.FormatInt(timeLine, 10)

	if !isLeader {
		recoveryParams, err := mgr.readRecoveryParams(ctx)
		if err != nil {
			mgr.Logger.Errorf("get recoveryParams failed, err:%v", err)
			return nil
		}
		mgr.recoveryParams = recoveryParams
	}

	return mgr.DBState
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	isSet, isLeader := mgr.GetIsLeader()
	if isSet {
		return isLeader, nil
	}

	return mgr.IsLeaderWithHost(ctx, "")
}

func (mgr *Manager) IsLeaderWithHost(ctx context.Context, host string) (bool, error) {
	role, err := mgr.GetMemberRoleWithHost(ctx, host)
	if err != nil {
		return false, errors.Errorf("check is leader with host:%s failed, err:%v", host, err)
	}

	return role == binding.PRIMARY, nil
}

func (mgr *Manager) IsDBStartupReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if mgr.DBStartupReady {
		return true
	}

	if !mgr.IsPgReady(ctx) {
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) GetMemberRoleWithHost(ctx context.Context, host string) (string, error) {
	sql := "select pg_is_in_recovery();"

	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil {
		mgr.Logger.Errorf("get member role failed, err: %v", err)
		return "", err
	}

	result, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse member role failed, err:%v", err)
		return "", err
	}

	if cast.ToBool(result[0]["pg_is_in_recovery"]) {
		return binding.SECONDARY, nil
	} else {
		return binding.PRIMARY, nil
	}
}

func (mgr *Manager) GetMemberAddrs(ctx context.Context, cluster *dcs.Cluster) []string {
	return cluster.GetMemberAddrs()
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(ctx, cluster, cluster.GetMemberWithName(mgr.CurrentMemberName))
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	var host string
	if member.Name != mgr.CurrentMemberName {
		host = cluster.GetMemberAddr(*member)
	}

	replicationMode, err := mgr.getReplicationMode(ctx)
	if err != nil {
		mgr.Logger.Errorf("get db replication mode failed, err:%v", err)
		return false
	}

	if replicationMode == postgres.Synchronous {
		if !mgr.checkStandbySynchronizedToLeader(true, cluster) {
			return false
		}
	}

	if cluster.Leader != nil && cluster.Leader.Name == member.Name {
		if !mgr.WriteCheck(ctx, host) {
			return false
		}
	}
	if !mgr.ReadCheck(ctx, host) {
		return false
	}

	return true
}

func (mgr *Manager) IsMemberLagging(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, int64) {
	if cluster.Leader == nil || cluster.Leader.DBState == nil {
		mgr.Logger.Warnf("No leader DBState info")
		return false, 0
	}

	var host string
	if member.Name != mgr.CurrentMemberName {
		host = cluster.GetMemberAddr(*member)
	}
	walPosition, err := mgr.getWalPositionWithHost(ctx, host)
	if err != nil {
		mgr.Logger.Errorf("check member lagging failed, err:%v", err)
		return true, cluster.HaConfig.GetMaxLagOnSwitchover() + 1
	}

	// TODO: check timeLine

	return cluster.Leader.DBState.OpTimestamp-walPosition > cluster.HaConfig.GetMaxLagOnSwitchover(), cluster.Leader.DBState.OpTimestamp - walPosition
}

// Typically, the synchronous_commit parameter remains consistent between the primary and standby
func (mgr *Manager) getReplicationMode(ctx context.Context) (string, error) {
	if mgr.DBState != nil && mgr.DBState.Extra[postgres.ReplicationMode] != "" {
		return mgr.DBState.Extra[postgres.ReplicationMode], nil
	}

	sql := "select pg_catalog.current_setting('synchronous_commit');"
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		return "", err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		return "", errors.Errorf("parse query response:%s failed, err:%v", string(resp), err)
	}

	switch cast.ToString(resMap[0]["current_setting"]) {
	case "off":
		return postgres.Asynchronous, nil
	case "local":
		return postgres.Asynchronous, nil
	case "remote_write":
		return postgres.Asynchronous, nil
	case "on":
		return postgres.Synchronous, nil
	case "remote_apply":
		return postgres.Synchronous, nil
	default: // default "on"
		return postgres.Synchronous, nil
	}
}

func (mgr *Manager) getWalPositionWithHost(ctx context.Context, host string) (int64, error) {
	if mgr.DBState != nil && mgr.DBState.OpTimestamp != 0 {
		return mgr.DBState.OpTimestamp, nil
	}

	var (
		lsn      int64
		isLeader bool
		err      error
	)

	if host == "" {
		isLeader, err = mgr.IsLeader(ctx, nil)
	} else {
		isLeader, err = mgr.IsLeaderWithHost(ctx, host)
	}
	if isLeader && err == nil {
		lsn, err = mgr.getLsnWithHost(ctx, "current", host)
		if err != nil {
			return 0, err
		}
	} else {
		replayLsn, errReplay := mgr.getLsnWithHost(ctx, "replay", host)
		receiveLsn, errReceive := mgr.getLsnWithHost(ctx, "receive", host)
		if errReplay != nil && errReceive != nil {
			return 0, errors.Errorf("get replayLsn or receiveLsn failed, replayLsn err:%v, receiveLsn err:%v", errReplay, errReceive)
		}
		lsn = component.MaxInt64(replayLsn, receiveLsn)
	}

	return lsn, nil
}

func (mgr *Manager) getLsnWithHost(ctx context.Context, types string, host string) (int64, error) {
	var sql string
	switch types {
	case "current":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_current_wal_lsn(), '0/0')::bigint;"
	case "replay":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_last_wal_replay_lsn(), '0/0')::bigint;"
	case "receive":
		sql = "select pg_catalog.pg_wal_lsn_diff(COALESCE(pg_catalog.pg_last_wal_receive_lsn(), '0/0'), '0/0')::bigint;"
	}

	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil {
		mgr.Logger.Errorf("get wal position failed, err:%v", err)
		return 0, err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		return 0, errors.Errorf("parse query response:%s failed, err:%v", string(resp), err)
	}

	return cast.ToInt64(resMap[0]["pg_wal_lsn_diff"]), nil
}

// only the leader has this information.
func (mgr *Manager) getSyncStandbys(ctx context.Context) *postgres.PGStandby {
	if mgr.syncStandbys != nil {
		return mgr.syncStandbys
	}

	sql := "select pg_catalog.current_setting('synchronous_standby_names');"
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return nil
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return nil
	}

	syncStandbys, err := postgres.ParsePGSyncStandby(cast.ToString(resMap[0]["current_setting"]))
	if err != nil {
		mgr.Logger.Errorf("parse pg sync standby failed, err:%v", err)
		return nil
	}
	return syncStandbys
}

func (mgr *Manager) checkStandbySynchronizedToLeader(checkLeader bool, cluster *dcs.Cluster) bool {
	if cluster.Leader == nil || cluster.Leader.DBState == nil {
		return false
	}
	syncStandBysStr := cluster.Leader.DBState.Extra[postgres.SyncStandBys]
	syncStandBys := strings.Split(syncStandBysStr, ",")

	return (checkLeader && mgr.CurrentMemberName == cluster.Leader.Name) || slices.Contains(syncStandBys, mgr.CurrentMemberName)
}

func (mgr *Manager) handleRewind(ctx context.Context, cluster *dcs.Cluster) error {
	needRewind := mgr.checkTimelineAndLsn(ctx, cluster)
	if !needRewind {
		return nil
	}

	return mgr.executeRewind()
}

func (mgr *Manager) executeRewind() error {
	return nil
}

func (mgr *Manager) checkTimelineAndLsn(ctx context.Context, cluster *dcs.Cluster) bool {
	var needRewind bool
	var historys []*postgres.History

	isRecovery, localTimeLine, localLsn := mgr.getLocalTimeLineAndLsn(ctx)
	if localTimeLine == 0 || localLsn == 0 {
		return false
	}

	isLeader, err := mgr.IsLeaderWithHost(ctx, cluster.GetMemberAddr(*cluster.GetLeaderMember()))
	if err != nil || !isLeader {
		mgr.Logger.Warnf("Leader is still in recovery and can't rewind")
		return false
	}

	primaryTimeLine, err := mgr.getPrimaryTimeLine(cluster.GetMemberAddr(*cluster.GetLeaderMember()))
	if err != nil {
		mgr.Logger.Errorf("get primary timeLine failed, err:%v", err)
		return false
	}

	switch {
	case localTimeLine > primaryTimeLine:
		needRewind = true
	case localTimeLine == primaryTimeLine:
		needRewind = false
	case primaryTimeLine > 1:
		historys = mgr.getHistory()
	}

	if len(historys) != 0 {
		for _, h := range historys {
			if h.ParentTimeline == localTimeLine {
				switch {
				case isRecovery:
					needRewind = localLsn > h.SwitchPoint
				case localLsn >= h.SwitchPoint:
					needRewind = true
				default:
					// TODO:get checkpoint end
				}
				break
			} else if h.ParentTimeline > localTimeLine {
				needRewind = true
				break
			}
		}
	}

	return needRewind
}

func (mgr *Manager) getPrimaryTimeLine(host string) (int64, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("psql", "-h", host, "replication=database", "-c", "IDENTIFY_SYSTEM")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil || stderr.String() != "" {
		mgr.Logger.Errorf("get primary time line failed, err:%v, stderr%s", err, stderr.String())
		return 0, err
	}

	stdoutList := strings.Split(stdout.String(), "\n")
	value := stdoutList[2]
	values := strings.Split(value, "|")

	primaryTimeLine := strings.TrimSpace(values[1])

	return strconv.ParseInt(primaryTimeLine, 10, 64)
}

func (mgr *Manager) getLocalTimeLineAndLsn(ctx context.Context) (bool, int64, int64) {
	var inRecovery bool

	if !mgr.IsRunning() {
		return mgr.getLocalTimeLineAndLsnFromControlData()
	}

	inRecovery = true
	timeLine := mgr.getReceivedTimeLine(ctx)
	lsn, _ := mgr.getLsnWithHost(ctx, "replay", "")

	return inRecovery, timeLine, lsn
}

func (mgr *Manager) getLocalTimeLineAndLsnFromControlData() (bool, int64, int64) {
	var inRecovery bool
	var timeLineStr, lsnStr string
	var timeLine, lsn int64

	pgControlData := mgr.getPgControlData()
	if slices.Contains([]string{"shut down in recovery", "in archive recovery"}, (*pgControlData)["Database cluster state"]) {
		inRecovery = true
		lsnStr = (*pgControlData)["Minimum recovery ending location"]
		timeLineStr = (*pgControlData)["Min recovery ending loc's timeline"]
	} else if (*pgControlData)["Database cluster state"] == "shut down" {
		inRecovery = false
		lsnStr = (*pgControlData)["Latest checkpoint location"]
		timeLineStr = (*pgControlData)["Latest checkpoint's TimeLineID"]
	}

	if lsnStr != "" {
		lsn = postgres.ParsePgLsn(lsnStr)
	}
	if timeLineStr != "" {
		timeLine, _ = strconv.ParseInt(timeLineStr, 10, 64)
	}

	return inRecovery, timeLine, lsn
}

func (mgr *Manager) getCurrentTimeLine(ctx context.Context) int64 {
	if mgr.DBState != nil && mgr.DBState.Extra[postgres.TimeLine] != "" {
		return cast.ToInt64(mgr.DBState.Extra[postgres.TimeLine])
	}

	sql := "SELECT timeline_id FROM pg_control_checkpoint();"
	resp, err := mgr.Query(ctx, sql)
	if err != nil || resp == nil {
		mgr.Logger.Errorf("get current timeline failed, err%v", err)
		return 0
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return 0
	}

	return cast.ToInt64(resMap[0]["timeline_id"])
}

func (mgr *Manager) getReceivedTimeLine(ctx context.Context) int64 {
	if mgr.DBState != nil && mgr.DBState.Extra[postgres.TimeLine] != "" {
		return cast.ToInt64(mgr.DBState.Extra[postgres.TimeLine])
	}

	sql := "select case when latest_end_lsn is null then null " +
		"else received_tli end as received_tli from pg_catalog.pg_stat_get_wal_receiver();"
	resp, err := mgr.Query(ctx, sql)
	if err != nil || resp == nil {
		mgr.Logger.Errorf("get received timeline failed, err%v", err)
		return 0
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return 0
	}

	return cast.ToInt64(resMap[0]["received_tli"])
}

func (mgr *Manager) getPgControlData() *map[string]string {
	result := map[string]string{}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("pg_controldata")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil || stderr.String() != "" {
		mgr.Logger.Errorf("get pg control data failed, err:%v, stderr: %s", err, stderr.String())
		return &result
	}

	stdoutList := strings.Split(stdout.String(), "\n")
	for _, s := range stdoutList {
		out := strings.Split(s, ":")
		if len(out) == 2 {
			result[out[0]] = strings.TrimSpace(out[1])
		}
	}

	return &result
}

func (mgr *Manager) checkRecoveryConf(ctx context.Context, leaderName string) (bool, bool) {
	if mgr.MajorVersion >= 12 {
		_, err := fs.Stat(mgr.DataDir + "/standby.signal")
		if errors.Is(err, afero.ErrFileNotFound) {
			return true, true
		}
	} else {
		mgr.Logger.Infof("check recovery conf")
		// TODO: support check recovery.conf
	}

	recoveryParams, err := mgr.readRecoveryParams(ctx)
	if err != nil {
		return true, true
	}

	if !strings.HasPrefix(recoveryParams[postgres.PrimaryConnInfo]["host"], leaderName) {
		if recoveryParams[postgres.PrimaryConnInfo]["context"] == "postmaster" {
			mgr.Logger.Warnf("host not match, need to restart")
			return true, true
		} else {
			mgr.Logger.Warnf("host not match, need to reload")
			return true, false
		}
	}

	return false, false
}

func (mgr *Manager) readRecoveryParams(ctx context.Context) (map[string]map[string]string, error) {
	if mgr.recoveryParams != nil {
		return mgr.recoveryParams, nil
	}

	sql := fmt.Sprintf(`SELECT name, setting, context FROM pg_catalog.pg_settings WHERE pg_catalog.lower(name) = '%s';`, postgres.PrimaryConnInfo)
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		return nil, err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		return nil, err
	}

	primaryConnInfo := postgres.ParsePrimaryConnInfo(cast.ToString(resMap[0]["setting"]))
	primaryConnInfo["context"] = cast.ToString(resMap[0]["context"])

	return map[string]map[string]string{
		postgres.PrimaryConnInfo: primaryConnInfo,
	}, nil
}

// TODO: Parse history file
func (mgr *Manager) getHistory() []*postgres.History {
	return nil
}

func (mgr *Manager) Promote(ctx context.Context, cluster *dcs.Cluster) error {
	if isLeader, err := mgr.IsLeader(ctx, nil); isLeader && err == nil {
		mgr.Logger.Infof("i am already the leader, don't need to promote")
		return nil
	}

	err := mgr.prePromote()
	if err != nil {
		return err
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("su", "-c", "pg_ctl promote", "postgres")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil || stderr.String() != "" {
		mgr.Logger.Errorf("promote failed, err:%v, stderr:%s", err, stderr.String())
		return err
	}
	mgr.Logger.Infof("promote success, response:%s", stdout.String())

	err = mgr.postPromote()
	if err != nil {
		return err
	}
	return nil
}

func (mgr *Manager) prePromote() error {
	return nil
}

func (mgr *Manager) postPromote() error {
	return nil
}

func (mgr *Manager) Demote(ctx context.Context) error {
	mgr.Logger.Infof("current member demoting: %s", mgr.CurrentMemberName)
	if isLeader, err := mgr.IsLeader(ctx, nil); !isLeader && err == nil {
		mgr.Logger.Infof("i am not the leader, don't need to demote")
		return nil
	}

	return mgr.Stop()
}

func (mgr *Manager) Stop() error {
	err := mgr.DBManagerBase.Stop()
	if err != nil {
		return err
	}

	var stdout, stderr bytes.Buffer
	stopCmd := exec.Command("su", "-c", "pg_ctl stop -m fast", "postgres")
	stopCmd.Stdout = &stdout
	stopCmd.Stderr = &stderr

	err = stopCmd.Run()
	if err != nil || stderr.String() != "" {
		mgr.Logger.Errorf("stop failed, err:%v, stderr:%s", err, stderr.String())
		return err
	}

	return nil
}

func (mgr *Manager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	// only when db is not running, leader probably be nil
	if cluster.Leader == nil {
		mgr.Logger.Infof("cluster has no leader now, starts db firstly without following")
		return nil
	}

	err := mgr.handleRewind(ctx, cluster)
	if err != nil {
		mgr.Logger.Errorf("handle rewind failed, err:%v", err)
	}

	needChange, needRestart := mgr.checkRecoveryConf(ctx, cluster.Leader.Name)
	if needChange {
		return mgr.follow(ctx, needRestart, cluster)
	}

	mgr.Logger.Infof("no action coz i still follow the leader:%s", cluster.Leader.Name)
	return nil
}

func (mgr *Manager) follow(ctx context.Context, needRestart bool, cluster *dcs.Cluster) error {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Infof("cluster has no leader now, just start if need")
		if needRestart {
			return mgr.DBManagerBase.Start(ctx, cluster)
		}
		return nil
	}

	if mgr.CurrentMemberName == leaderMember.Name {
		mgr.Logger.Infof("i get the leader key, don't need to follow")
		return nil
	}

	primaryInfo := fmt.Sprintf("\nprimary_conninfo = 'host=%s port=%s user=%s password=%s application_name=%s'",
		cluster.GetMemberAddr(*leaderMember), leaderMember.DBPort, mgr.Config.Username, mgr.Config.Password, mgr.CurrentMemberName)

	pgConf, err := fs.OpenFile("/kubeblocks/conf/postgresql.conf", os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		mgr.Logger.Errorf("open postgresql.conf failed, err:%v", err)
		return err
	}
	defer func() {
		_ = pgConf.Close()
	}()

	writer := bufio.NewWriter(pgConf)
	_, err = writer.WriteString(primaryInfo)
	if err != nil {
		mgr.Logger.Errorf("write into postgresql.conf failed, err:%v", err)
		return err
	}

	err = writer.Flush()
	if err != nil {
		mgr.Logger.Errorf("writer flush failed, err:%v", err)
		return err
	}

	if !needRestart {
		if err = mgr.PgReload(ctx); err != nil {
			mgr.Logger.Errorf("reload conf failed, err:%v", err)
			return err
		}
		return nil
	}

	return mgr.DBManagerBase.Start(ctx, cluster)
}

// Start for postgresql replication, not only means the start of a database instance
// but also signifies its launch as a follower in the cluster, following the leader.
func (mgr *Manager) Start(ctx context.Context, cluster *dcs.Cluster) error {
	err := mgr.follow(ctx, true, cluster)
	if err != nil {
		mgr.Logger.Errorf("start failed, err:%v", err)
		return err
	}
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member {
	return nil
}

func (mgr *Manager) HasOtherHealthyMembers(ctx context.Context, cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)

	for i, m := range cluster.Members {
		if m.Name != leader && mgr.IsMemberHealthy(ctx, cluster, &m) {
			members = append(members, &cluster.Members[i])
		}
	}

	return members
}
