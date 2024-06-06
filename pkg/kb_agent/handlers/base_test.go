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
	"testing"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
)

func TestNewHandlerBase(t *testing.T) {
	logger := logr.Discard()
	_, err := NewHandlerBase(logger)
	assert.Error(t, err)

	viper.Set(constant.KBEnvPodName, "test-pod-0")
	viper.Set(constant.KBEnvClusterCompName, "test")
	viper.Set(constant.KBEnvNamespace, "default")
	mgr, err := NewHandlerBase(logger)
	assert.NoError(t, err)

	assert.Equal(t, viper.GetString(constant.KBEnvPodName), mgr.CurrentMemberName)
	assert.Equal(t, viper.GetString(constant.KBEnvPodIP), mgr.CurrentMemberIP)
	assert.Equal(t, viper.GetString(constant.KBEnvClusterCompName), mgr.ClusterCompName)
	assert.Equal(t, viper.GetString(constant.KBEnvNamespace), mgr.Namespace)
	assert.Equal(t, logger, mgr.Logger)
}

func TestHandlerBase(t *testing.T) {
	logger := logr.Discard()
	viper.Set(constant.KBEnvPodName, "test-pod-0")
	viper.Set(constant.KBEnvClusterCompName, "test")
	viper.Set(constant.KBEnvNamespace, "default")
	mgr, err := NewHandlerBase(logger)
	assert.NoError(t, err)

	assert.False(t, mgr.IsDBStartupReady())

	assert.Equal(t, logger, mgr.GetLogger())

	newLogger := logr.Discard()
	mgr.SetLogger(newLogger)
	assert.Equal(t, newLogger, mgr.GetLogger())

	assert.Equal(t, viper.GetString(constant.KBEnvPodName), mgr.GetCurrentMemberName())

	err = mgr.HealthyCheck(context.Background())
	assert.NoError(t, err)

	err = mgr.JoinMember(context.Background(), "")
	assert.NoError(t, err)

	err = mgr.LeaveMember(context.Background(), "")
	assert.NoError(t, err)

	err = mgr.DataLoad(context.Background())
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.DataDump(context.Background())
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.Switchover(context.Background(), "", "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	role, err := mgr.GetReplicaRole(context.Background())
	assert.Equal(t, "", role)
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	users, err := mgr.ListUsers(context.Background())
	assert.Nil(t, users)
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	systemAccounts, err := mgr.ListSystemAccounts(context.Background())
	assert.Nil(t, systemAccounts)
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.CreateUser(context.Background(), "", "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.DeleteUser(context.Background(), "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	userInfo, err := mgr.DescribeUser(context.Background(), "")
	assert.Nil(t, userInfo)
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.GrantUserRole(context.Background(), "", "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.RevokeUserRole(context.Background(), "", "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	assert.False(t, mgr.IsRunning())

	err = mgr.PostProvision(context.Background())
	assert.NoError(t, err)

	err = mgr.PreTerminate(context.Background())
	assert.NoError(t, err)

	err = mgr.ReadOnly(context.Background(), "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	err = mgr.ReadWrite(context.Background(), "")
	assert.EqualError(t, err, models.ErrNotImplemented.Error())

	mgr.ShutDownWithWait()
}
