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

import (
	"fmt"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type Cluster struct {
	ClusterCompName string
	Namespace       string
	Replicas        int32
	HaConfig        *HaConfig
	Leader          *Leader
	Members         []Member
	Switchover      *Switchover
	Extra           map[string]string
	resource        any
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

func (c *Cluster) GetMemberAddrWithPort(member Member) string {
	addr := c.GetMemberAddr(member)
	return fmt.Sprintf("%s:%s", addr, member.DBPort)
}

func (c *Cluster) GetMemberAddr(member Member) string {
	clusterDomain := viper.GetString(constant.KubernetesClusterDomainEnv)
	return fmt.Sprintf("%s.%s-headless.%s.svc.%s", member.Name, c.ClusterCompName, c.Namespace, clusterDomain)
}

func (c *Cluster) GetMemberAddrs() []string {
	hosts := make([]string, len(c.Members))
	for i, member := range c.Members {
		hosts[i] = c.GetMemberAddrWithPort(member)
	}
	return hosts
}

type MemberToDelete struct {
	UID        string
	IsFinished bool
}

type HaConfig struct {
	index              string
	ttl                int
	enable             bool
	maxLagOnSwitchover int64
	DeleteMembers      map[string]MemberToDelete
	resource           any
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

func (c *HaConfig) IsDeleting(member *Member) bool {
	memberToDelete := c.GetMemberToDelete(member)
	return memberToDelete != nil
}

func (c *HaConfig) IsDeleted(member *Member) bool {
	memberToDelete := c.GetMemberToDelete(member)
	if memberToDelete == nil {
		return false
	}
	return memberToDelete.IsFinished
}

func (c *HaConfig) FinishDeleted(member *Member) {
	memberToDelete := c.GetMemberToDelete(member)
	memberToDelete.IsFinished = true
	c.DeleteMembers[member.Name] = *memberToDelete
}

func (c *HaConfig) GetMemberToDelete(member *Member) *MemberToDelete {
	memberToDelete, ok := c.DeleteMembers[member.Name]
	if !ok {
		return nil
	}

	if memberToDelete.UID != member.UID {
		return nil
	}
	return &memberToDelete
}

func (c *HaConfig) AddMemberToDelete(member *Member) {
	memberToDelete := MemberToDelete{
		UID:        member.UID,
		IsFinished: false,
	}
	c.DeleteMembers[member.Name] = memberToDelete
}

type Leader struct {
	DBState     *DBState
	Index       string
	Name        string
	AcquireTime int64
	RenewTime   int64
	TTL         int
	Resource    any
}

type DBState struct {
	OpTimestamp int64
	Extra       map[string]string
}
type Member struct {
	Index     string
	Name      string
	Role      string
	PodIP     string
	DBPort    string
	LorryPort string
	UID       string
	resource  any
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
