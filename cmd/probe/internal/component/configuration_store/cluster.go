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

package configuration_store

import (
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
)

type Cluster struct {
	SysID      string
	Config     *ClusterConfig
	Leader     *Leader
	Members    []*Member
	Switchover *Switchover
	Extra      map[string]string
}

func (c *Cluster) HasMember(memberName string) bool {
	for _, member := range c.Members {
		if memberName == member.name {
			return true
		}
	}
	return false
}

func (c *Cluster) GetMemberName() []string {
	var memberList []string
	for _, member := range c.Members {
		memberList = append(memberList, member.name)
	}

	return memberList
}

type ClusterConfig struct {
	index       string
	modifyIndex int64
	data        *ClusterData
}

func (c *ClusterConfig) GetData() *ClusterData {
	return c.data
}

func getClusterConfigFromConfigMap(configmap *v1.ConfigMap) *ClusterConfig {
	annotations := configmap.Annotations
	ttl, err := strconv.Atoi(annotations[TTL])
	if err != nil {
		ttl = 0
	}
	maxLagOnSwitchover, err := strconv.Atoi(annotations[MaxLagOnSwitchover])
	if err != nil {
		maxLagOnSwitchover = 1048576
	}

	data := newClusterData(int64(ttl), int64(maxLagOnSwitchover))

	return &ClusterConfig{
		index:       configmap.ResourceVersion,
		modifyIndex: 0, // TODO: is modifyIndex need?
		data:        data,
	}
}

type Leader struct {
	index  string
	member *Member
}

func (l *Leader) GetMember() *Member {
	return l.member
}

func newLeader(index string, member *Member) *Leader {
	return &Leader{
		index:  index,
		member: member,
	}
}

type Member struct {
	index    string
	name     string
	data     *MemberData
	podLabel map[string]string
}

func (m *Member) GetData() *MemberData {
	return m.data
}

func getMemberFromPod(pod *v1.Pod) *Member {
	member := newMember(pod.ResourceVersion, pod.Name, pod.Labels)
	member.podLabel = pod.Labels
	return member
}

func newMember(index string, name string, labels map[string]string) *Member {
	return &Member{
		index: index,
		name:  name,
		data: &MemberData{
			role: labels[KbRoleLabel],
		},
	}
}

func (m *Member) GetName() string {
	return m.name
}

type Switchover struct {
	index       string
	leader      string
	candidate   string
	scheduledAt int64
}

func newSwitchover(index string, leader string, candidate string, scheduledAt int64) *Switchover {
	return &Switchover{
		index:       index,
		leader:      leader,
		candidate:   candidate,
		scheduledAt: scheduledAt,
	}
}

// SyncState 最后观察到的同步复制状态。
type SyncState struct {
	index       string
	leader      string
	syncStandby []string // synchronous standby list which are last synchronized to leader
}

func newSyncState(index string, leader string, syncStandby string) *SyncState {
	standby := strings.Split(syncStandby, ",")
	return &SyncState{
		index:       index,
		leader:      leader,
		syncStandby: standby,
	}
}

func (s *SyncState) SynchronizedToLeader(candidate string) bool {
	return slices.Contains(s.syncStandby, candidate)
}

type MemberData struct {
	role string
}

type TimelineHistory struct {
	index int64
	value int64
	lines []string
}

type ClusterData struct {
	ttl                int64
	maxLagOnSwitchover int64
}

func newClusterData(ttl int64, maxLagOnSwitchover int64) *ClusterData {
	return &ClusterData{
		ttl:                ttl,
		maxLagOnSwitchover: maxLagOnSwitchover,
	}
}

func (c *ClusterData) GetTtl() int64 {
	return c.ttl
}

func (c *ClusterData) GetMaxLagOnSwitchover() int64 {
	return c.maxLagOnSwitchover
}
