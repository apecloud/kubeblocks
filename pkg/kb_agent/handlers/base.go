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

package handlers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
)

type HandlerBase struct {
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

func NewHandlerBase(logger logr.Logger) (*HandlerBase, error) {
	currentMemberName := viper.GetString(constant.KBEnvPodName)
	if currentMemberName == "" {
		return nil, fmt.Errorf("%s is not set", constant.KBEnvPodName)
	}

	mgr := HandlerBase{
		CurrentMemberName: currentMemberName,
		CurrentMemberIP:   viper.GetString(constant.KBEnvPodIP),
		ClusterCompName:   viper.GetString(constant.KBEnvClusterCompName),
		Namespace:         viper.GetString(constant.KBEnvNamespace),
		Logger:            logger,
	}
	return &mgr, nil
}

func (mgr *HandlerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *HandlerBase) GetLogger() logr.Logger {
	return mgr.Logger
}

func (mgr *HandlerBase) SetLogger(logger logr.Logger) {
	mgr.Logger = logger
}

func (mgr *HandlerBase) GetCurrentMemberName() string {
	return mgr.CurrentMemberName
}

func (mgr *HandlerBase) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return false, models.ErrNotImplemented
}

func (mgr *HandlerBase) MemberHealthyCheck(context.Context, *dcs.Cluster, *dcs.Member) error {
	return nil
}

func (mgr *HandlerBase) JoinMember(context.Context, *dcs.Cluster, string) error {
	return nil
}

func (mgr *HandlerBase) LeaveMember(context.Context, *dcs.Cluster, string) error {
	return nil
}

func (mgr *HandlerBase) GetLag(context.Context, *dcs.Cluster) (int64, error) {
	return 0, models.ErrNotImplemented
}

func (mgr *HandlerBase) MoveData(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *HandlerBase) GetReplicaRole(context.Context, *dcs.Cluster) (string, error) {
	return "", models.ErrNotImplemented
}

func (mgr *HandlerBase) Exec(context.Context, string) (int64, error) {
	return 0, models.ErrNotImplemented
}

func (mgr *HandlerBase) Query(context.Context, string) ([]byte, error) {
	return []byte{}, models.ErrNotImplemented
}

func (mgr *HandlerBase) GetPort() (int, error) {
	return 0, models.ErrNotImplemented
}

func (mgr *HandlerBase) ListUsers(context.Context) ([]models.UserInfo, error) {
	return nil, models.ErrNotImplemented
}

func (mgr *HandlerBase) ListSystemAccounts(context.Context) ([]models.UserInfo, error) {
	return nil, models.ErrNotImplemented
}

func (mgr *HandlerBase) CreateUser(context.Context, string, string) error {
	return models.ErrNotImplemented
}

func (mgr *HandlerBase) DeleteUser(context.Context, string) error {
	return models.ErrNotImplemented
}

func (mgr *HandlerBase) DescribeUser(context.Context, string) (*models.UserInfo, error) {
	return nil, models.ErrNotImplemented
}

func (mgr *HandlerBase) GrantUserRole(context.Context, string, string) error {
	return models.ErrNotImplemented
}

func (mgr *HandlerBase) RevokeUserRole(context.Context, string, string) error {
	return models.ErrNotImplemented
}

func (mgr *HandlerBase) IsRunning() bool {
	return false
}

func (mgr *HandlerBase) Lock(context.Context, string) error {
	return models.ErrNotImplemented
}

func (mgr *HandlerBase) Unlock(context.Context) error {
	return models.ErrNotImplemented
}

func (mgr *HandlerBase) ShutDownWithWait() {
	mgr.Logger.Info("Override me if need")
}
