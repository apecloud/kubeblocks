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

package redis

import (
	"context"
	"strings"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

func (mgr *Manager) GetReplicaRole(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	section := "Replication"

	var role string
	result, err := mgr.client.Info(ctx, section).Result()
	if err != nil {
		mgr.Logger.Error(err, "Role query error")
		return role, err
	} else {
		// split the result into lines
		lines := strings.Split(result, "\r\n")
		// find the line with role
		for _, line := range lines {
			if strings.HasPrefix(line, "role:") {
				role = strings.Split(line, ":")[1]
				break
			}
		}
	}
	if role == models.MASTER {
		return models.PRIMARY, nil
	}
	if role == models.SLAVE {
		return models.SECONDARY, nil
	}
	return role, nil
}
