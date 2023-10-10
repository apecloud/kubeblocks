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

package mongodb

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	MinVotingMembers    = 1
	MaxVotingMembers    = 7
	MaxMembers          = 50
	DefaultPriority     = 2
	DefaultVotes        = 1
	DefaultReadConcern  = "majority"
	DefaultWriteConcern = "majority"
)

// ReplsetTags Set tags: https://docs.mongodb.com/manual/tutorial/configure-replica-set-tag-sets/#add-tag-sets-to-a-replica-set
type ReplsetTags map[string]string

// ConfigMember document from 'replSetGetConfig': https://docs.mongodb.com/manual/reference/command/replSetGetConfig/#dbcmd.replSetGetConfig
type ConfigMember struct {
	ID                 int         `bson:"_id" json:"_id"`
	Host               string      `bson:"host" json:"host"`
	ArbiterOnly        *bool       `bson:"arbiterOnly,omitempty" json:"arbiterOnly,omitempty"`
	BuildIndexes       *bool       `bson:"buildIndexes,omitempty" json:"buildIndexes,omitempty"`
	Hidden             *bool       `bson:"hidden,omitempty" json:"hidden,omitempty"`
	Priority           int         `bson:"priority,omitempty" json:"priority,omitempty"`
	Tags               ReplsetTags `bson:"tags,omitempty" json:"tags,omitempty"`
	SlaveDelay         *int64      `bson:"slaveDelay,omitempty" json:"slaveDelay,omitempty"`
	SecondaryDelaySecs *int64      `bson:"secondaryDelaySecs,omitempty" json:"secondaryDelaySecs,omitempty"`
	Votes              *int        `bson:"votes,omitempty" json:"votes,omitempty"`
}

type ConfigMembers []ConfigMember

type RSConfig struct {
	ID                                 string        `bson:"_id,omitempty" json:"_id,omitempty"`
	Version                            int           `bson:"version,omitempty" json:"version,omitempty"`
	Members                            ConfigMembers `bson:"members" json:"members"`
	Configsvr                          bool          `bson:"configsvr,omitempty" json:"configsvr,omitempty"`
	ProtocolVersion                    int           `bson:"protocolVersion,omitempty" json:"protocolVersion,omitempty"`
	Settings                           Settings      `bson:"-" json:"settings,omitempty"`
	WriteConcernMajorityJournalDefault bool          `bson:"writeConcernMajorityJournalDefault,omitempty" json:"writeConcernMajorityJournalDefault,omitempty"`
}

// Settings document from 'replSetGetConfig': https://docs.mongodb.com/manual/reference/command/replSetGetConfig/#dbcmd.replSetGetConfig
type Settings struct {
	ChainingAllowed         bool                   `bson:"chainingAllowed,omitempty" json:"chainingAllowed,omitempty"`
	HeartbeatIntervalMillis int64                  `bson:"heartbeatIntervalMillis,omitempty" json:"heartbeatIntervalMillis,omitempty"`
	HeartbeatTimeoutSecs    int                    `bson:"heartbeatTimeoutSecs,omitempty" json:"heartbeatTimeoutSecs,omitempty"`
	ElectionTimeoutMillis   int64                  `bson:"electionTimeoutMillis,omitempty" json:"electionTimeoutMillis,omitempty"`
	CatchUpTimeoutMillis    int64                  `bson:"catchUpTimeoutMillis,omitempty" json:"catchUpTimeoutMillis,omitempty"`
	GetLastErrorModes       map[string]ReplsetTags `bson:"getLastErrorModes,omitempty" json:"getLastErrorModes,omitempty"`
	GetLastErrorDefaults    WriteConcern           `bson:"getLastErrorDefaults,omitempty" json:"getLastErrorDefaults,omitempty"`
	ReplicaSetID            primitive.ObjectID     `bson:"replicaSetId,omitempty" json:"replicaSetId,omitempty"`
}

// ReplSetGetConfig Response document from 'replSetGetConfig': https://docs.mongodb.com/manual/reference/command/replSetGetConfig/#dbcmd.replSetGetConfig
type ReplSetGetConfig struct {
	Config     *RSConfig `bson:"config" json:"config"`
	OKResponse `bson:",inline"`
}

// BuildInfo contains information about mongod build params
type BuildInfo struct {
	Version    string `json:"version" bson:"version"`
	OKResponse `bson:",inline"`
}

// OKResponse is a standard MongoDB response
type OKResponse struct {
	Errmsg string `bson:"errmsg,omitempty" json:"errmsg,omitempty"`
	OK     int    `bson:"ok" json:"ok"`
}

// WriteConcern document: https://docs.mongodb.com/manual/reference/write-concern/
type WriteConcern struct {
	WriteConcern interface{} `bson:"w" json:"w"`
	WriteTimeout int         `bson:"wtimeout" json:"wtimeout"`
	Journal      bool        `bson:"j,omitempty" json:"j,omitempty"`
}

type BalancerStatus struct {
	Mode       string `json:"mode"`
	OKResponse `bson:",inline"`
}

type LockResp struct {
	Info       string `bson:"info" json:"info"`
	LockCount  int64  `bson:"lockCount" json:"lockCount"`
	OKResponse `bson:",inline"`
}

type DBList struct {
	DBs []struct {
		Name string `bson:"name" json:"name"`
	} `bson:"databases" json:"databases"`
	OKResponse `bson:",inline"`
}

type ShardList struct {
	Shards []struct {
		ID    string `json:"_id" bson:"_id"`
		Host  string `json:"host" bson:"host"`
		State int    `json:"state" bson:"state"`
	} `json:"shards" bson:"shards"`
	OKResponse `bson:",inline"`
}

type FCV struct {
	FCV struct {
		Version string `json:"version" bson:"version"`
	} `json:"featureCompatibilityVersion" bson:"featureCompatibilityVersion"`
	OKResponse `bson:",inline"`
}

const ShardRemoveCompleted string = "completed"

type ShardRemoveResp struct {
	Msg       string `json:"msg" bson:"msg"`
	State     string `json:"state" bson:"state"`
	Remaining struct {
		Chunks      int `json:"chunks" bson:"chunks"`
		JumboChunks int `json:"jumboChunks" bson:"jumboChunks"`
	} `json:"remaining" bson:"remaining"`
	OKResponse `bson:",inline"`
}

type IsMasterResp struct {
	IsMaster   bool   `bson:"ismaster" json:"ismaster"`
	IsArbiter  bool   `bson:"arbiterOnly" json:"arbiterOnly"`
	Primary    string `bson:"primary" json:"primary"`
	Me         string `bson:"me" json:"me"`
	Msg        string `bson:"msg" json:"msg"`
	OKResponse `bson:",inline"`
}

type ReplSetStatus struct {
	Set                     string         `bson:"set" json:"set"`
	Date                    time.Time      `bson:"date" json:"date"`
	MyState                 MemberState    `bson:"myState" json:"myState"`
	Members                 []*Member      `bson:"members" json:"members"`
	Term                    int64          `bson:"term,omitempty" json:"term,omitempty"`
	HeartbeatIntervalMillis int64          `bson:"heartbeatIntervalMillis,omitempty" json:"heartbeatIntervalMillis,omitempty"`
	Optimes                 *StatusOptimes `bson:"optimes,omitempty" json:"optimes,omitempty"`
	OKResponse              `bson:",inline"`
}

type Member struct {
	ID                int                 `bson:"_id" json:"_id"`
	Name              string              `bson:"name" json:"name"`
	Health            MemberHealth        `bson:"health" json:"health"`
	State             MemberState         `bson:"state" json:"state"`
	StateStr          string              `bson:"stateStr" json:"stateStr"`
	Uptime            int64               `bson:"uptime" json:"uptime"`
	Optime            *Optime             `bson:"optime" json:"optime"`
	OptimeDate        time.Time           `bson:"optimeDate" json:"optimeDate"`
	ConfigVersion     int                 `bson:"configVersion" json:"configVersion"`
	ElectionTime      primitive.Timestamp `bson:"electionTime,omitempty" json:"electionTime,omitempty"`
	ElectionDate      time.Time           `bson:"electionDate,omitempty" json:"electionDate,omitempty"`
	InfoMessage       string              `bson:"infoMessage,omitempty" json:"infoMessage,omitempty"`
	OptimeDurable     *Optime             `bson:"optimeDurable,omitempty" json:"optimeDurable,omitempty"`
	OptimeDurableDate time.Time           `bson:"optimeDurableDate,omitempty" json:"optimeDurableDate,omitempty"`
	LastHeartbeat     time.Time           `bson:"lastHeartbeat,omitempty" json:"lastHeartbeat,omitempty"`
	LastHeartbeatRecv time.Time           `bson:"lastHeartbeatRecv,omitempty" json:"lastHeartbeatRecv,omitempty"`
	PingMs            int64               `bson:"pingMs,omitempty" json:"pingMs,omitempty"`
	Self              bool                `bson:"self,omitempty" json:"self,omitempty"`
	SyncingTo         string              `bson:"syncingTo,omitempty" json:"syncingTo,omitempty"`
}

func (s *ReplSetStatus) GetSelf() *Member {
	for _, member := range s.Members {
		if member.Self {
			return member
		}
	}
	return nil
}

type Optime struct {
	Timestamp primitive.Timestamp `bson:"ts" json:"ts"`
	Term      int64               `bson:"t" json:"t"`
}

type StatusOptimes struct {
	LastCommittedOpTime *Optime `bson:"lastCommittedOpTime" json:"lastCommittedOpTime"`
	AppliedOpTime       *Optime `bson:"appliedOpTime" json:"appliedOpTime"`
	DurableOptime       *Optime `bson:"durableOpTime" json:"durableOpTime"`
}

type MemberHealth int
type MemberState int

const (
	MemberHealthDown MemberHealth = iota
	MemberHealthUp
	MemberStateStartup    MemberState = 0
	MemberStatePrimary    MemberState = 1
	MemberStateSecondary  MemberState = 2
	MemberStateRecovering MemberState = 3
	MemberStateStartup2   MemberState = 5
	MemberStateUnknown    MemberState = 6
	MemberStateArbiter    MemberState = 7
	MemberStateDown       MemberState = 8
	MemberStateRollback   MemberState = 9
	MemberStateRemoved    MemberState = 10
)

var MemberStateStrings = map[MemberState]string{
	MemberStateStartup:    "STARTUP",
	MemberStatePrimary:    "PRIMARY",
	MemberStateSecondary:  "SECONDARY",
	MemberStateRecovering: "RECOVERING",
	MemberStateStartup2:   "STARTUP2",
	MemberStateUnknown:    "UNKNOWN",
	MemberStateArbiter:    "ARBITER",
	MemberStateDown:       "DOWN",
	MemberStateRollback:   "ROLLBACK",
	MemberStateRemoved:    "REMOVED",
}

func (s *ReplSetStatus) GetMembersByState(state MemberState, limit int) []*Member {
	members := make([]*Member, 0)
	for _, member := range s.Members {
		if member.State == state {
			members = append(members, member)
			if limit > 0 && len(members) == limit {
				return members
			}
		}
	}
	return members
}

func (s *ReplSetStatus) Primary() *Member {
	primary := s.GetMembersByState(MemberStatePrimary, 1)
	if len(primary) == 1 {
		return primary[0]
	}
	return nil
}

type RolePrivilege struct {
	Resource map[string]interface{} `bson:"resource" json:"resource"`
	Actions  []string               `bson:"actions" json:"actions"`
}

type Role struct {
	Role       string                   `bson:"role" json:"role"`
	DB         string                   `bson:"db" json:"db"`
	IsBuiltin  string                   `bson:"isBuiltin" json:"isBuiltin"`
	Roles      []map[string]interface{} `bson:"roles" json:"roles"`
	Privileges []RolePrivilege          `bson:"privileges" json:"privileges"`
}

type RoleInfo struct {
	Roles      []Role `bson:"roles" json:"roles"`
	OKResponse `bson:",inline"`
}

type User struct {
	ID    string                   `bson:"_id" json:"_id"`
	User  string                   `bson:"user" json:"user"`
	DB    string                   `bson:"db" json:"db"`
	Roles []map[string]interface{} `bson:"roles" json:"roles"`
}

type UsersInfo struct {
	Users      []User `bson:"users" json:"users"`
	OKResponse `bson:",inline"`
}
