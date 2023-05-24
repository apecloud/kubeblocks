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

import (
	"golang.org/x/exp/slices"
	"time"
)

type Cluster struct {
	sysID    string
	Config   *ClusterConfig
	Leader   *Leader
	Members  []*Member
	FailOver *FailOver
	Sync     *SyncState
	Extra    map[string]string
}

type ClusterConfig struct {
	index       int64
	modifyIndex int64
	data        *clusterData
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
	syncStandby []string // synchronous standby list which are last synchronized to leader
}

func (s *SyncState) SynchronizedToLeader(candidate string) bool {
	return slices.Contains(s.syncStandby, candidate)
}

type MemberData struct {
	connUrl string
	state   string
	role    string
}

type TimelineHistory struct {
	index int64
	value int64
	lines []string
}

type clusterData struct {
	ttl int32
}
