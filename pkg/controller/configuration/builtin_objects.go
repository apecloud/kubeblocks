/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type ResourceDefinition struct {
	MemorySize int64 `json:"memorySize,omitempty"`
	CoreNum    int64 `json:"coreNum,omitempty"`
}

type builtInObjects struct {
	cluster   *appsv1alpha1.Cluster
	podSpec   *corev1.PodSpec
	component *component.SynthesizedComponent
}

// General built-in objects
const (
	builtinClusterObject       = "cluster"
	builtinComponentObject     = "component"
	builtinPodObject           = "podSpec"
	builtinClusterDomainObject = "clusterDomain"
)

func buildInComponentObjects(podSpec *corev1.PodSpec, component *component.SynthesizedComponent, cluster *appsv1alpha1.Cluster) *builtInObjects {
	return &builtInObjects{
		podSpec:   podSpec,
		component: component,
		cluster:   cluster,
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
		builtinClusterObject:       builtin.cluster,
		builtinComponentObject:     builtin.component,
		builtinPodObject:           builtin.podSpec,
		builtinClusterDomainObject: viper.GetString(constant.KubernetesClusterDomainEnv),
	}
}
