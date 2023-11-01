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

package engines

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

type DBManager interface {
	IsRunning() bool

	IsDBStartupReady() bool

	// Functions related to cluster initialization.
	InitializeCluster(context.Context, *dcs.Cluster) error
	IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error)
	// IsCurrentMemberInCluster checks if current member is configured in cluster for consensus.
	// it will always return true for replicationset.
	IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool

	// IsClusterHealthy is only for consensus cluster healthy check.
	// For Replication cluster IsClusterHealthy will always return true,
	// and its cluster's healthy is equal to leader member's healthy.
	IsClusterHealthy(context.Context, *dcs.Cluster) bool

	// Member healthy check
	// IsMemberHealthy focuses on the database's read and write capabilities.
	IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool
	IsCurrentMemberHealthy(context.Context, *dcs.Cluster) bool
	// IsMemberLagging focuses on the latency between the leader and standby
	IsMemberLagging(context.Context, *dcs.Cluster, *dcs.Member) (bool, int64)
	GetLag(context.Context, *dcs.Cluster) (int64, error)

	// GetDBState will get most required database kernel states of current member in one HA loop to Avoiding duplicate queries and conserve I/O.
	// We believe that the states of database kernel remains unchanged within a single HA loop.
	GetDBState(context.Context, *dcs.Cluster) *dcs.DBState

	// HasOtherHealthyLeader is applicable only to consensus cluster,
	// where the db's internal role services as the source of truth.
	// for replicationset cluster,  HasOtherHealthyLeader will always be nil.
	HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member
	HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member

	// Functions related to replica member relationship.
	IsLeader(context.Context, *dcs.Cluster) (bool, error)
	IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error)
	IsFirstMember() bool
	GetReplicaRole(context.Context, *dcs.Cluster) (string, error)

	JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error
	LeaveMemberFromCluster(context.Context, *dcs.Cluster, string) error

	// IsPromoted is applicable only to consensus cluster, which is used to
	// check if DB has complete switchover.
	// for replicationset cluster,  it will always be true.
	IsPromoted(context.Context) bool
	// Functions related to HA
	// The functions should be idempotent, indicating that if they have been executed in one ha cycle,
	// any subsequent calls during that cycle will have no effect.
	Promote(context.Context, *dcs.Cluster) error
	Demote(context.Context) error
	Follow(context.Context, *dcs.Cluster) error
	Recover(context.Context) error

	// Start and Stop just send signal to lorryctl
	Start(context.Context, *dcs.Cluster) error
	Stop() error

	// GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	// IsHealthiestMember(*dcs.Cluster) bool

	GetCurrentMemberName() string
	GetMemberAddrs(context.Context, *dcs.Cluster) []string

	// Functions related to account manage
	IsRootCreated(context.Context) (bool, error)
	CreateRoot(context.Context) error

	// Readonly lock for disk full
	Lock(context.Context, string) error
	Unlock(context.Context) error

	// sql query
	Exec(context.Context, string) (int64, error)
	Query(context.Context, string) ([]byte, error)

	// user management
	ListUsers(context.Context) ([]models.UserInfo, error)
	ListSystemAccounts(context.Context) ([]models.UserInfo, error)
	CreateUser(context.Context, string, string) error
	DeleteUser(context.Context, string) error
	DescribeUser(context.Context, string) (*models.UserInfo, error)
	GrantUserRole(context.Context, string, string) error
	RevokeUserRole(context.Context, string, string) error

	GetPort() (int, error)

	MoveData(context.Context, *dcs.Cluster) error

	GetLogger() logr.Logger

	ShutDownWithWait()
}
