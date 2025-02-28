/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	// when we can't get role from sentinel, we query redis instead
	getRoleFromRedisClient := func() (string, error) {
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

	if mgr.sentinelClient == nil {
		return getRoleFromRedisClient()
	}

	// We use the role obtained from Sentinel as the sole source of truth.
	masterAddr, err := mgr.sentinelClient.GetMasterAddrByName(ctx, mgr.masterName).Result()
	if err != nil {
		mgr.Logger.Info("failed to get master address from Sentinel, try to get from Redis", "error", err.Error())
		return getRoleFromRedisClient()
	}

	masterIP := masterAddr[0]
	masterPort := masterAddr[1]
	return mgr.checkPrimary(masterIP, masterPort), nil
}

func (mgr *Manager) SubscribeRoleChange(ctx context.Context) {
	pubSub := mgr.sentinelClient.Subscribe(ctx, "+switch-master")

	// go-redis periodically sends ping messages to test connection health
	// and re-subscribes if ping can not receive for 30 seconds.
	// so we don't need to retry
	ch := pubSub.Channel()
	for msg := range ch {
		// +switch-master <master name> <old ip> <old port> <new ip> <new port>
		msgInfo := strings.Split(msg.Payload, " ")
		if len(msgInfo) != 5 {
			mgr.Logger.Info("failed to get switch master info from subscribe", "msg", msg.Payload)
		}

		masterIP := msgInfo[3]
		masterPort := msgInfo[4]
		mgr.role = mgr.checkPrimary(masterIP, masterPort)
		mgr.roleSubscribeUpdateTime = time.Now().Unix()
	}
}

// If current member is not master from sentinel, just return secondary to avoid double master
// When currentRedisHost is a domain name, it does not include dnsDomain by default,
// and prefix matching can override the matching of domain names or IPs.
func (mgr *Manager) checkPrimary(masterIP, masterPort string) string {
	if !strings.HasPrefix(masterIP, mgr.currentRedisHost) || masterPort != mgr.currentRedisPort {
		return models.SECONDARY
	}
	return models.PRIMARY
}
