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

package apps

import (
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ValidateEnableLogsTransformer validates config and sends warning event log if necessary
type ValidateEnableLogsTransformer struct{}

func (e *ValidateEnableLogsTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if cluster.IsDeleting() {
		return nil
	}

	// validate config and send warning event log if necessary
	err := cluster.Spec.ValidateEnabledLogs(transCtx.ClusterDef)
	setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	return nil
}

var _ graph.Transformer = &ValidateEnableLogsTransformer{}
