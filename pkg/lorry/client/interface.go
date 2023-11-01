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

package client

import "context"

type Client interface {
	// GetRole return the replication role(like primary/secondary) of the target replica
	GetRole(ctx context.Context) (string, error)

	// user management funcs
	CreateUser(ctx context.Context, userName, password string) error
	DeleteUser(ctx context.Context, userName string) error
	DescribeUser(ctx context.Context, userName string) (map[string]any, error)
	GrantUserRole(ctx context.Context, userName, roleName string) error
	RevokeUserRole(ctx context.Context, userName, roleName string) error
	ListUsers(ctx context.Context) ([]map[string]any, error)
	ListSystemAccounts(ctx context.Context) ([]map[string]any, error)

	// JoinMember sends a join member operation request to Lorry, located on the target pod that is about to join.
	JoinMember(ctx context.Context) error

	// LeaveMember sends a Leave member operation request to Lorry, located on the target pod that is about to leave.
	LeaveMember(ctx context.Context) error
}
