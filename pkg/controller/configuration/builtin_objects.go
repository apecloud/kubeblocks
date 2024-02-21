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

package configuration

import (
	"encoding/json"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type ResourceDefinition struct {
	MemorySize int64 `json:"memorySize,omitempty"`
	CoreNum    int64 `json:"coreNum,omitempty"`
}

type componentTemplateValues struct {
	TypeName    string
	ServiceName string
	Replicas    int32

	// Container *corev1.Container
	Resource    *ResourceDefinition
	ConfigSpecs []appsv1alpha1.ComponentConfigSpec
}

type builtInObjects struct {
	cluster          *appsv1alpha1.Cluster
	podSpec          *corev1.PodSpec
	component        *component.SynthesizedComponent
	dynamicCompInfos *[]DynamicComponentInfo
	componentValues  *componentTemplateValues
}

// General built-in objects
const (
	builtinClusterObject           = "cluster"
	builtinComponentObject         = "component"
	builtinDynamicCompInfosObject  = "dynamicCompInfos"
	builtinPodObject               = "podSpec"
	builtinComponentResourceObject = "componentResource"
	builtinClusterDomainObject     = "clusterDomain"
)

func buildInComponentObjects(cache []client.Object, podSpec *corev1.PodSpec, component *component.SynthesizedComponent, configSpecs []appsv1alpha1.ComponentConfigSpec, cluster *appsv1alpha1.Cluster) *builtInObjects {
	var resource *ResourceDefinition

	container := intctrlutil.GetContainerByConfigSpec(podSpec, configSpecs)
	if container != nil && len(container.Resources.Limits) > 0 {
		resource = &ResourceDefinition{
			MemorySize: intctrlutil.GetMemorySize(*container),
			CoreNum:    intctrlutil.GetCoreNum(*container),
		}
	}

	return &builtInObjects{
		componentValues: &componentTemplateValues{
			TypeName:    component.ClusterCompDefName,
			Replicas:    component.Replicas,
			Resource:    resource,
			ConfigSpecs: configSpecs,
		},
		podSpec:          podSpec,
		component:        component,
		cluster:          cluster,
		dynamicCompInfos: buildDynamicCompInfos(cache, podSpec, component),
	}
}

func builtinObjectsAsValues(builtin *builtInObjects) (*gotemplate.TplValues, error) {
	vars := builtinCustomObjects(builtin)
	if builtin.component.TemplateVars != nil {
		maps.Copy(vars, builtin.component.TemplateVars)
	}

	b, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}
	var tplValue gotemplate.TplValues
	if err = json.Unmarshal(b, &tplValue); err != nil {
		return nil, err
	}
	return &tplValue, nil
}

func builtinCustomObjects(builtin *builtInObjects) map[string]any {
	return map[string]any{
		builtinClusterObject:           builtin.cluster,
		builtinComponentObject:         builtin.component,
		builtinDynamicCompInfosObject:  builtin.dynamicCompInfos,
		builtinPodObject:               builtin.podSpec,
		builtinComponentResourceObject: builtin.componentValues.Resource,
		builtinClusterDomainObject:     viper.GetString(constant.KubernetesClusterDomainEnv),
	}
}
