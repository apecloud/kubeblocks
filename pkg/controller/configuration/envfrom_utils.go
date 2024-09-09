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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
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

	withEnvSource := func(asSecret bool) func(name string) corev1.EnvFromSource {
		return func(name string) corev1.EnvFromSource {
			if asSecret {
				return corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: name,
						}}}
			}
			return corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					}}}
		}
	}

	injectConfigmap := func(envMap map[string]string, configSpec appsv1alpha1.ComponentConfigSpec, cmName string) error {
		envSourceObject, err := createOrUpdateResourceFromConfigTemplate(cluster, component, configSpec, client.ObjectKeyFromObject(cm), envMap, ctx, cli, true)
		if err != nil {
			return core.WrapError(err, "failed to generate env configmap[%s]", cmName)
		}
		if configSpec.ToSecret() && configSpec.VolumeName != "" {
			podSpec.Volumes = updateSecretVolumes(podSpec.Volumes, configSpec, envSourceObject, component)
		} else {
			injectEnvFrom(podSpec.Containers, configSpec.ContainersInjectedTo(), envSourceObject.GetName(), withEnvSource(configSpec.ToSecret()))
			injectEnvFrom(podSpec.InitContainers, configSpec.ContainersInjectedTo(), envSourceObject.GetName(), withEnvSource(configSpec.ToSecret()))
		}
		return nil
	}

	for _, template := range component.ConfigTemplates {
		if !template.InjectEnvEnabled() || template.ConfigConstraintRef == "" {
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
		envMap, err := fromConfigmapFiles(fromConfigSpec(template, cm), cm, cc.FileFormatConfig)
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

func updateSecretVolumes(volumes []corev1.Volume, configSpec appsv1alpha1.ComponentConfigSpec, secret client.Object, component *component.SynthesizedComponent) []corev1.Volume {
	sets := configSetFromComponent(component.ConfigTemplates)
	createFn := func(_ string) corev1.Volume {
		return corev1.Volume{
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secret.GetName(),
					DefaultMode: intctrlutil.BuildVolumeMode(sets, configSpec.ComponentTemplateSpec),
				},
			},
			Name: configSpec.VolumeName,
		}
	}
	volumes, _ = intctrlutil.CreateOrUpdateVolume(volumes, configSpec.VolumeName, createFn, nil)
	return volumes
}

func getConfigConstraint(template appsv1alpha1.ComponentConfigSpec, cli client.Client, ctx context.Context) (*appsv1beta1.ConfigConstraintSpec, error) {
	ccKey := client.ObjectKey{
		Namespace: "",
		Name:      template.ConfigConstraintRef,
	}
	cc := &appsv1beta1.ConfigConstraint{}
	if err := cli.Get(ctx, ccKey, cc); err != nil {
		return nil, core.WrapError(err, "failed to get ConfigConstraint, key[%v]", ccKey)
	}
	if cc.Spec.FileFormatConfig == nil {
		return nil, core.MakeError("ConfigConstraint[%v] is not a formatter", cc.Name)
	}
	return &cc.Spec, nil
}

func fromConfigmapFiles(keys []string, cm *corev1.ConfigMap, formatter *appsv1beta1.FileFormatConfig) (map[string]string, error) {
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
	if err := cli.Get(ctx, cmKey, cmObj, inDataContext()); err != nil {
		return nil, err
	}
	return cmObj, nil
}

func createOrUpdateResourceFromConfigTemplate(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent, template appsv1alpha1.ComponentConfigSpec, originKey client.ObjectKey, envMap map[string]string, ctx context.Context, cli client.Client, createOnly bool) (client.Object, error) {
	cmKey := client.ObjectKey{
		Name:      core.GenerateEnvFromName(originKey.Name),
		Namespace: originKey.Namespace,
	}

	updateObjectMeta := func(obj client.Object) {
		obj.SetLabels(constant.GetKBConfigMapWellKnownLabels(template.Name, component.CompDefName, component.ClusterName, component.Name))
		_ = intctrlutil.SetOwnerReference(cluster, obj)
	}

	if template.ToSecret() {
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
	case err == nil && createOnly:
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

func fromFileContent(format *appsv1beta1.FileFormatConfig, configContext string) (map[string]string, error) {
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

func SyncEnvConfigmap(configSpec appsv1alpha1.ComponentConfigSpec, cmObj *corev1.ConfigMap, cc *appsv1beta1.ConfigConstraintSpec, cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) error {
	if !configSpec.InjectEnvEnabled() || cc == nil || cc.FileFormatConfig == nil {
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
