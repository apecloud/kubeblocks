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

package lifecycle

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// OwnershipTransformer adds finalizer to all none cluster objects
type OwnershipTransformer struct{}

var _ graph.Transformer = &OwnershipTransformer{}

func (f *OwnershipTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	rootVertex, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}
	vertices := ictrltypes.FindAllNot[*appsv1alpha1.Cluster](dag)

	controllerutil.AddFinalizer(rootVertex.Obj, constant.DBClusterFinalizerName)
	for _, vertex := range vertices {
		v, _ := vertex.(*ictrltypes.LifecycleVertex)
		if err := intctrlutil.SetOwnership(rootVertex.Obj, v.Obj, scheme, constant.DBClusterFinalizerName); err != nil {
			if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
				continue
			}
			return err
		}
	}
	return nil
}
