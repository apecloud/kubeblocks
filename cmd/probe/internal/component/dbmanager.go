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

	// Functions related to account manage
	IsRootCreated(context.Context) (bool, error)
	CreateRoot(context.Context) error

	// Readonly lock for disk full
	Lock(context.Context, string) error
	Unlock(context.Context) error

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
