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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	graph2 "github.com/apecloud/kubeblocks/pkg/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/pkg/controller/types"
)

type initTransformer struct {
	cluster       *appsv1alpha1.Cluster
	originCluster *appsv1alpha1.Cluster
}

var _ graph2.Transformer = &initTransformer{}

func (t *initTransformer) Transform(ctx graph2.TransformContext, dag *graph2.DAG) error {
	// put the cluster object first, it will be root vertex of DAG
	rootVertex := &ictrltypes.LifecycleVertex{Obj: t.cluster, ObjCopy: t.originCluster, Action: ictrltypes.ActionStatusPtr()}
	dag.AddVertex(rootVertex)

	// TODO: why set cluster status phase here?
	if t.cluster.IsUpdating() {
		t.handleClusterPhase()
	}
	return nil
}

func (t *initTransformer) handleClusterPhase() {
	clusterPhase := t.cluster.Status.Phase
	if clusterPhase == "" {
		t.cluster.Status.Phase = appsv1alpha1.CreatingClusterPhase
	} else if clusterPhase != appsv1alpha1.CreatingClusterPhase {
		t.cluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
	}
}
