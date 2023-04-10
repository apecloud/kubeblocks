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

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// configTransformer makes all config related ConfigMaps immutable
type configTransformer struct{}

func (c *configTransformer) Transform(dag *graph.DAG) error {
	cmVertices := findAll[*corev1.ConfigMap](dag)
	isConfig := func(cm *corev1.ConfigMap) bool {
		// TODO: we should find a way to know if cm is a true config
		// TODO: the main problem is we can't separate script from config,
		// TODO: as componentDef.ConfigSpec defines them in same way
		return false
	}
	for _, vertex := range cmVertices {
		v, _ := vertex.(*lifecycleVertex)
		cm, _ := v.obj.(*corev1.ConfigMap)
		if isConfig(cm) {
			v.immutable = true
		}
	}
	return nil
}
