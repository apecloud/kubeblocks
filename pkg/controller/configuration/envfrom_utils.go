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
	"context"

	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
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

func fromConfigmapFiles(keys []string, cm *corev1.ConfigMap, formatter *parametersv1alpha1.FileFormatConfig) (map[string]string, error) {
	mergeMap := func(dst, src map[string]string) {
		for key, val := range src {
			dst[key] = val
		}
	}

	gEnvMap := make(map[string]string)
	for _, file := range keys {
		envMap, err := resolveParametersFromFileContent(formatter, cm.Data[file])
		if err != nil {
			return nil, err
		}
		mergeMap(gEnvMap, envMap)
	}
	return gEnvMap, nil
}

func resolveConfigMap(localObjs []*corev1.ConfigMap, key string) *corev1.ConfigMap {
	for _, obj := range localObjs {
		if _, ok := obj.Data[key]; ok {
			return obj
		}
	}
	return nil
}

func createOrUpdateResourceFromConfigTemplate(cluster *appsv1.Cluster, component *component.SynthesizedComponent, template appsv1.ComponentConfigSpec, originKey client.ObjectKey, envMap map[string]string, ctx context.Context, cli client.Client, createOnly bool) (client.Object, error) {
	cmKey := client.ObjectKey{
		Name:      core.GenerateEnvFromName(originKey.Name),
		Namespace: originKey.Namespace,
	}

	updateObjectMeta := func(obj client.Object) {
		obj.SetLabels(constant.GetConfigurationLabels(component.ClusterName, component.Name, template.Name))
		_ = intctrlutil.SetOwnerReference(cluster, obj)
	}

	if toSecret(template) {
		return updateOrCreateEnvObject(ctx, cli, &corev1.Secret{}, cmKey, func(c *corev1.Secret) {
			c.StringData = envMap
			updateObjectMeta(c)
		}, createOnly)
	}
	return updateOrCreateEnvObject(ctx, cli, &corev1.ConfigMap{}, cmKey, func(c *corev1.ConfigMap) {
		c.Data = envMap
		updateObjectMeta(c)
	}, createOnly)
}

func updateOrCreateEnvObject[T generics.Object, PT generics.PObject[T]](ctx context.Context, cli client.Client, obj PT, objKey client.ObjectKey, updater func(PT), createOnly bool) (client.Object, error) {
	err := cli.Get(ctx, objKey, obj, inDataContext())
	switch {
	case err != nil:
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		obj.SetName(objKey.Name)
		obj.SetNamespace(objKey.Namespace)
		updater(obj)
		return obj, cli.Create(ctx, obj, inDataContext())
	case createOnly:
		return obj, nil
	default:
		updater(obj)
		return obj, cli.Update(ctx, obj, inDataContext())
	}
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

func fromConfigSpec(configSpec appsv1.ComponentConfigSpec, cm *corev1.ConfigMap) []string {
	keys := configSpec.Keys
	if len(keys) == 0 {
		keys = cfgutil.ToSet(cm.Data).AsSlice()
	}
	return keys
}

func SyncEnvSourceObject(configSpec appsv1.ComponentConfigSpec, cmObj *corev1.ConfigMap, cc *appsv1beta1.ConfigConstraintSpec, cli client.Client, ctx context.Context, cluster *appsv1.Cluster, component *component.SynthesizedComponent) error {
	if !InjectEnvEnabled(configSpec) || cc == nil || cc.FileFormatConfig == nil {
		return nil
	}
	envMap, err := fromConfigmapFiles(fromConfigSpec(configSpec, cmObj), cmObj, cc.FileFormatConfig)
	if err != nil {
		return err
	}
	if len(envMap) != 0 {
		_, err = createOrUpdateResourceFromConfigTemplate(cluster, component, configSpec, client.ObjectKeyFromObject(cmObj), envMap, ctx, cli, false)
	}
	return err
}

func InjectEnvEnabled(spec appsv1.ComponentConfigSpec) bool {
	return len(spec.AsEnvFrom) > 0 || len(spec.InjectEnvTo) > 0
}

func toSecret(spec appsv1.ComponentConfigSpec) bool {
	return spec.AsSecret != nil && *spec.AsSecret
}
