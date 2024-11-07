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
	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func InjectTemplateEnvFrom(component *component.SynthesizedComponent,
	podSpec *corev1.PodSpec,
	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
	tplObjs []*corev1.ConfigMap) ([]*corev1.ConfigMap, error) {
	withEnvSource := func(name string) corev1.EnvFromSource {
		return corev1.EnvFromSource{ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			}}}
	}

	injectConfigmap := func(envMap map[string]string, templateName string, injectEnvs []string) *corev1.ConfigMap {
		cmName := core.GetComponentCfgName(component.ClusterName, component.Name, templateName)
		envSourceObject := builder.NewConfigMapBuilder(component.Namespace, core.GenerateEnvFromName(cmName)).
			AddLabels(constant.CMConfigurationSpecProviderLabelKey, templateName).
			AddLabelsInMap(constant.GetCompLabels(component.ClusterName, component.Name)).
			SetData(envMap).
			GetObject()
		if podSpec != nil {
			injectEnvFrom(podSpec.Containers, injectEnvs, envSourceObject.GetName(), withEnvSource)
			injectEnvFrom(podSpec.InitContainers, injectEnvs, envSourceObject.GetName(), withEnvSource)
		}
		return envSourceObject
	}

	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil, nil
	}

	var cm *corev1.ConfigMap
	var envObjs []*corev1.ConfigMap
	for _, config := range configRender.Spec.Configs {
		if len(config.InjectEnvTo) == 0 || config.FileFormatConfig == nil {
			continue
		}
		if cm = resolveConfigMap(tplObjs, config.Name); cm == nil {
			continue
		}
		envMap, err := resolveParametersFromFileContent(config.FileFormatConfig, cm.Data[config.Name])
		if err != nil {
			return nil, err
		}
		envObjs = append(envObjs, injectConfigmap(envMap, cm.Labels[constant.CMConfigurationSpecProviderLabelKey], config.InjectEnvTo))
	}
	return envObjs, nil
}

func resolveConfigMap(localObjs []*corev1.ConfigMap, key string) *corev1.ConfigMap {
	for _, obj := range localObjs {
		if _, ok := obj.Data[key]; ok {
			return obj
		}
	}
	return nil
}

func CheckEnvFrom(container *corev1.Container, cmName string) bool {
	for i := range container.EnvFrom {
		source := &container.EnvFrom[i]
		if source.ConfigMapRef != nil && source.ConfigMapRef.Name == cmName {
			return true
		}
		if source.SecretRef != nil && source.SecretRef.Name == cmName {
			return true
		}
	}
	return false
}

func injectEnvFrom(containers []corev1.Container, injectEnvTo []string, cmName string, fn func(string) corev1.EnvFromSource) {
	sets := cfgutil.NewSet(injectEnvTo...)
	for i := range containers {
		container := &containers[i]
		if sets.InArray(container.Name) && !CheckEnvFrom(container, cmName) {
			container.EnvFrom = append(container.EnvFrom, fn(cmName))
		}
	}
}

func resolveParametersFromFileContent(format *parametersv1alpha1.FileFormatConfig, configContext string) (map[string]string, error) {
	keyValue, err := validate.LoadConfigObjectFromContent(format.Format, configContext)
	if err != nil {
		return nil, err
	}
	envMap := make(map[string]string, len(keyValue))
	for key, v := range keyValue {
		envMap[key] = cast.ToString(v)
	}
	return envMap, nil
}
