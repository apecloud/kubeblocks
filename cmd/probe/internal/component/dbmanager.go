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

	"github.com/go-logr/logr"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type DBManager interface {
	IsRunning() bool

	IsDBStartupReady() bool

	// Functions related to cluster initialization.
	InitializeCluster(context.Context, *dcs.Cluster) error
	IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error)
	IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool

	// Functions related to cluster healthy check.
	IsCurrentMemberHealthy(context.Context) bool
	IsClusterHealthy(context.Context, *dcs.Cluster) bool

	// Member healthy check
	IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool
	HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member
	HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member

	// Functions related to member check.
	IsLeader(context.Context, *dcs.Cluster) (bool, error)
	IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error)
	IsFirstMember() bool

	AddCurrentMemberToCluster(*dcs.Cluster) error
	DeleteMemberFromCluster(*dcs.Cluster, string) error

	// Functions related to HA
	Promote() error
	Demote() error
	Follow(*dcs.Cluster) error
	Recover()

	GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	// IsHealthiestMember(*dcs.Cluster) bool

	GetCurrentMemberName() string
	GetMemberAddrs(*dcs.Cluster) []string
	GetLogger() logr.Logger

	// Functions related to account manage
	IsRootCreated(context.Context) (bool, error)
	CreateRoot(context.Context) error

	// Readonly lock for disk full
	Lock(context.Context, string) error
	Unlock(context.Context) error
}

var managers = make(map[string]DBManager)

type DBManagerBase struct {
	CurrentMemberName string
	ClusterCompName   string
	Namespace         string
	DataDir           string
	Logger            logr.Logger
	DBStartupReady    bool
}

func (mgr *DBManagerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *DBManagerBase) GetLogger() logr.Logger {
	return mgr.Logger
}

func (mgr *DBManagerBase) GetCurrentMemberName() string {
	return mgr.CurrentMemberName
}

func (mgr *DBManagerBase) IsFirstMember() bool {
	return strings.HasSuffix(mgr.CurrentMemberName, "-0")
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

func (*FakeManager) IsCurrentMemberHealthy(context.Context) bool {
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

func (*FakeManager) Promote() error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) Demote() error {
	return fmt.Errorf("NotSuppported")
}

func (*FakeManager) Follow(*dcs.Cluster) error {
	return fmt.Errorf("NotSupported")
}

func (*FakeManager) Recover() {

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
