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

package render

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// RenderTemplate renders multiple component templates into Kubernetes ConfigMap objects.
//
// Parameters:
// - resourceCtx: The context for resource operations.
// - cluster: The cluster being reconciled.
// - synthesizedComponent: Details of the synthesized component.
// - comp: The component being reconciled.
// - localObjs: A cache of client objects.
// - tpls: A list of component template specifications.
//
// Returns:
// - A slice of pointers to the rendered ConfigMap objects.
// - An error if the rendering or validation fails.
func RenderTemplate(resourceCtx *ResourceCtx,
	cluster *appsv1.Cluster,
	synthesizedComponent *component.SynthesizedComponent,
	comp *appsv1.Component,
	localObjs []client.Object,
	tpls []appsv1.ComponentTemplateSpec) ([]*corev1.ConfigMap, error) {
	var err error
	var configMap *corev1.ConfigMap

	reconcileCtx := &ReconcileCtx{
		ResourceCtx:          resourceCtx,
		Cluster:              cluster,
		Component:            comp,
		SynthesizedComponent: synthesizedComponent,
		PodSpec:              synthesizedComponent.PodSpec,
		Cache:                localObjs,
	}

	tplBuilder := NewTemplateBuilder(reconcileCtx)
	configMaps := make([]*corev1.ConfigMap, 0, len(tpls))
	for _, template := range tpls {
		cmName := core.GetComponentCfgName(cluster.Name, synthesizedComponent.Name, template.Name)
		if configMap, err = tplBuilder.RenderComponentTemplate(template, cmName, nil); err != nil {
			return nil, err
		}
		if err = intctrlutil.SetOwnerReference(comp, configMap); err != nil {
			return nil, err
		}
		configMaps = append(configMaps, configMap)
	}
	return configMaps, nil
}
