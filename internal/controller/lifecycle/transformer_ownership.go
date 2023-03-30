/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ownershipTransformer add finalizer to all none cluster objects
type ownershipTransformer struct {
	finalizer string
}

func (f *ownershipTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	vertices := findAllNot[*appsv1alpha1.Cluster](dag)

	controllerutil.AddFinalizer(rootVertex.obj, dbClusterFinalizerName)
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		if err := intctrlutil.SetOwnership(rootVertex.obj, v.obj, scheme, dbClusterFinalizerName); err != nil {
			return err
		}
	}
	return nil
}
