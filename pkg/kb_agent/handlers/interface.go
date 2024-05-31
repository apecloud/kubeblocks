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

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
)

type Handler interface {
	IsRunning() bool

	IsDBStartupReady() bool

	// Member healthy check
	MemberHealthyCheck(context.Context, *dcs.Cluster, *dcs.Member) error
	GetLag(context.Context, *dcs.Cluster) (int64, error)

	// Functions related to replica member relationship.
	IsLeader(context.Context, *dcs.Cluster) (bool, error)
	GetReplicaRole(context.Context, *dcs.Cluster) (string, error)

	JoinMember(context.Context, *dcs.Cluster, string) error
	LeaveMember(context.Context, *dcs.Cluster, string) error

	GetCurrentMemberName() string

	// Readonly lock for disk full
	Lock(context.Context, string) error
	Unlock(context.Context, string) error

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

	PostProvision(context.Context, *dcs.Cluster) error
	PreTerminate(context.Context, *dcs.Cluster) error

	GetLogger() logr.Logger

	ShutDownWithWait()
}
