/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

type DBManagerBase struct {
	CurrentMemberName string
	CurrentMemberIP   string
	ClusterCompName   string
	Namespace         string
	DataDir           string
	Logger            logr.Logger
	DBStartupReady    bool
	IsLocked          bool
	DBState           *dcs.DBState
}

func NewDBManagerBase(logger logr.Logger) (*DBManagerBase, error) {
	currentMemberName := viper.GetString(constant.KBEnvPodName)
	if currentMemberName == "" {
		return nil, fmt.Errorf("%s is not set", constant.KBEnvPodName)
	}

	mgr := DBManagerBase{
		CurrentMemberName: currentMemberName,
		CurrentMemberIP:   viper.GetString(constant.KBEnvPodIP),
		ClusterCompName:   viper.GetString(constant.KBEnvClusterCompName),
		Namespace:         viper.GetString(constant.KBEnvNamespace),
		Logger:            logger,
	}
	return &mgr, nil
}

func (mgr *DBManagerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *DBManagerBase) GetLogger() logr.Logger {
	return mgr.Logger
}

func (mgr *DBManagerBase) SetLogger(logger logr.Logger) {
	mgr.Logger = logger
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

func (mgr *DBManagerBase) Promote(context.Context, *dcs.Cluster) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) Demote(context.Context) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) Follow(context.Context, *dcs.Cluster) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) Recover(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *DBManagerBase) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return false, models.ErrNotImplemented
}

func (mgr *DBManagerBase) IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error) {
	return false, models.ErrNotImplemented
}

func (mgr *DBManagerBase) GetMemberAddrs(context.Context, *dcs.Cluster) []string {
	return nil
}

func (mgr *DBManagerBase) InitializeCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *DBManagerBase) IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error) {
	return true, nil
}

func (mgr *DBManagerBase) IsClusterHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (mgr *DBManagerBase) MemberHealthyCheck(context.Context, *dcs.Cluster, *dcs.Member) error {
	return nil
}

func (mgr *DBManagerBase) LeaderHealthyCheck(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *DBManagerBase) CurrentMemberHealthyCheck(ctx context.Context, cluster *dcs.Cluster) error {
	member := cluster.GetMemberWithName(mgr.CurrentMemberName)
	return mgr.MemberHealthyCheck(ctx, cluster, member)
}

func (mgr *DBManagerBase) HasOtherHealthyLeader(context.Context, *dcs.Cluster) *dcs.Member {
	return nil
}

func (mgr *DBManagerBase) HasOtherHealthyMembers(context.Context, *dcs.Cluster, string) []*dcs.Member {
	return nil
}

func (mgr *DBManagerBase) IsMemberHealthy(context.Context, *dcs.Cluster, *dcs.Member) bool {
	return false
}

func (mgr *DBManagerBase) IsCurrentMemberHealthy(context.Context, *dcs.Cluster) bool {
	return true
}

func (mgr *DBManagerBase) IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool {
	return true
}

func (mgr *DBManagerBase) JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *DBManagerBase) LeaveMemberFromCluster(context.Context, *dcs.Cluster, string) error {
	return nil
}

func (mgr *DBManagerBase) IsMemberLagging(context.Context, *dcs.Cluster, *dcs.Member) (bool, int64) {
	return false, 0
}

func (mgr *DBManagerBase) GetLag(context.Context, *dcs.Cluster) (int64, error) {
	return 0, models.ErrNotImplemented
}

func (mgr *DBManagerBase) GetDBState(context.Context, *dcs.Cluster) *dcs.DBState {
	// mgr.DBState = DBState
	return nil
}

func (mgr *DBManagerBase) MoveData(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *DBManagerBase) GetReplicaRole(context.Context, *dcs.Cluster) (string, error) {
	return "", models.ErrNotImplemented
}

func (mgr *DBManagerBase) Exec(context.Context, string) (int64, error) {
	return 0, models.ErrNotImplemented
}

func (mgr *DBManagerBase) Query(context.Context, string) ([]byte, error) {
	return []byte{}, models.ErrNotImplemented
}

func (mgr *DBManagerBase) GetPort() (int, error) {
	return 0, models.ErrNotImplemented
}

func (mgr *DBManagerBase) IsRootCreated(context.Context) (bool, error) {
	return true, nil
}

func (mgr *DBManagerBase) ListUsers(context.Context) ([]models.UserInfo, error) {
	return nil, models.ErrNotImplemented
}

func (mgr *DBManagerBase) ListSystemAccounts(context.Context) ([]models.UserInfo, error) {
	return nil, models.ErrNotImplemented
}

func (mgr *DBManagerBase) CreateUser(context.Context, string, string) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) DeleteUser(context.Context, string) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) DescribeUser(context.Context, string) (*models.UserInfo, error) {
	return nil, models.ErrNotImplemented
}

func (mgr *DBManagerBase) GrantUserRole(context.Context, string, string) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) RevokeUserRole(context.Context, string, string) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) IsRunning() bool {
	return false
}

func (mgr *DBManagerBase) Lock(context.Context, string) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) Unlock(context.Context) error {
	return models.ErrNotImplemented
}

func (mgr *DBManagerBase) Start(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *DBManagerBase) Stop() error {
	return nil
}

func (mgr *DBManagerBase) CreateRoot(context.Context) error {
	return nil
}

func (mgr *DBManagerBase) ShutDownWithWait() {
	mgr.Logger.Info("Override me if need")
}
