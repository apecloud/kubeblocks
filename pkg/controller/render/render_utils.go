/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
func RenderTemplate(resourceCtx *ResourceCtx,
	cluster *appsv1.Cluster,
	synthesizedComponent *component.SynthesizedComponent,
	comp *appsv1.Component,
	localObjs []client.Object,
	tpls []appsv1.ComponentTemplateSpec) ([]*corev1.ConfigMap, error) {
	var err error
	var configMap *corev1.ConfigMap
	var configMaps []*corev1.ConfigMap

	reconcileCtx := &ReconcileCtx{
		ResourceCtx:          resourceCtx,
		Cluster:              cluster,
		Component:            comp,
		SynthesizedComponent: synthesizedComponent,
		PodSpec:              synthesizedComponent.PodSpec,
		Cache:                localObjs,
	}

	tplBuilder := NewTemplateBuilder(reconcileCtx)
	for _, template := range tpls {
		cmName := core.GetComponentCfgName(cluster.Name, synthesizedComponent.Name, template.Name)
		configMap, err = tplBuilder.RenderComponentTemplate(template, cmName, nil)
		if err != nil {
			return nil, err
		}
		if err = intctrlutil.SetOwnerReference(comp, configMap); err != nil {
			return nil, err
		}
		configMaps = append(configMaps, configMap)
	}
	return configMaps, nil
}
