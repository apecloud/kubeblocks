/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type doNotTerminateTransformer struct{}

func (d *doNotTerminateTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	cluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)

	if cluster.DeletionTimestamp.IsZero() {
		return nil
	}
	if cluster.Spec.TerminationPolicy != appsv1alpha1.DoNotTerminate {
		return nil
	}
	vertices := findAllNot[*appsv1alpha1.Cluster](dag)
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		v.immutable = true
	}
	return nil
}
