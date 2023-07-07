package postgres

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/pkg/cri/util"
	"github.com/jackc/pgx/v5"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type Manager struct {
	component.DBManagerBase
	Pool    *pgxpool.Pool
	PidFile *PidFile
	Proc    *process.Process
}

var Mgr *Manager

func NewManager(logger logger.Logger) (*Manager, error) {
	database, err := pgxpool.NewWithConfig(context.Background(), config.pool)
	if err != nil {
		return nil, errors.Errorf("unable to ping the DB: %v", err)
	}

	Mgr = &Manager{
		DBManagerBase: component.DBManagerBase{
			CurrentMemberName: viper.GetString("KB_POD_NAME"),
			ClusterCompName:   viper.GetString("KB_CLUSTER_COMP_NAME"),
			Namespace:         viper.GetString("KB_NAMESPACE"),
			Logger:            logger,
			DataDir:           viper.GetString("PGDATA"),
		},
		Pool: database,
	}

	pidFile, err := Mgr.readPidFile()
	if err != nil {
		return nil, errors.Wrap(err, "read pid file")
	}
	Mgr.PidFile = pidFile
	err = Mgr.newProcessFromPidFile()
	if err != nil {
		return nil, errors.Wrap(err, "new process from pid file")
	}

	return Mgr, nil
}

func (mgr *Manager) readPidFile() (*PidFile, error) {
	file := &PidFile{}
	f, err := os.Open(mgr.DataDir + "/postmaster.pid")
	if err != nil {
		return nil, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			mgr.Logger.Error(err)
		}
	}()

	scanner := bufio.NewScanner(f)
	var text []string
	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	pid, err := strconv.ParseInt(text[0], 10, 32)
	if err != nil {
		return nil, err
	}
	file.pid = int32(pid)
	file.dataDir = text[1]
	startTS, _ := strconv.ParseInt(text[2], 10, 64)
	file.startTs = startTS
	port, _ := strconv.ParseInt(text[3], 10, 64)
	file.port = int(port)

	return file, nil
}

func (mgr *Manager) newProcessFromPidFile() error {
	proc, err := process.NewProcess(mgr.PidFile.pid)
	if err != nil {
		return err
	}

	mgr.Proc = proc
	return nil
}

func (mgr *Manager) Query(ctx context.Context, sql string) (result []byte, err error) {
	return mgr.QueryWithPool(ctx, sql, nil)
}

func (mgr *Manager) QueryWithPool(ctx context.Context, sql string, pool *pgxpool.Pool) (result []byte, err error) {
	mgr.Logger.Debugf("query: %s", sql)

	var rows pgx.Rows
	if pool != nil {
		rows, err = pool.Query(ctx, sql)
		defer pool.Close()
	} else {
		rows, err = mgr.Pool.Query(ctx, sql)
	}
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer func() {
		rows.Close()
		_ = rows.Err()
	}()

	rs := make([]interface{}, 0)
	columnTypes := rows.FieldDescriptions()
	for rows.Next() {
		values := make([]interface{}, len(columnTypes))
		for i := range values {
			values[i] = new(interface{})
		}

		if err = rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		r := map[string]interface{}{}
		for i, ct := range columnTypes {
			r[ct.Name] = values[i]
		}
		rs = append(rs, r)
	}

	if result, err = json.Marshal(rs); err != nil {
		err = fmt.Errorf("error serializing results: %w", err)
	}
	return result, err
}

func (mgr *Manager) Exec(ctx context.Context, sql string) (result int64, err error) {
	mgr.Logger.Debugf("exec: %s", sql)

	res, err := mgr.Pool.Exec(ctx, sql)
	if err != nil {
		return 0, fmt.Errorf("error executing query: %w", err)
	}

	result = res.RowsAffected()

	return
}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}

	cmd := exec.Command("pg_isready")
	if config.username != "" {
		cmd.Args = append(cmd.Args, "-U", config.username)
	}
	if config.host != "" {
		cmd.Args = append(cmd.Args, "-h", config.host)
	}
	if config.port != 0 {
		cmd.Args = append(cmd.Args, "-p", strconv.FormatUint(uint64(config.port), 10))
	}
	err := cmd.Run()
	if err != nil {
		mgr.Logger.Infof("DB is not ready: %v", err)
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) GetMemberStateWithPool(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	sql := "select pg_is_in_recovery();"

	var rows pgx.Rows
	var err error
	if pool != nil {
		rows, err = pool.Query(ctx, sql)
		defer pool.Close()
	} else {
		rows, err = mgr.Pool.Query(ctx, sql)
	}
	if err != nil {
		mgr.Logger.Infof("error executing %s: %v", sql, err)
		return "", errors.Wrapf(err, "error executing %s", sql)
	}

	var isRecovery bool
	var isReady bool
	for rows.Next() {
		if err = rows.Scan(&isRecovery); err != nil {
			mgr.Logger.Errorf("Role query error: %v", err)
			return "", err
		}
		isReady = true
	}
	if isRecovery {
		return binding.SECONDARY, nil
	}
	if isReady {
		return binding.PRIMARY, nil
	}
	return "", errors.Errorf("exec sql %s failed: no data returned", sql)
}

func (mgr *Manager) IsLeader(ctx context.Context) (bool, error) {
	return mgr.IsLeaderWithPool(ctx, nil)
}

func (mgr *Manager) IsLeaderWithPool(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	role, err := mgr.GetMemberStateWithPool(ctx, pool)
	if err != nil {
		return false, errors.Wrap(err, "check is leader")
	}

	return role == binding.PRIMARY, nil
}

func (mgr *Manager) GetMemberAddrs(cluster *dcs.Cluster) []string {
	return cluster.GetMemberAddrs()
}

func (mgr *Manager) Initialize() {}

func (mgr *Manager) IsRunning() bool {
	if mgr.Proc != nil {
		if isRunning, err := mgr.Proc.IsRunning(); isRunning && err == nil {
			return true
		}
		mgr.Proc = nil
	}

	return mgr.newProcessFromPidFile() == nil
}

func (mgr *Manager) IsCurrentMemberInCluster(cluster *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) IsCurrentMemberHealthy(cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(mgr.CurrentMemberName, cluster)
}

func (mgr *Manager) IsMemberHealthy(memberName string, cluster *dcs.Cluster) bool {
	ctx := context.TODO()

	pools := []*pgxpool.Pool{nil}
	var err error
	if memberName != mgr.CurrentMemberName {
		host := cluster.GetMemberAddr(*cluster.GetMemberWithName(memberName))

		pools, err = mgr.GetOtherPoolsWithHosts(ctx, []string{host})
		if err != nil || pools[0] == nil {
			mgr.Logger.Errorf("Get other pools failed, err:%v", err)
			return false
		}
	}

	// Typically, the synchronous_commit parameter remains consistent between the primary and standby
	replicationMode, err := mgr.getReplicationMode(ctx)
	if err != nil {
		mgr.Logger.Errorf("get db replication mode failed, err:%v", err)
		return false
	}

	if replicationMode == "synchronous" {
		if !mgr.checkStandbySynchronizedToLeader(ctx, memberName, true, cluster) {
			return false
		}
	}

	walPosition, _ := mgr.getWalPositionWithPool(ctx, pools[0])
	if mgr.isLagging(walPosition, cluster) {
		mgr.Logger.Infof("my wal position exceeds max lag")
		return false
	}

	// TODO: check timeLine

	return true
}

func (mgr *Manager) getReplicationMode(ctx context.Context) (string, error) {
	sql := "select pg_catalog.current_setting('synchronous_commit');"

	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		return "", err
	}

	mode := strings.TrimFunc(strings.Split(string(resp), ":")[1], func(r rune) bool {
		return !unicode.IsLetter(r)
	})
	switch mode {
	case "off":
		return "asynchronous", nil
	case "local":
		return "asynchronous", nil
	case "remote_write":
		return "asynchronous", nil
	case "on":
		return "synchronous", nil
	case "remote_apply":
		return "synchronous", nil
	default: // default "on"
		return "synchronous", nil
	}
}

// TODO: restore the sync state to cluster coz these values only exist in primary
func (mgr *Manager) checkStandbySynchronizedToLeader(ctx context.Context, memberName string, isLeader bool, cluster *dcs.Cluster) bool {
	sql := "select pg_catalog.current_setting('synchronous_standby_names');"
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s, err:%v", sql, err)
		return false
	}
	standbyNames := strings.Split(strings.Split(string(resp), ":")[1], `"`)[1]

	syncStandbys, err := parsePGSyncStandby(standbyNames)
	if err != nil {
		mgr.Logger.Errorf("parse pg sync standby failed, err:%v", err)
		return false
	}

	return (isLeader && memberName == cluster.Leader.Name) || syncStandbys.Members.Contains(memberName) || syncStandbys.HasStar
}

func (mgr *Manager) getWalPositionWithPool(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var (
		lsn      int64
		isLeader bool
		err      error
	)

	if isLeader, err = mgr.IsLeaderWithPool(ctx, pool); isLeader && err == nil {
		lsn, err = mgr.getLsnWithPool(ctx, "current", pool)
		if err != nil {
			return 0, err
		}
	} else {
		replayLsn, errReplay := mgr.getLsnWithPool(ctx, "replay", pool)
		receiveLsn, errReceive := mgr.getLsnWithPool(ctx, "receive", pool)
		if errReplay != nil && errReceive != nil {
			return 0, errors.Errorf("get replayLsn or receiveLsn failed, replayLsn err:%v, receiveLsn err:%v", errReplay, errReceive)
		}
		lsn = component.MaxInt64(replayLsn, receiveLsn)
	}

	return lsn, nil
}

func (mgr *Manager) getLsnWithPool(ctx context.Context, types string, pool *pgxpool.Pool) (int64, error) {
	var sql string
	switch types {
	case "current":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_current_wal_lsn(), '0/0')::bigint;"
	case "replay":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_last_wal_replay_lsn(), '0/0')::bigint;"
	case "receive":
		sql = "select pg_catalog.pg_wal_lsn_diff(COALESCE(pg_catalog.pg_last_wal_receive_lsn(), '0/0'), '0/0')::bigint;"
	}

	resp, err := mgr.QueryWithPool(ctx, sql, pool)
	if err != nil {
		mgr.Logger.Errorf("get wal position failed, err:%v", err)
		return 0, err
	}
	lsnStr := strings.TrimFunc(string(resp), func(r rune) bool {
		return !unicode.IsDigit(r)
	})

	lsn, err := strconv.ParseInt(lsnStr, 10, 64)
	if err != nil {
		mgr.Logger.Errorf("convert lsnStr to lsn failed, err:%v", err)
	}

	return lsn, nil
}

func (mgr *Manager) isLagging(walPosition int64, cluster *dcs.Cluster) bool {
	lag := cluster.GetOpTime() - walPosition
	return lag > cluster.HaConfig.GetMaxLagOnSwitchover()
}

func (mgr *Manager) Recover() {}

// AddCurrentMemberToCluster postgresql don't need to add member
func (mgr *Manager) AddCurrentMemberToCluster(cluster *dcs.Cluster) error {
	return nil
}

// DeleteMemberFromCluster postgresql don't need to delete member
func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	return nil
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return true, nil
}

func (mgr *Manager) Promote() error {
	err := mgr.prePromote()
	if err != nil {
		return err
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("su", "-c", `'pg_ctl promote'`, "postgres")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil || string(stderr.Bytes()) != "" {
		mgr.Logger.Errorf("promote failed, err:%v, stderr:%s", err, string(stderr.Bytes()))
		return err
	}
	mgr.Logger.Infof("promote success, response:%s", string(stdout.Bytes()))

	err = mgr.postPromote()
	return nil
}

func (mgr *Manager) prePromote() error {
	return nil
}

func (mgr *Manager) postPromote() error {
	return nil
}

func (mgr *Manager) Demote() error {
	var stdout, stderr bytes.Buffer
	stopCmd := exec.Command("su", "-c", `'pg_ctl stop -m fast'`, "postgres")
	stopCmd.Stdout = &stdout
	stopCmd.Stderr = &stderr

	err := stopCmd.Run()
	if err != nil || string(stderr.Bytes()) != "" {
		mgr.Logger.Errorf("stop failed, err:%v, stderr:%s", err, string(stderr.Bytes()))
		return err
	}
	time.Sleep(time.Second * 2)

	return mgr.Follow()
}

func (mgr *Manager) Follow() error {
	return nil
}

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	// TODO: check SynchronizedToLeader and compare the lags
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(cluster *dcs.Cluster) *dcs.Member {
	ctx := context.TODO()

	hosts := cluster.GetMemberAddrs()
	pools, err := mgr.GetOtherPoolsWithHosts(ctx, hosts)
	if err != nil {
		mgr.Logger.Errorf("Get other pools failed, err:%v", err)
		return nil
	}

	for i, pool := range pools {
		if pool != nil {
			if isLeader, err := mgr.IsLeaderWithPool(ctx, pool); isLeader && err == nil {
				return cluster.GetMemberWithHost(hosts[i])
			}
		}
	}

	return nil
}

func (mgr *Manager) HasOtherHealthyMembers(cluster *dcs.Cluster) []*dcs.Member {
	ctx := context.TODO()
	members := make([]*dcs.Member, 0)
	replicationMode, err := mgr.getReplicationMode(ctx)
	if err != nil {
		mgr.Logger.Errorf("get db replication mode failed:%v", err)
		return members
	}

	if replicationMode == "synchronous" {
		for i, m := range cluster.Members {
			if mgr.IsMemberHealthy(m.Name, cluster) {
				members = append(members, &cluster.Members[i])
			}
		}
	}

	return members
}

func (mgr *Manager) GetOtherPoolsWithHosts(ctx context.Context, hosts []string) ([]*pgxpool.Pool, error) {
	if len(hosts) == 0 {
		return nil, errors.New("Get other pool without hosts")
	}

	var tempConfig *pgxpool.Config
	err := util.DeepCopy(tempConfig, config.pool)
	if err != nil {
		return nil, err
	}

	var tempPool *pgxpool.Pool
	resp := make([]*pgxpool.Pool, 0, len(hosts))
	for i, host := range hosts {
		tempConfig.ConnConfig.Host = host
		tempPool, err = pgxpool.NewWithConfig(ctx, tempConfig)
		if err != nil {
			mgr.Logger.Errorf("unable to ping the DB: %v, host:%s", err, host)
			resp[i] = nil
			continue
		}
		resp[i] = tempPool
	}

	return resp, nil
}
