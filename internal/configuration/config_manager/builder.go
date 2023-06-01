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

package configmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	configTemplateName   = "reload.yaml"
	scriptVolumePrefix   = "cm-script-"
	configVolumePrefix   = "cm-config-"
	kbScriptVolumePath   = "/opt/kb-tools/reload"
	kbConfigVolumePath   = "/opt/kb-tools/config"
	scriptConfigField    = "scripts"
	formatterConfigField = "formatterConfig"
)

const KBConfigSpecYamlFile = "kb-cm-config.yaml"

func BuildConfigManagerContainerParams(cli client.Client, ctx context.Context, cmBuildParams *CfgManagerBuildParams, volumeDirs []corev1.VolumeMount) error {
	allVolumeMounts := getWatchedVolume(volumeDirs, cmBuildParams.ConfigSpecsBuildParams)
	for i := range cmBuildParams.ConfigSpecsBuildParams {
		buildParam := &cmBuildParams.ConfigSpecsBuildParams[i]
		volumeMount := FindVolumeMount(cmBuildParams.Volumes, buildParam.ConfigSpec.VolumeName)
		if volumeMount == nil {
			logger.Info(fmt.Sprintf("volume mount not be use : %s", buildParam.ConfigSpec.VolumeName))
			continue
		}
		buildParam.MountPoint = volumeMount.MountPath
		if err := buildConfigSpecHandleMeta(cli, ctx, buildParam, cmBuildParams); err != nil {
			return err
		}
		if err := buildConfigSpecSecondaryRenderConfig(cli, ctx, buildParam, cmBuildParams); err != nil {
			return err
		}
	}
	downwardAPIVolumes := buildDownwardAPIVolumes(cmBuildParams)
	allVolumeMounts = append(allVolumeMounts, downwardAPIVolumes...)
	cmBuildParams.Volumes = append(cmBuildParams.Volumes, downwardAPIVolumes...)
	return buildConfigManagerArgs(cmBuildParams, allVolumeMounts)
}

func getWatchedVolume(volumeDirs []corev1.VolumeMount, buildParams []ConfigSpecMeta) []corev1.VolumeMount {
	enableWatchVolume := func(volume corev1.VolumeMount) bool {
		for _, param := range buildParams {
			if param.ReloadType != appsv1alpha1.TPLScriptType || param.ConfigSpec.VolumeName != volume.Name {
				continue
			}
			triggerOptions := param.ReloadOptions.TPLScriptTrigger
			if triggerOptions == nil || triggerOptions.Sync == nil {
				return true
			}
			return !*triggerOptions.Sync
		}
		return true
	}

	allVolumeMounts := make([]corev1.VolumeMount, 0, len(volumeDirs))
	for _, volume := range volumeDirs {
		if enableWatchVolume(volume) {
			allVolumeMounts = append(allVolumeMounts, volume)
		}
	}
	return allVolumeMounts
}

// buildConfigSpecSecondaryRenderConfig prepare secondary render config and volume
func buildConfigSpecSecondaryRenderConfig(cli client.Client, ctx context.Context, param *ConfigSpecMeta, manager *CfgManagerBuildParams) error {
	processYamlConfig := func(cm *corev1.ConfigMap) error {
		renderMeta := ConfigSecondaryRenderMeta{
			ComponentConfigSpec: &param.ConfigSpec,
			Templates:           cfgutil.ToSet(cm.Data).AsSlice(),
			FormatterConfig:     param.FormatterConfig,
		}
		b, err := cfgutil.ToYamlConfig(renderMeta)
		if err != nil {
			return err
		}
		cm.Data[KBConfigSpecYamlFile] = string(b)
		return nil
	}

	secondaryTemplate := param.ConfigSpec.SecondaryRenderedConfigSpec
	if secondaryTemplate == nil {
		return nil
	}
	referenceCMKey := client.ObjectKey{
		Namespace: secondaryTemplate.Namespace,
		Name:      secondaryTemplate.TemplateRef,
	}
	configCMKey := client.ObjectKey{
		Namespace: manager.Cluster.GetNamespace(),
		Name:      fmt.Sprintf("%s-%s", secondaryTemplate.TemplateRef, manager.Cluster.GetName()),
	}
	if err := checkOrCreateConfigMap(referenceCMKey, configCMKey, cli, ctx, manager.Cluster, processYamlConfig); err != nil {
		return err
	}
	buildConfigSecondaryRenderVolume(configCMKey.Name, manager, GetConfigMountPoint(param.ConfigSpec), GetConfigVolumeName(param.ConfigSpec), param.ConfigSpec)
	return nil
}

func buildDownwardAPIVolumes(params *CfgManagerBuildParams) []corev1.VolumeMount {
	for _, buildParam := range params.ConfigSpecsBuildParams {
		for _, info := range buildParam.DownwardAPIOptions {
			if FindVolumeMount(params.DownwardAPIVolumes, info.Name) == nil {
				buildDownwardAPIVolume(params, info)
			}
		}
	}
	return params.DownwardAPIVolumes
}

func buildConfigManagerArgs(params *CfgManagerBuildParams, volumeDirs []corev1.VolumeMount) error {
	args := buildConfigManagerCommonArgs(volumeDirs)
	args = append(args, "--operator-update-enable")
	// args = append(args, "--log-level", viper.GetString(constant.ConfigManagerLogLevel))
	args = append(args, "--tcp", viper.GetString(constant.ConfigManagerGPRCPortEnv))
	// args = append(args, "--notify-type", string(appsv1alpha1.MultiType))

	b, err := json.Marshal(frmConfigSpecMeta(params.ConfigSpecsBuildParams))
	if err != nil {
		return err
	}
	args = append(args, "--config", string(b))
	params.Args = args
	return nil
}

func frmConfigSpecMeta(metas []ConfigSpecMeta) []ConfigSpecInfo {
	configSpecs := make([]ConfigSpecInfo, 0, len(metas))
	for _, meta := range metas {
		configSpecs = append(configSpecs, meta.ConfigSpecInfo)
	}
	return configSpecs
}

func FindVolumeMount(volumeDirs []corev1.VolumeMount, volumeName string) *corev1.VolumeMount {
	for i := range volumeDirs {
		if volumeDirs[i].Name == volumeName {
			return &volumeDirs[i]
		}
	}
	return nil
}

func buildConfigSpecHandleMeta(cli client.Client, ctx context.Context, buildParam *ConfigSpecMeta, cmBuildParam *CfgManagerBuildParams) error {
	for _, script := range buildParam.ScriptConfig {
		if err := buildCfgManagerScripts(script, cmBuildParam, cli, ctx, buildParam.ConfigSpec); err != nil {
			return err
		}
	}
	if buildParam.ReloadType == appsv1alpha1.TPLScriptType {
		return buildTPLScriptCM(buildParam, cmBuildParam, cli, ctx)
	}
	return nil
}

func buildTPLScriptCM(configSpecBuildMeta *ConfigSpecMeta, manager *CfgManagerBuildParams, cli client.Client, ctx context.Context) error {
	var (
		options      = configSpecBuildMeta.TPLScriptTrigger
		formatConfig = configSpecBuildMeta.FormatterConfig
		mountPoint   = GetScriptsMountPoint(configSpecBuildMeta.ConfigSpec)
	)

	reloadYamlFn := func(cm *corev1.ConfigMap) error {
		newData, err := checkAndUpdateReloadYaml(cm.Data, configTemplateName, formatConfig)
		if err != nil {
			return err
		}
		cm.Data = newData
		return nil
	}

	referenceCMKey := client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}
	scriptCMKey := client.ObjectKey{
		Namespace: manager.Cluster.GetNamespace(),
		Name:      fmt.Sprintf("%s-%s", options.ScriptConfigMapRef, manager.Cluster.GetName()),
	}
	if err := checkOrCreateConfigMap(referenceCMKey, scriptCMKey, cli, ctx, manager.Cluster, reloadYamlFn); err != nil {
		return err
	}
	buildReloadScriptVolume(scriptCMKey.Name, manager, mountPoint, GetScriptsVolumeName(configSpecBuildMeta.ConfigSpec))
	configSpecBuildMeta.TPLConfig = filepath.Join(mountPoint, configTemplateName)
	return nil
}

func buildDownwardAPIVolume(manager *CfgManagerBuildParams, fieldInfo appsv1alpha1.DownwardAPIOption) {
	manager.DownwardAPIVolumes = append(manager.DownwardAPIVolumes, corev1.VolumeMount{
		Name:      fieldInfo.Name,
		MountPath: fieldInfo.MountPoint,
	})
	manager.CMConfigVolumes = append(manager.CMConfigVolumes, corev1.Volume{
		Name: fieldInfo.Name,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: fieldInfo.Items,
			}},
	})
}

func buildReloadScriptVolume(scriptCMName string, manager *CfgManagerBuildParams, mountPoint, volumeName string) {
	var execMode int32 = 0755
	manager.Volumes = append(manager.Volumes, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPoint,
	})
	manager.ScriptVolume = append(manager.ScriptVolume, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: scriptCMName},
				DefaultMode:          &execMode,
			},
		},
	})
}

func buildConfigSecondaryRenderVolume(cmName string, manager *CfgManagerBuildParams, mountPoint, volumeName string, configSpec appsv1alpha1.ComponentConfigSpec) {
	n := len(manager.Volumes)
	manager.Volumes = append(manager.Volumes, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPoint,
	})
	manager.CMConfigVolumes = append(manager.CMConfigVolumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
			},
		},
	})
	manager.ConfigSecondaryVolumes[configSpec.VolumeName] = manager.Volumes[n]
}

func checkOrCreateConfigMap(referenceCM client.ObjectKey, scriptCMKey client.ObjectKey, cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster, fn func(cm *corev1.ConfigMap) error) error {
	var (
		err error

		refCM     = corev1.ConfigMap{}
		sidecarCM = corev1.ConfigMap{}
	)

	if err = cli.Get(ctx, referenceCM, &refCM); err != nil {
		return err
	}
	if err = cli.Get(ctx, scriptCMKey, &sidecarCM); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		sidecarCM.Data = refCM.Data
		if fn != nil && fn(&sidecarCM) != nil {
			return err
		}
		sidecarCM.SetLabels(refCM.GetLabels())
		sidecarCM.SetName(scriptCMKey.Name)
		sidecarCM.SetNamespace(scriptCMKey.Namespace)
		sidecarCM.SetLabels(refCM.Labels)
		if err := controllerutil.SetOwnerReference(cluster, &sidecarCM, scheme); err != nil {
			return err
		}
		if err := cli.Create(ctx, &sidecarCM); err != nil {
			return err
		}
	}
	return nil
}

func checkAndUpdateReloadYaml(data map[string]string, reloadConfig string, formatterConfig appsv1alpha1.FormatterConfig) (map[string]string, error) {
	configObject := make(map[string]interface{})
	if content, ok := data[reloadConfig]; ok {
		if err := yaml.Unmarshal([]byte(content), &configObject); err != nil {
			return nil, err
		}
	}
	if res, _, _ := unstructured.NestedFieldNoCopy(configObject, scriptConfigField); res == nil {
		return nil, cfgcore.MakeError("reload.yaml required field: %s", scriptConfigField)
	}

	formatObject, err := apiruntime.DefaultUnstructuredConverter.ToUnstructured(&formatterConfig)
	if err != nil {
		return nil, err
	}
	if err := unstructured.SetNestedField(configObject, formatObject, formatterConfigField); err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(configObject)
	if err != nil {
		return nil, err
	}
	data[reloadConfig] = string(b)
	return data, nil
}

func buildCfgManagerScripts(options appsv1alpha1.ScriptConfig, manager *CfgManagerBuildParams, cli client.Client, ctx context.Context, configSpec appsv1alpha1.ComponentConfigSpec) error {
	mountPoint := filepath.Join(kbScriptVolumePath, configSpec.Name)
	referenceCMKey := client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}
	scriptsCMKey := client.ObjectKey{
		Namespace: manager.Cluster.GetNamespace(),
		Name:      fmt.Sprintf("%s-%s", options.ScriptConfigMapRef, manager.Cluster.GetName()),
	}
	if err := checkOrCreateConfigMap(referenceCMKey, scriptsCMKey, cli, ctx, manager.Cluster, nil); err != nil {
		return err
	}
	buildReloadScriptVolume(scriptsCMKey.Name, manager, mountPoint, GetScriptsVolumeName(configSpec))
	return nil
}

func GetConfigMountPoint(configSpec appsv1alpha1.ComponentConfigSpec) string {
	return filepath.Join(kbConfigVolumePath, configSpec.Name)
}

func GetScriptsMountPoint(configSpec appsv1alpha1.ComponentConfigSpec) string {
	return filepath.Join(kbScriptVolumePath, configSpec.Name)
}

func GetScriptsVolumeName(configSpec appsv1alpha1.ComponentConfigSpec) string {
	return fmt.Sprintf("%s%s", scriptVolumePrefix, configSpec.Name)
}

func GetConfigVolumeName(configSpec appsv1alpha1.ComponentConfigSpec) string {
	return fmt.Sprintf("%s%s", configVolumePrefix, configSpec.Name)
}

func buildConfigManagerCommonArgs(volumeDirs []corev1.VolumeMount) []string {
	args := make([]string, 0)
	// set grpc port
	// args = append(args, "--tcp", viper.GetString(cfgcore.ConfigManagerGPRCPortEnv))
	args = append(args, "--log-level", viper.GetString(constant.ConfigManagerLogLevel))
	for _, volume := range volumeDirs {
		args = append(args, "--volume-dir", volume.MountPath)
	}
	return args
}
