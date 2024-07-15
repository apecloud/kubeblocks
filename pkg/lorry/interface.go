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

package lorry

import (
	"context"
)

type Client interface {
	CreateUser(ctx context.Context, userName, password, roleName, statement string) error

	JoinMember(ctx context.Context) error

	LeaveMember(ctx context.Context) error

	Switchover(ctx context.Context, primary, candidate string, force bool) error
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
	PostProvision(ctx context.Context, componentNames, podNames, podIPs, podHostNames, podHostIPs string) error
	PreTerminate(ctx context.Context) error

	DataDump(ctx context.Context) error
	DataLoad(ctx context.Context) error
}
