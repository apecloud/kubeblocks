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

package configuration

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ReconcileCtx struct {
	*ResourceCtx

	Cluster              *appsv1.Cluster
	Component            *appsv1.Component
	SynthesizedComponent *component.SynthesizedComponent
	PodSpec              *corev1.PodSpec
	ComponentParameter   *parametersv1alpha1.ComponentParameter

	Cache []client.Object
}

func RenderTemplate(resourceCtx *ResourceCtx,
	cluster *appsv1.Cluster,
	synthesizedComponent *component.SynthesizedComponent,
	comp *appsv1.Component,
	localObjs []client.Object,
	tpls []appsv1.ComponentTemplateSpec) ([]*corev1.ConfigMap, error) {
	var err error
	var configCM *corev1.ConfigMap
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
	for _, tpl := range tpls {
		configCMName := core.GetComponentCfgName(cluster.Name, comp.Name, tpl.Name)
		if configCM, err = generateConfigMapFromTemplate(cluster, synthesizedComponent, tplBuilder, configCMName, tpl, resourceCtx, reconcileCtx.Client, nil); err != nil {
		}
		if err != nil {
			return nil, err
		}
		if err = intctrlutil.SetControllerReference(comp, configCM); err != nil {
			return nil, err
		}
		configMaps = append(configMaps, configCM)
	}
	return configMaps, nil
}

func RerenderParametersTemplate(reconcileCtx *ReconcileCtx, tpl appsv1.ComponentTemplateSpec, render *parametersv1alpha1.ParameterDrivenConfigRender, defs []*parametersv1alpha1.ParametersDefinition) (*corev1.ConfigMap, error) {
}
