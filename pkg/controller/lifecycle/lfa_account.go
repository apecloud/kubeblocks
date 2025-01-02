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

package lifecycle

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	accountName      = "KB_ACCOUNT_NAME"
	accountPassword  = "KB_ACCOUNT_PASSWORD"
	accountStatement = "KB_ACCOUNT_STATEMENT"
)

type accountProvision struct {
	statement string
	user      string
	password  string
}

var _ lifecycleAction = &accountProvision{}

func (a *accountProvision) name() string {
	return "accountProvision"
}

func (a *accountProvision) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following variables:
	//
	// - KB_ACCOUNT_NAME: The name of the system account to be created.
	// - KB_ACCOUNT_PASSWORD: The password for the system account.
	// - KB_ACCOUNT_STATEMENT: The statement used to create the system account.
	return map[string]string{
		accountName:      a.user,
		accountPassword:  a.password,
		accountStatement: a.statement,
	}, nil
}
