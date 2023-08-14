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

package component

import (
	"context"
	"fmt"
	"strings"

	"github.com/dapr/kit/logger"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
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
	// and its cluster's healthty is equal to leader member's heathly.
	IsClusterHealthy(context.Context, *dcs.Cluster) bool

	// Member healthy check
	IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool
	IsCurrentMemberHealthy(context.Context, *dcs.Cluster) bool
	IsMemberLagging(context.Context, *dcs.Cluster, *dcs.Member) bool
	GetDBState(context.Context, *dcs.Cluster, *dcs.Member) *dcs.DBState

	// HasOtherHealthyLeader is applicable only to consensus cluster,
	// where the db's internal role services as the source of truth.
	// for replicationset cluster,  HasOtherHealthyLeader will always be nil.
	HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member
	HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member

	// Functions related to member check.
	IsLeader(context.Context, *dcs.Cluster) (bool, error)
	IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error)
	IsFirstMember() bool

	AddCurrentMemberToCluster(*dcs.Cluster) error
	DeleteMemberFromCluster(*dcs.Cluster, string) error

	// IsPromoted is applicable only to consensus cluster, which is used to
	// check if DB has complete switchover.
	// for replicationset cluster,  it will always be true.
	IsPromoted(context.Context) bool
	// Functions related to HA
	// The functions should be idempotent, indicating that if they have been executed in one ha cycle,
	// any subsequent calls during that cycle will have no effect.
	Promote(context.Context) error
	Demote(context.Context) error
	Follow(context.Context, *dcs.Cluster) error
	Recover(context.Context) error

	GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	// IsHealthiestMember(*dcs.Cluster) bool

	GetCurrentMemberName() string
	GetMemberAddrs(*dcs.Cluster) []string

	// Functions related to account manage
	IsRootCreated(context.Context) (bool, error)
	CreateRoot(context.Context) error

	// Readonly lock for disk full
	Lock(context.Context, string) error
	Unlock(context.Context) error

	PreDelete(context.Context) error

	GetLogger() logger.Logger
}

var managers = make(map[string]DBManager)

type DBManagerBase struct {
	CurrentMemberName string
	ClusterCompName   string
	Namespace         string
	DataDir           string
	Logger            logger.Logger
	DBStartupReady    bool
	IsLocked          bool
	DBState           *dcs.DBState
}

func (mgr *DBManagerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *DBManagerBase) GetLogger() logger.Logger {
	return mgr.Logger
}

func (mgr *DBManagerBase) GetCurrentMemberName() string {
	return mgr.CurrentMemberName
}

func (mgr *DBManagerBase) IsFirstMember() bool {
	return strings.HasSuffix(mgr.CurrentMemberName, "-0")
}

func (mgr *DBManagerBase) IsPromoted(context.Context) bool {
	return true
}

func (mgr *DBManagerBase) IsClusterHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (mgr *DBManagerBase) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	return nil
}

func (mgr *DBManagerBase) IsMemberLagging(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	return false
}

func (mgr *DBManagerBase) GetDBState(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) *dcs.DBState {
	return nil
}

func (mgr *DBManagerBase) PreDelete(context.Context) error {
	return nil
}

func (mgr *DBManagerBase) Demote(context.Context) error {
	return nil
}

func (mgr *DBManagerBase) Follow(context.Context, *dcs.Cluster) error {
	return nil
}

func RegisterManager(characterType string, manager DBManager) {
	characterType = strings.ToLower(characterType)
	managers[characterType] = manager
}

func GetManager(characterType string) DBManager {
	characterType = strings.ToLower(characterType)
	return managers[characterType]
}

func GetDefaultManager() (DBManager, error) {
	characterType := viper.GetString("KB_SERVICE_CHARACTER_TYPE")
	if characterType == "" {
		return nil, fmt.Errorf("KB_SERVICE_CHARACTER_TYPE not set")
	}

	return GetManager(characterType), nil
}

type FakeManager struct {
	DBManagerBase
}

var _ DBManager = &FakeManager{}

func (*FakeManager) IsRunning() bool {
	return true
}

func (*FakeManager) IsDBStartupReady() bool {
	return true
}

func (*FakeManager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}
func (*FakeManager) IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool {
	return true
}

func (*FakeManager) IsCurrentMemberHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (*FakeManager) IsClusterHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (*FakeManager) IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool {
	return true
}

func (*FakeManager) HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member {
	return nil
}

func (*FakeManager) HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member {
	return nil
}

func (*FakeManager) IsLeader(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) IsFirstMember() bool {
	return true
}

func (*FakeManager) AddCurrentMemberToCluster(*dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) DeleteMemberFromCluster(*dcs.Cluster, string) error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Promote(context.Context) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) IsPromoted(context.Context) bool {
	return true
}

func (*FakeManager) Demote(context.Context) error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Follow(context.Context, *dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) Recover(context.Context) error {
	return nil

}

func (*FakeManager) GetHealthiestMember(*dcs.Cluster, string) *dcs.Member {
	return nil
}

func (*FakeManager) GetMemberAddrs(*dcs.Cluster) []string {
	return nil
}

func (*FakeManager) IsRootCreated(context.Context) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*FakeManager) CreateRoot(context.Context) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) Lock(context.Context, string) error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Unlock(context.Context) error {
	return fmt.Errorf("NotSuppported")
}
