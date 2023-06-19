package dcs

import appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"

type Cluster struct {
	ClusterCompName string
	Replicas        int32
	HaConfig        *HaConfig
	//Leader     *Leader
	OpTime          int64
	Members         []Member
	Switchover      *Switchover
	Extra           map[string]string
	clusterResource *appsv1alpha1.Cluster
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
			return &m
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
	return false //c.Leader != nil && c.Leader.member != nil && c.Leader.member.name != ""
}

func (c *Cluster) GetOpTime() int64 {
	return c.OpTime
}

type HaConfig struct {
	index              string
	ttl                int
	maxLagOnSwitchover int64
}

func (c *HaConfig) GetTtl() int {
	return c.ttl
}

func (c *HaConfig) GetMaxLagOnSwitchover() int64 {
	return c.maxLagOnSwitchover
}

type Leader struct {
	index       string
	name        string
	acquireTime int64
	renewTime   int64
	ttl         int
}

type Member struct {
	index          string
	name           string
	role           string
	PodIP          string
	DBPort         string
	SQLChannelPort string
}

func (m *Member) GetName() string {
	return m.name
}

func newMember(index string, name string, role string, url string) *Member {
	return &Member{
		index: index,
		name:  name,
		role:  role,
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
