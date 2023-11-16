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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cast"
	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/postgres"
)

type Manager struct {
	postgres.Manager
	syncStandbys   *postgres.PGStandby
	recoveryParams map[string]map[string]string
	pgControlData  map[string]string
}

var _ engines.DBManager = &Manager{}

var Mgr *Manager

var fs = afero.NewOsFs()

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	Mgr = &Manager{}

	baseManager, err := postgres.NewManager(properties)
	if err != nil {
		return nil, errors.Errorf("new base manager failed, err: %v", err)
	}

	Mgr.Manager = *baseManager.(*postgres.Manager)
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
	mgr.pgControlData = nil
	mgr.DBState = &dcs.DBState{
		Extra: map[string]string{},
	}
}

func (mgr *Manager) GetDBState(ctx context.Context, cluster *dcs.Cluster) *dcs.DBState {
	mgr.cleanDBState()

	isLeader, err := mgr.IsLeader(ctx, cluster)
	if err != nil {
		mgr.Logger.Error(err, "check is leader failed")
		return nil
	}
	mgr.SetIsLeader(isLeader)

	replicationMode, err := mgr.getReplicationMode(ctx)
	if err != nil {
		mgr.Logger.Error(err, "get replication mode failed")
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
		mgr.Logger.Error(err, "get wal position failed")
		return nil
	}
	mgr.DBState.OpTimestamp = walPosition

	timeLine := mgr.getTimeLineWithHost(ctx, "")
	if timeLine == 0 {
		mgr.Logger.Error(err, "get received timeLine failed")
		return nil
	}
	mgr.DBState.Extra[postgres.TimeLine] = strconv.FormatInt(timeLine, 10)

	if !isLeader {
		recoveryParams, err := mgr.readRecoveryParams(ctx)
		if err != nil {
			mgr.Logger.Error(nil, "get recoveryParams failed", "err", err)
			return nil
		}
		mgr.recoveryParams = recoveryParams
	}

	pgControlData := mgr.getPgControlData()
	if pgControlData == nil {
		mgr.Logger.Error(err, "get pg controlData failed")
		return nil
	}
	mgr.pgControlData = pgControlData

	return mgr.DBState
}

func (mgr *Manager) IsLeader(ctx context.Context, _ *dcs.Cluster) (bool, error) {
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

	mgr.Logger.Info(fmt.Sprintf("get member:%s role:%s", host, role))
	return role == models.PRIMARY, nil
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
	mgr.Logger.Info("DB startup ready")
	return true
}

func (mgr *Manager) GetMemberRoleWithHost(ctx context.Context, host string) (string, error) {
	sql := "select pg_is_in_recovery();"

	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil {
		mgr.Logger.Error(err, "get member role failed")
		return "", err
	}

	result, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, "parse member role failed")
		return "", err
	}

	if cast.ToBool(result[0]["pg_is_in_recovery"]) {
		return models.SECONDARY, nil
	} else {
		return models.PRIMARY, nil
	}
}

func (mgr *Manager) GetMemberAddrs(_ context.Context, cluster *dcs.Cluster) []string {
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
		mgr.Logger.Info("No leader DBState info")
		return false, 0
	}
	maxLag := cluster.HaConfig.GetMaxLagOnSwitchover()

	var host string
	if member.Name != mgr.CurrentMemberName {
		host = cluster.GetMemberAddr(*member)
	}

	replicationMode, err := mgr.getReplicationMode(ctx)
	if err != nil {
		mgr.Logger.Error(err, "get db replication mode failed")
		return true, maxLag + 1
	}

	if replicationMode == postgres.Synchronous {
		if !mgr.checkStandbySynchronizedToLeader(true, cluster) {
			return true, maxLag + 1
		}
	}

	timeLine := mgr.getTimeLineWithHost(ctx, host)
	if timeLine == 0 {
		mgr.Logger.Error(err, "get timeline with host:%s failed")
		return true, maxLag + 1
	}
	clusterTimeLine := cast.ToInt64(cluster.Leader.DBState.Extra[postgres.TimeLine])
	if clusterTimeLine != 0 && clusterTimeLine != timeLine {
		return true, maxLag + 1
	}

	walPosition, err := mgr.getWalPositionWithHost(ctx, host)
	if err != nil {
		mgr.Logger.Error(err, "check member lagging failed")
		return true, maxLag + 1
	}

	return cluster.Leader.DBState.OpTimestamp-walPosition > cluster.HaConfig.GetMaxLagOnSwitchover(), cluster.Leader.DBState.OpTimestamp - walPosition
}

// Typically, the synchronous_commit parameter remains consistent between the primary and standby
func (mgr *Manager) getReplicationMode(ctx context.Context) (string, error) {
	if mgr.DBState != nil && mgr.DBState.Extra[postgres.ReplicationMode] != "" {
		return mgr.DBState.Extra[postgres.ReplicationMode], nil
	}

	synchronousCommit, err := mgr.GetPgCurrentSetting(ctx, "synchronous_commit")
	if err != nil {
		return "", err
	}

	switch synchronousCommit {
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
	if mgr.DBState != nil && mgr.DBState.OpTimestamp != 0 && host == "" {
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
	if err != nil {
		return 0, err
	}

	if isLeader {
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
		lsn = engines.MaxInt64(replayLsn, receiveLsn)
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
		mgr.Logger.Error(err, "get wal position failed")
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

	synchronousStandbyNames, err := mgr.GetPgCurrentSetting(ctx, "synchronous_standby_names")
	if err != nil {
		mgr.Logger.Error(err, "get synchronous_standby_names failed")
		return nil
	}

	syncStandbys, err := postgres.ParsePGSyncStandby(synchronousStandbyNames)
	if err != nil {
		mgr.Logger.Error(err, "parse pg sync standby failed")
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

	if !mgr.canRewind() {
		return nil
	}

	return mgr.executeRewind(ctx)
}

func (mgr *Manager) canRewind() bool {
	_, err := postgres.PgRewind("--help")
	if err != nil {
		mgr.Logger.Error(err, "unable to execute pg_rewind")
		return false
	}

	pgControlData := mgr.getPgControlData()
	if pgControlData["wal_log_hints setting"] != "on" && pgControlData["Data page checksum version"] == "0" {
		mgr.Logger.Info("unable to execute pg_rewind due to configuration not allowed")
	}

	return true
}

func (mgr *Manager) executeRewind(ctx context.Context) error {
	if mgr.IsRunning() {
		return errors.New("can't run rewind when pg is running")
	}

	err := mgr.checkArchiveReadyWal(ctx)
	if err != nil {
		return err
	}

	// TODO: checkpoint

	return nil
}

func (mgr *Manager) checkArchiveReadyWal(ctx context.Context) error {
	archiveMode, _ := mgr.GetPgCurrentSetting(ctx, "archive_mode")
	archiveCommand, _ := mgr.GetPgCurrentSetting(ctx, "archive_command")

	if (archiveMode != "on" && archiveMode != "always") || archiveCommand == "" {
		mgr.Logger.Info("archive is not enabled")
		return nil
	}

	// starting from PostgreSQL 10, the "wal" directory has been renamed to "pg_wal"
	archiveDir := mgr.DataDir + "pg_wal/archive_status"
	var walFileList []string
	err := filepath.Walk(archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fileName := strings.Split(info.Name(), ".")
		if len(fileName) == 2 && fileName[1] == "ready" {
			walFileList = append(walFileList, fileName[0])
		}

		return nil
	})
	if err != nil {
		return err
	}
	if len(walFileList) == 0 {
		mgr.Logger.Info("no ready wal file exist")
		return nil
	}

	sort.Strings(walFileList)
	for _, wal := range walFileList {
		walFileName := archiveDir + wal + ".ready"
		_, err = postgres.ExecCommand(buildArchiverCommand(archiveCommand, wal, archiveDir))
		if err != nil {
			return err
		}

		err = fs.Rename(walFileName, archiveDir+wal+".done")
		if err != nil {
			return err
		}
	}

	return nil
}

func buildArchiverCommand(archiveCommand, walFileName, walDir string) string {
	cmd := ""

	i := 0
	archiveCommandLength := len(archiveCommand)
	for i < archiveCommandLength {
		if archiveCommand[i] == '%' && i+1 < archiveCommandLength {
			i += 1
			switch archiveCommand[i] {
			case 'p':
				cmd += walDir + walFileName
			case 'f':
				cmd += walFileName
			case 'r':
				cmd += "000000010000000000000001"
			case '%':
				cmd += "%"
			default:
				cmd += "%"
				i -= 1
			}
		} else {
			cmd += string(archiveCommand[i])
		}
		i += 1
	}

	return cmd
}

func (mgr *Manager) checkTimelineAndLsn(ctx context.Context, cluster *dcs.Cluster) bool {
	var needRewind bool
	var history *postgres.HistoryFile

	isRecovery, localTimeLine, localLsn := mgr.getLocalTimeLineAndLsn(ctx)
	if localTimeLine == 0 || localLsn == 0 {
		return false
	}

	isLeader, err := mgr.IsLeaderWithHost(ctx, cluster.GetMemberAddr(*cluster.GetLeaderMember()))
	if err != nil || !isLeader {
		mgr.Logger.Error(nil, "Leader is still in recovery and can't rewind")
		return false
	}

	primaryTimeLine, err := mgr.getPrimaryTimeLine(cluster.GetMemberAddr(*cluster.GetLeaderMember()))
	if err != nil {
		mgr.Logger.Error(err, "get primary timeLine failed")
		return false
	}

	switch {
	case localTimeLine > primaryTimeLine:
		needRewind = true
	case localTimeLine == primaryTimeLine:
		needRewind = false
	case localTimeLine < primaryTimeLine:
		history = mgr.getHistory(cluster.GetMemberAddr(*cluster.GetLeaderMember()), primaryTimeLine)
	}

	if len(history.History) != 0 {
		// use a boolean value to check if the loop should exit early
		exitFlag := false
		for _, h := range history.History {
			// Don't need to rewind just when:
			// for replica: replayed location is not ahead of switchpoint
			// for the former primary: end of checkpoint record is the same as switchpoint
			if h.ParentTimeline == localTimeLine {
				switch {
				case isRecovery:
					needRewind = localLsn > h.SwitchPoint
				case localLsn >= h.SwitchPoint:
					needRewind = true
				default:
					checkPointEnd := mgr.getCheckPointEnd(localTimeLine, localLsn)
					needRewind = h.SwitchPoint != checkPointEnd
				}
				exitFlag = true
				break
			} else if h.ParentTimeline > localTimeLine {
				needRewind = true
				exitFlag = true
				break
			}
		}
		if !exitFlag {
			needRewind = true
		}
	}

	return needRewind
}

func (mgr *Manager) getCheckPointEnd(timeLine, lsn int64) int64 {
	lsnStr := postgres.FormatPgLsn(lsn)

	resp, err := postgres.PgWalDump("-t", strconv.FormatInt(timeLine, 10), "-s", lsnStr, "-n", "2")
	if err == nil || resp == "" {
		return 0
	}

	checkPointEndStr := postgres.ParsePgWalDumpError(err.Error(), lsnStr)

	return postgres.ParsePgLsn(checkPointEndStr)
}

func (mgr *Manager) getPrimaryTimeLine(host string) (int64, error) {
	resp, err := postgres.Psql("-h", host, "replication=database", "-c", "IDENTIFY_SYSTEM")
	if err != nil {
		mgr.Logger.Error(err, "get primary time line failed")
		return 0, err
	}

	stdoutList := strings.Split(resp, "\n")
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
	timeLine := mgr.getReceivedTimeLine(ctx, "")
	lsn, _ := mgr.getLsnWithHost(ctx, "replay", "")

	return inRecovery, timeLine, lsn
}

func (mgr *Manager) getLocalTimeLineAndLsnFromControlData() (bool, int64, int64) {
	var inRecovery bool
	var timeLineStr, lsnStr string
	var timeLine, lsn int64

	pgControlData := mgr.getPgControlData()
	if slices.Contains([]string{"shut down in recovery", "in archive recovery"}, (pgControlData)["Database cluster state"]) {
		inRecovery = true
		lsnStr = (pgControlData)["Minimum recovery ending location"]
		timeLineStr = (pgControlData)["Min recovery ending loc's timeline"]
	} else if (pgControlData)["Database cluster state"] == "shut down" {
		inRecovery = false
		lsnStr = (pgControlData)["Latest checkpoint location"]
		timeLineStr = (pgControlData)["Latest checkpoint's TimeLineID"]
	}

	if lsnStr != "" {
		lsn = postgres.ParsePgLsn(lsnStr)
	}
	if timeLineStr != "" {
		timeLine, _ = strconv.ParseInt(timeLineStr, 10, 64)
	}

	return inRecovery, timeLine, lsn
}

func (mgr *Manager) getTimeLineWithHost(ctx context.Context, host string) int64 {
	if mgr.DBState != nil && mgr.DBState.Extra[postgres.TimeLine] != "" && host == "" {
		return cast.ToInt64(mgr.DBState.Extra[postgres.TimeLine])
	}

	var isLeader bool
	var err error
	if host == "" {
		isLeader, err = mgr.IsLeader(ctx, nil)
	} else {
		isLeader, err = mgr.IsLeaderWithHost(ctx, host)
	}
	if err != nil {
		mgr.Logger.Error(err, "get timeLine check leader failed")
		return 0
	}

	if isLeader {
		return mgr.getCurrentTimeLine(ctx, host)
	} else {
		return mgr.getReceivedTimeLine(ctx, host)
	}
}

func (mgr *Manager) getCurrentTimeLine(ctx context.Context, host string) int64 {
	sql := "SELECT timeline_id FROM pg_control_checkpoint();"
	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil || resp == nil {
		mgr.Logger.Error(err, "get current timeline failed")
		return 0
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, "parse query response failed", "response", string(resp))
		return 0
	}

	return cast.ToInt64(resMap[0]["timeline_id"])
}

func (mgr *Manager) getReceivedTimeLine(ctx context.Context, host string) int64 {
	sql := "select case when latest_end_lsn is null then null " +
		"else received_tli end as received_tli from pg_catalog.pg_stat_get_wal_receiver();"
	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil || resp == nil {
		mgr.Logger.Error(err, "get received timeline failed")
		return 0
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("parse query response:%s failed", string(resp)))
		return 0
	}

	return cast.ToInt64(resMap[0]["received_tli"])
}

func (mgr *Manager) getPgControlData() map[string]string {
	if mgr.pgControlData != nil {
		return mgr.pgControlData
	}

	result := map[string]string{}

	resp, err := postgres.ExecCommand("pg_controldata")
	if err != nil {
		mgr.Logger.Error(err, "get pg control data failed")
		return nil
	}

	controlDataList := strings.Split(resp, "\n")
	for _, s := range controlDataList {
		out := strings.Split(s, ":")
		if len(out) == 2 {
			result[out[0]] = strings.TrimSpace(out[1])
		}
	}
	return result
}

func (mgr *Manager) checkRecoveryConf(ctx context.Context, leaderName string) (bool, bool) {
	if mgr.MajorVersion >= 12 {
		_, err := fs.Stat(mgr.DataDir + "/standby.signal")
		if errors.Is(err, afero.ErrFileNotFound) {
			return true, true
		}
	} else {
		mgr.Logger.Info("check recovery conf")
		// TODO: support check recovery.conf
	}

	recoveryParams, err := mgr.readRecoveryParams(ctx)
	if err != nil {
		return true, true
	}

	if !strings.HasPrefix(recoveryParams[postgres.PrimaryConnInfo]["host"], leaderName) {
		if recoveryParams[postgres.PrimaryConnInfo]["context"] == "postmaster" {
			mgr.Logger.Info("host not match, need to restart")
			return true, true
		} else {
			mgr.Logger.Info("host not match, need to reload")
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

func (mgr *Manager) getHistory(host string, timeline int64) *postgres.HistoryFile {
	resp, err := postgres.Psql("-h", host, "replication=database", "-c", fmt.Sprintf("TIMELINE_HISTORY %d", timeline))
	if err != nil {
		mgr.Logger.Error(err, "get history failed")
		return nil
	}

	return postgres.ParseHistory(resp)
}

func (mgr *Manager) Promote(ctx context.Context, _ *dcs.Cluster) error {
	if isLeader, err := mgr.IsLeader(ctx, nil); isLeader && err == nil {
		mgr.Logger.Info("i am already the leader, don't need to promote")
		return nil
	}

	resp, err := postgres.PgCtl("promote")
	if err != nil {
		mgr.Logger.Error(err, "promote failed")
		return err
	}

	mgr.Logger.Info("promote success", "response", resp)
	return nil
}

func (mgr *Manager) Demote(ctx context.Context) error {
	mgr.Logger.Info(fmt.Sprintf("current member demoting: %s", mgr.CurrentMemberName))
	if isLeader, err := mgr.IsLeader(ctx, nil); !isLeader && err == nil {
		mgr.Logger.Info("i am not the leader, don't need to demote")
		return nil
	}

	return mgr.Stop()
}

func (mgr *Manager) Stop() error {
	err := mgr.DBManagerBase.Stop()
	if err != nil {
		return err
	}

	_, err = postgres.PgCtl("stop -m fast")
	if err != nil {
		mgr.Logger.Error(err, "pg_ctl stop failed")
		return err
	}

	return nil
}

func (mgr *Manager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	// only when db is not running, leader probably be nil
	if cluster.Leader == nil {
		mgr.Logger.Info("cluster has no leader now, starts db firstly without following")
		return nil
	}

	err := mgr.handleRewind(ctx, cluster)
	if err != nil {
		mgr.Logger.Error(err, "handle rewind failed")
		return err
	}

	needChange, needRestart := mgr.checkRecoveryConf(ctx, cluster.Leader.Name)
	if needChange {
		return mgr.follow(ctx, needRestart, cluster)
	}

	mgr.Logger.Info(fmt.Sprintf("no action coz i still follow the leader:%s", cluster.Leader.Name))
	return nil
}

func (mgr *Manager) follow(ctx context.Context, needRestart bool, cluster *dcs.Cluster) error {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Info("cluster has no leader now, just start if need")
		if needRestart {
			return mgr.DBManagerBase.Start(ctx, cluster)
		}
		return nil
	}

	if mgr.CurrentMemberName == leaderMember.Name {
		mgr.Logger.Info("i get the leader key, don't need to follow")
		return nil
	}

	primaryInfo := fmt.Sprintf("\nprimary_conninfo = 'host=%s port=%s user=%s password=%s application_name=%s'",
		cluster.GetMemberAddr(*leaderMember), leaderMember.DBPort, mgr.Config.Username, mgr.Config.Password, mgr.CurrentMemberName)

	pgConf, err := fs.OpenFile("/kubeblocks/conf/postgresql.conf", os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		mgr.Logger.Error(err, "open postgresql.conf failed")
		return err
	}
	defer func() {
		_ = pgConf.Close()
	}()

	writer := bufio.NewWriter(pgConf)
	_, err = writer.WriteString(primaryInfo)
	if err != nil {
		mgr.Logger.Error(err, "write into postgresql.conf failed")
		return err
	}

	err = writer.Flush()
	if err != nil {
		mgr.Logger.Error(err, "writer flush failed")
		return err
	}

	if !needRestart {
		if err = mgr.PgReload(ctx); err != nil {
			mgr.Logger.Error(err, "reload conf failed")
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
		mgr.Logger.Error(err, "start failed")
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
