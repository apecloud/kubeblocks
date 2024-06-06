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

	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
)

type Handler interface {
	// healthy check
	IsRunning() bool
	IsDBStartupReady() bool
	HealthyCheck(context.Context) error

	// Functions related to replica member relationship.
	GetReplicaRole(context.Context) (string, error)
	JoinMember(context.Context, string) error
	LeaveMember(context.Context, string) error
	Switchover(context.Context, string, string) error

	// Readonly lock for disk full
	ReadOnly(context.Context, string) error
	ReadWrite(context.Context, string) error

	// user management
	ListUsers(context.Context) ([]models.UserInfo, error)
	ListSystemAccounts(context.Context) ([]models.UserInfo, error)
	CreateUser(context.Context, string, string) error
	DeleteUser(context.Context, string) error
	DescribeUser(context.Context, string) (*models.UserInfo, error)
	GrantUserRole(context.Context, string, string) error
	RevokeUserRole(context.Context, string, string) error

	DataLoad(context.Context) error
	DataDump(context.Context) error

	PostProvision(context.Context) error
	PreTerminate(context.Context) error

	GetCurrentMemberName() string
	GetLogger() logr.Logger
	ShutDownWithWait()
}
