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

package mysql

import (
	"context"
)

const (
	PRIMARY   = "primary"
	SECONDARY = "secondary"
	MASTER    = "master"
	SLAVE     = "slave"
	LEADER    = "Leader"
	FOLLOWER  = "Follower"
	LEARNER   = "Learner"
	CANDIDATE = "Candidate"
)

func (mgr *Manager) GetRole(ctx context.Context) (string, error) {
	slaveRunning, err := mgr.isSlaveRunning(ctx)
	if err != nil {
		return "", err
	}
	if slaveRunning {
		return SECONDARY, nil
	}

	hasSlave, err := mgr.hasSlaveHosts(ctx)
	if err != nil {
		return "", err
	}
	if hasSlave {
		return PRIMARY, nil
	}

	isReadonly, err := mgr.IsReadonly(ctx, nil, nil)
	if err != nil {
		return "", err
	}
	if isReadonly {
		// TODO: in case of diskfull lock, dababase will be set readonly,
		// how to deal with this situation
		return SECONDARY, nil
	}

	return PRIMARY, nil
}
