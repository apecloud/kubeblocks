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

package redis

import (
	"context"
	"strings"
	"time"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

func (mgr *Manager) GetReplicaRole(ctx context.Context, _ *dcs.Cluster) (string, error) {
	// To ensure that the role information obtained through subscription is always delivered.
	if mgr.role != "" && mgr.roleSubscribeUpdateTime+mgr.roleProbePeriod*2 < time.Now().Unix() {
		return mgr.role, nil
	}

	// We use the role obtained from Sentinel as the sole source of truth.
	masterAddr, err := mgr.sentinelClient.GetMasterAddrByName(ctx, mgr.ClusterCompName).Result()
	if err != nil {
		// when we can't get role from sentinel, we query redis instead
		var role string
		result, err := mgr.client.Info(ctx, "Replication").Result()
		if err != nil {
			mgr.Logger.Info("Role query failed", "error", err.Error())
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
		} else {
			return models.SECONDARY, nil
		}
	}

	masterName := strings.Split(masterAddr[0], ".")[0]
	// if current member is not master from sentinel, just return secondary to avoid double master
	if masterName != mgr.CurrentMemberName {
		return models.SECONDARY, nil
	}
	return models.PRIMARY, nil
}

func (mgr *Manager) SubscribeRoleChange(ctx context.Context) {
	pubSub := mgr.sentinelClient.Subscribe(ctx, "+switch-master")

	// go-redis periodically sends ping messages to test connection health
	// and re-subscribes if ping can not receive for 30 seconds.
	// so we don't need to retry
	ch := pubSub.Channel()
	for msg := range ch {
		// +switch-master <master name> <old ip> <old port> <new ip> <new port>
		masterAddr := strings.Split(msg.Payload, " ")
		masterName := strings.Split(masterAddr[3], ".")[0]

		if masterName == mgr.CurrentMemberName {
			mgr.role = models.PRIMARY
		} else {
			mgr.role = models.SECONDARY
		}
		mgr.roleSubscribeUpdateTime = time.Now().Unix()
	}
}
