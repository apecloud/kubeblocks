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

package cluster

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type clusterTerminationPolicyTransformer struct{}

var _ graph.Transformer = &clusterTerminationPolicyTransformer{}

func (t *clusterTerminationPolicyTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	if !cluster.IsDeleting() {
		return nil
	}

	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}
	if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return err
	}

	hasUpdate := false
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for i, comp := range compList.Items {
		if cluster.Spec.TerminationPolicy != comp.Spec.TerminationPolicy {
			hasUpdate = true
			obj := compList.Items[i].DeepCopy()
			obj.Spec.TerminationPolicy = cluster.Spec.TerminationPolicy
			graphCli.Update(dag, &compList.Items[i], obj)
		}
	}

	if hasUpdate {
		return graph.ErrPrematureStop
	}
	return nil
}
