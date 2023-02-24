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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// finalizerSetterTransformer add finalizer to all none cluster objects
type finalizerSetterTransformer struct {
	finalizer string
}

func (f *finalizerSetterTransformer) Transform(dag *graph.DAG) error {
	vertices, err := findAllNot[*appsv1alpha1.Cluster](dag)
	if err != nil {
		return err
	}
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		if controllerutil.ContainsFinalizer(v.obj, f.finalizer) {
			continue
		}
		// pvc objects do not need to add finalizer
		if _, ok := v.obj.(*corev1.PersistentVolumeClaim); ok {
			continue
		}
		controllerutil.AddFinalizer(v.obj, f.finalizer)
	}
	return nil
}