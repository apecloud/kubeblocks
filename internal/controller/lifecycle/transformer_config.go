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
	corev1 "k8s.io/api/core/v1"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// configTransformer makes all config related ConfigMaps immutable
type configTransformer struct{}

func (c *configTransformer) Transform(dag *graph.DAG) error {
	for _, vertex := range findAll[*corev1.ConfigMap](dag) {
		v, _ := vertex.(*lifecycleVertex)
		cm, _ := v.obj.(*corev1.ConfigMap)
		// Note: Disable updating of the config resources.
		// Labels and Annotations have the necessary meta information for controller.
		if cfgcore.IsSchedulableConfigResource(cm) {
			v.immutable = true
		}
	}
	return nil
}
