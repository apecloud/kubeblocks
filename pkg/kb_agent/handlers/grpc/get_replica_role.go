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

package grpc

import (
	"context"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/plugin"
)

func (h *Handler) GetReplicaRole(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	getRoleRequest := &plugin.GetRoleRequest{
		EngineInfo: plugin.GetEngineInfo(),
	}

	resp, err := h.dbClient.GetRole(ctx, getRoleRequest)
	if err != nil {
		return "", err
	}

	return resp.Role, nil
}
