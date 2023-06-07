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
	v1 "k8s.io/api/core/v1"
	"strconv"
)

type Cluster struct {
	SysID      string
	Config     *ClusterConfig
	Leader     *Leader
	OpTime     int64
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

func (c *Cluster) GetMemberWithName(name string) *Member {
	for _, m := range c.Members {
		if m.name == name {
			return m
		}
	}

	return nil
}

func (c *Cluster) GetMemberName() []string {
	var memberList []string
	for _, member := range c.Members {
		memberList = append(memberList, member.name)
	}

	return memberList
}

func (c *Cluster) IsLocked() bool {
	return c.Leader != nil && c.Leader.member != nil && c.Leader.member.name != ""
}

func (c *Cluster) GetOpTime() int64 {
	return c.OpTime
}

type ClusterConfig struct {
	index string
	data  *ClusterData
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
		index: configmap.ResourceVersion,
		data:  data,
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

func (m *Member) GetName() string {
	return m.name
}

func getMemberFromPod(pod *v1.Pod) *Member {
	member := newMember(pod.ResourceVersion, pod.Name, pod.Labels, pod.Annotations)
	member.podLabel = pod.Labels
	return member
}

func newMember(index string, name string, labels map[string]string, annotations map[string]string) *Member {
	return &Member{
		index: index,
		name:  name,
		data: &MemberData{
			role: labels[KbRoleLabel],
			url:  annotations[Url],
		},
	}
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

func (s *Switchover) GetLeader() string {
	return s.leader
}

func (s *Switchover) GetCandidate() string {
	return s.candidate
}

type MemberData struct {
	role string
	url  string
}

func (m *MemberData) GetUrl() string {
	return m.url
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
