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

package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

// Query is equivalent to QueryWithHost(ctx, sql, ""), query itself.
func (mgr *Manager) Query(ctx context.Context, sql string) (result []byte, err error) {
	return mgr.QueryWithHost(ctx, sql, "")
}

func (mgr *Manager) QueryWithHost(ctx context.Context, sql string, host string) (result []byte, err error) {
	var rows pgx.Rows
	// when host is empty, use manager's connection pool
	if host == "" {
		rows, err = mgr.Pool.Query(ctx, sql)
	} else {
		rows, err = mgr.QueryOthers(ctx, sql, host)
	}
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return nil, err
	}
	defer func() {
		rows.Close()
		_ = rows.Err()
	}()

	result, err = parseRows(rows)
	if err != nil {
		mgr.Logger.Errorf("parse query:%s failed, err:%v", sql, err)
		return nil, err
	}

	return result, nil
}

func (mgr *Manager) QueryOthers(ctx context.Context, sql string, host string) (rows pgx.Rows, err error) {
	conn, err := pgx.Connect(ctx, config.GetConnectURLWithHost(host))
	if err != nil {
		mgr.Logger.Errorf("get host:%s connection failed, err:%v", host, err)
		return nil, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	return conn.Query(ctx, sql)
}

func (mgr *Manager) QueryLeader(ctx context.Context, sql string, cluster *dcs.Cluster) (result []byte, err error) {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return nil, ClusterHasNoLeader
	}

	var host string
	if leaderMember.Name != mgr.CurrentMemberName {
		host = cluster.GetMemberAddr(*leaderMember)
	}
	return mgr.QueryWithHost(ctx, sql, host)
}

// Exec is equivalent to ExecWithHost(ctx, sql, ""), exec itself.
func (mgr *Manager) Exec(ctx context.Context, sql string) (result int64, err error) {
	return mgr.ExecWithHost(ctx, sql, "")
}

func (mgr *Manager) ExecWithHost(ctx context.Context, sql string, host string) (result int64, err error) {
	var res pgconn.CommandTag

	// when host is empty, use manager's connection pool
	if host == "" {
		res, err = mgr.Pool.Exec(ctx, sql)
	} else {
		res, err = mgr.ExecOthers(ctx, sql, host)
	}
	if err != nil {
		return 0, errors.Errorf("exec sql:%s failed: %v", sql, err)
	}

	result = res.RowsAffected()
	return result, nil
}

func (mgr *Manager) ExecOthers(ctx context.Context, sql string, host string) (resp pgconn.CommandTag, err error) {
	conn, err := pgx.Connect(ctx, config.GetConnectURLWithHost(host))
	if err != nil {
		return resp, err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	resp, err = conn.Exec(ctx, sql)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

func (mgr *Manager) ExecLeader(ctx context.Context, sql string, cluster *dcs.Cluster) (result int64, err error) {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return 0, ClusterHasNoLeader
	}

	var host string
	if leaderMember.Name != mgr.CurrentMemberName {
		host = cluster.GetMemberAddr(*leaderMember)
	}
	return mgr.ExecWithHost(ctx, sql, host)
}

func parseRows(rows pgx.Rows) (result []byte, err error) {
	rs := make([]interface{}, 0)
	columnTypes := rows.FieldDescriptions()
	for rows.Next() {
		values := make([]interface{}, len(columnTypes))
		for i := range values {
			values[i] = new(interface{})
		}

		if err = rows.Scan(values...); err != nil {
			return nil, errors.Errorf("scanning row failed, err:%v", err)
		}

		r := map[string]interface{}{}
		for i, ct := range columnTypes {
			r[ct.Name] = values[i]
		}
		rs = append(rs, r)
	}

	if result, err = json.Marshal(rs); err != nil {
		err = errors.Errorf("json marshal failed, err: %v", err)
	}
	return result, err
}

func ParseQuery(str string) (result []map[string]interface{}, err error) {
	// Notice: in golang, json unmarshal will map all numeric types to float64.
	err = json.Unmarshal([]byte(str), &result)
	if err != nil || len(result) == 0 {
		return nil, errors.Errorf("json unmarshal failed, err:%v", err)
	}

	return result, nil
}
