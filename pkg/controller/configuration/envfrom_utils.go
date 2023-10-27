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
	"context"

	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func injectTemplateEnvFrom(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent, podSpec *corev1.PodSpec, cli client.Client, ctx context.Context, localObjs []client.Object) error {
	var err error
	var cm *corev1.ConfigMap

	injectConfigmap := func(envMap map[string]string, configSpec appsv1alpha1.ComponentConfigSpec, cmName string) error {
		envConfigMap, err := createEnvFromConfigmap(cluster, component.Name, configSpec, client.ObjectKeyFromObject(cm), envMap, ctx, cli)
		if err != nil {
			return core.WrapError(err, "failed to generate env configmap[%s]", cmName)
		}
		injectEnvFrom(podSpec.Containers, configSpec.AsEnvFrom, envConfigMap.Name)
		injectEnvFrom(podSpec.InitContainers, configSpec.AsEnvFrom, envConfigMap.Name)
		return nil
	}

	for _, template := range component.ConfigTemplates {
		if len(template.AsEnvFrom) == 0 || template.ConfigConstraintRef == "" {
			continue
		}
		cmName := core.GetComponentCfgName(cluster.Name, component.Name, template.Name)
		if cm, err = fetchConfigmap(localObjs, cmName, cluster.Namespace, cli, ctx); err != nil {
			return err
		}
		cc, err := getConfigConstraint(template, cli, ctx)
		if err != nil {
			return err
		}
		envMap, err := fromConfigmapFiles(fromConfigSpec(template, cm), cm, cc.FormatterConfig)
		if err != nil {
			return err
		}
		if len(envMap) == 0 {
			continue
		}
		if err := injectConfigmap(envMap, template, cmName); err != nil {
			return err
		}
	}
	return nil
}

func getConfigConstraint(template appsv1alpha1.ComponentConfigSpec, cli client.Client, ctx context.Context) (*appsv1alpha1.ConfigConstraintSpec, error) {
	ccKey := client.ObjectKey{
		Namespace: "",
		Name:      template.ConfigConstraintRef,
	}
	cc := &appsv1alpha1.ConfigConstraint{}
	if err := cli.Get(ctx, ccKey, cc); err != nil {
		return nil, core.WrapError(err, "failed to get ConfigConstraint, key[%v]", ccKey)
	}
	if cc.Spec.FormatterConfig == nil {
		return nil, core.MakeError("ConfigConstraint[%v] is not a formatter", cc.Name)
	}
	return &cc.Spec, nil
}

func fromConfigmapFiles(keys []string, cm *corev1.ConfigMap, formatter *appsv1alpha1.FormatterConfig) (map[string]string, error) {
	mergeMap := func(dst, src map[string]string) {
		for key, val := range src {
			dst[key] = val
		}
	}

	gEnvMap := make(map[string]string)
	for _, file := range keys {
		envMap, err := fromFileContent(formatter, cm.Data[file])
		if err != nil {
			return nil, err
		}
		mergeMap(gEnvMap, envMap)
	}
	return gEnvMap, nil
}

func fetchConfigmap(localObjs []client.Object, cmName, namespace string, cli client.Client, ctx context.Context) (*corev1.ConfigMap, error) {
	var (
		cmObj = &corev1.ConfigMap{}
		cmKey = client.ObjectKey{Name: cmName, Namespace: namespace}
	)

	localObject := findMatchedLocalObject(localObjs, cmKey, generics.ToGVK(cmObj))
	if localObject != nil {
		return localObject.(*corev1.ConfigMap), nil
	}
	if err := cli.Get(ctx, cmKey, cmObj); err != nil {
		return nil, err
	}
	return cmObj, nil
}

func createEnvFromConfigmap(cluster *appsv1alpha1.Cluster, componentName string, template appsv1alpha1.ComponentConfigSpec, originKey client.ObjectKey, envMap map[string]string, ctx context.Context, cli client.Client) (*corev1.ConfigMap, error) {
	cmKey := client.ObjectKey{
		Name:      core.GenerateEnvFromName(originKey.Name),
		Namespace: originKey.Namespace,
	}
	cm := &corev1.ConfigMap{}
	err := cli.Get(ctx, cmKey, cm)
	if err == nil {
		return cm, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	cm.Name = cmKey.Name
	cm.Namespace = cmKey.Namespace
	cm.Data = envMap
	cm.Labels = map[string]string{
		constant.CMTemplateNameLabelKey: template.Name,
		constant.AppNameLabelKey:        cluster.Spec.ClusterDefRef,
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: componentName,
	}
	if err := intctrlutil.SetOwnerReference(cluster, cm); err != nil {
		return nil, err
	}
	return cm, cli.Create(ctx, cm)
}

func CheckEnvFrom(container *corev1.Container, cmName string) bool {
	for i := range container.EnvFrom {
		source := &container.EnvFrom[i]
		if source.ConfigMapRef != nil && source.ConfigMapRef.Name == cmName {
			return true
		}
	}
	return false
}

func injectEnvFrom(containers []corev1.Container, asEnvFrom []string, cmName string) {
	sets := cfgutil.NewSet(asEnvFrom...)
	for i := range containers {
		container := &containers[i]
		if sets.InArray(container.Name) && !CheckEnvFrom(container, cmName) {
			container.EnvFrom = append(container.EnvFrom,
				corev1.EnvFromSource{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cmName,
						}},
				})
		}
	}
}

func fromFileContent(format *appsv1alpha1.FormatterConfig, configContext string) (map[string]string, error) {
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

func fromConfigSpec(configSpec appsv1alpha1.ComponentConfigSpec, cm *corev1.ConfigMap) []string {
	keys := configSpec.Keys
	if len(keys) == 0 {
		keys = cfgutil.ToSet(cm.Data).AsSlice()
	}
	return keys
}

func SyncEnvConfigmap(configSpec appsv1alpha1.ComponentConfigSpec, cmObj *corev1.ConfigMap, cc *appsv1alpha1.ConfigConstraintSpec, cli client.Client, ctx context.Context) error {
	if len(configSpec.AsEnvFrom) == 0 || cc == nil || cc.FormatterConfig == nil {
		return nil
	}
	envMap, err := fromConfigmapFiles(fromConfigSpec(configSpec, cmObj), cmObj, cc.FormatterConfig)
	if err != nil {
		return err
	}
	if len(envMap) == 0 {
		return nil
	}

	return updateEnvFromConfigmap(client.ObjectKeyFromObject(cmObj), envMap, cli, ctx)
}

func updateEnvFromConfigmap(origObj client.ObjectKey, envMap map[string]string, cli client.Client, ctx context.Context) error {
	cmKey := client.ObjectKey{
		Name:      core.GenerateEnvFromName(origObj.Name),
		Namespace: origObj.Namespace,
	}
	cm := &corev1.ConfigMap{}
	if err := cli.Get(ctx, cmKey, cm); err != nil {
		return err
	}
	patch := client.MergeFrom(cm.DeepCopy())
	cm.Data = envMap
	if err := cli.Patch(ctx, cm, patch); err != nil {
		return err
	}
	return nil
}
