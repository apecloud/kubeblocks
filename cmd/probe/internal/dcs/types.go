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

package dcs

import "fmt"

type Cluster struct {
	ClusterCompName string
	Namespace       string
	Replicas        int32
	HaConfig        *HaConfig
	Leader          *Leader
	OpTime          int64
	Members         []Member
	Switchover      *Switchover
	Extra           map[string]string
	resource        interface{}
}

func (c *Cluster) HasMember(memberName string) bool {
	for _, member := range c.Members {
		if memberName == member.Name {
			return true
		}
	}
	return false
}

func (c *Cluster) GetLeaderMember() *Member {
	if c.Leader == nil || c.Leader.Name == "" {
		return nil
	}

	return c.GetMemberWithName(c.Leader.Name)
}

func (c *Cluster) GetMemberWithName(name string) *Member {
	for _, m := range c.Members {
		if m.Name == name {
			return &m
		}
	}

	return nil
}

func (c *Cluster) GetMemberWithHost(host string) *Member {
	for _, m := range c.Members {
		if host == c.GetMemberAddr(m) {
			return &m
		}
	}

	return nil
}

func (c *Cluster) GetMemberName() []string {
	var memberList []string
	for _, member := range c.Members {
		memberList = append(memberList, member.Name)
	}

	return memberList
}

func (c *Cluster) IsLocked() bool {
	return c.Leader != nil && c.Leader.Name != ""
}

func (c *Cluster) GetOpTime() int64 {
	return c.OpTime
}

func (c *Cluster) GetMemberAddrWithPort(member Member) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc.cluster.local:%s", member.Name, c.ClusterCompName, c.Namespace, member.DBPort)
}

func (c *Cluster) GetMemberAddr(member Member) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc.cluster.local", member.Name, c.ClusterCompName, c.Namespace)
}

func (c *Cluster) GetMemberAddrs() []string {
	hosts := make([]string, len(c.Members))
	for i, member := range c.Members {
		hosts[i] = c.GetMemberAddrWithPort(member)
	}
	return hosts
}

type HaConfig struct {
	index              string
	ttl                int
	enable             bool
	maxLagOnSwitchover int64
}

func (c *HaConfig) GetTTL() int {
	return c.ttl
}

func (c *HaConfig) IsEnable() bool {
	return c.enable
}

func (c *HaConfig) GetMaxLagOnSwitchover() int64 {
	return c.maxLagOnSwitchover
}

type Leader struct {
	Index       string
	Name        string
	AcquireTime int64
	RenewTime   int64
	TTL         int
	Resource    interface{}
}

type Member struct {
	Index          string
	Name           string
	Role           string
	PodIP          string
	DBPort         string
	SQLChannelPort string
}

func (m *Member) GetName() string {
	return m.Name
}

// func newMember(index string, name string, role string, url string) *Member {
// 	return &Member{
// 		Index: index,
// 		Name:  name,
// 		Role:  role,
// 	}
// }

type Switchover struct {
	Index       string
	Leader      string
	Candidate   string
	ScheduledAt int64
}

func newSwitchover(index string, leader string, candidate string, scheduledAt int64) *Switchover {
	return &Switchover{
		Index:       index,
		Leader:      leader,
		Candidate:   candidate,
		ScheduledAt: scheduledAt,
	}
}

func (s *Switchover) GetLeader() string {
	return s.Leader
}

func (s *Switchover) GetCandidate() string {
	return s.Candidate
}
