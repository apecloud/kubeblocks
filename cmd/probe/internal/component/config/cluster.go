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

package config

import "time"

type Cluster struct {
	Initialize string
	Config     *ClusterConfig
	Leader     *Leader
	LastLsn    int64     //包含最后一个已知领导者 LSN 的位置
	Members    []*Member //Member对象列表，包括leader在内的所有PostgreSQL集群成员
	FailOver   *FailOver
	Sync       *SyncState
	// TODO: history,slots,failsafe,workers'
}

type ClusterConfig struct {
	index       int64 //对象上一次修改的索引ID
	modifyIndex int64 //会话id或者时间戳
	//data
}

type Leader struct {
	index   int64
	session int64
	Member  *Member
}

type Member struct {
	index   int64
	name    string
	session int64
	data    *MemberData
}

func (m *Member) GetName() string {
	return m.name
}

// FailOver 对象，记录即将发生的failover操作信息
type FailOver struct {
	index       int64
	leader      string
	candidate   string
	scheduledAt time.Time
}

// SyncState 最后观察到的同步复制状态。
type SyncState struct {
	index       int64
	leader      *Leader
	syncStandby string // synchronous standby list (comma delimited) which are last synchronized to leader
}

func (s *SyncState) SynchronizedToLeader(candidate string) bool {
	// TODO: how to maintain candidate state
	return false
}

type MemberData struct {
	// TODO
}
