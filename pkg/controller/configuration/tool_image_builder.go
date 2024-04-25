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
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	toolsVolumeName                      = "kb-tools"
	initSecRenderedToolContainerName     = "init-secondary-rendered-tool"
	installConfigMangerToolContainerName = "install-config-manager-tool"
)

func buildReloadToolsContainer(cfgManagerParams *cfgcm.CfgManagerBuildParams, podSpec *corev1.PodSpec) error {
	if len(cfgManagerParams.ConfigSpecsBuildParams) == 0 {
		return nil
	}

	// construct config manager tools volume
	toolsImageMap := make(map[string]cfgcm.ConfigSpecMeta)

	var toolsPath string
	var sidecarImage string
	var toolContainers []appsv1beta1.ToolConfig
	for _, buildParam := range cfgManagerParams.ConfigSpecsBuildParams {
		if buildParam.ToolsImageSpec == nil {
			continue
		}
		for _, toolImage := range buildParam.ToolsImageSpec.ToolConfigs {
			if _, ok := toolsImageMap[toolImage.Name]; ok {
				continue
			}
			toolsImageMap[toolImage.Name] = buildParam
			replaceToolsImageHolder(&toolImage, podSpec, buildParam.ConfigSpec.VolumeName)
			if toolImage.AsSidecarContainerImage() && sidecarImage == "" {
				sidecarImage = toolImage.Image
			} else {
				toolContainers = append(toolContainers, toolImage)
			}
		}
		buildToolsVolumeMount(cfgManagerParams, podSpec, buildParam.ConfigSpec.VolumeName, buildParam.ToolsImageSpec.MountPoint)
	}

	// Ensure that the order in which iniContainers are generated does not change
	toolContainers, toolsPath = checkAndInstallToolsImageVolume(toolContainers, cfgManagerParams.ConfigSpecsBuildParams, sidecarImage == "")
	if len(toolContainers) == 0 {
		return nil
	}
	if sidecarImage != "" {
		cfgManagerParams.Image = sidecarImage
	}
	if toolsPath != "" {
		cfgManagerParams.ConfigManagerReloadPath = toolsPath
	}
	containers, err := factory.BuildCfgManagerToolsContainer(cfgManagerParams, toolContainers, toolsImageMap)
	if err == nil {
		cfgManagerParams.ToolsContainers = containers
	}
	return err
}

func checkAndInstallToolsImageVolume(toolContainers []appsv1beta1.ToolConfig, buildParams []cfgcm.ConfigSpecMeta, useBuiltinSidecarImage bool) ([]appsv1beta1.ToolConfig, string) {
	var configManagerBinaryPath string
	for _, buildParam := range buildParams {
		if buildParam.ToolsImageSpec != nil && buildParam.ConfigSpec.LegacyRenderedConfigSpec != nil {
			// auto install config_render tool
			toolContainers = checkAndCreateRenderedInitContainer(toolContainers, buildParam.ToolsImageSpec.MountPoint)
		}
		if !useBuiltinSidecarImage {
			toolContainers = checkAndCreateConfigManagerToolsContainer(toolContainers, buildParam.ToolsImageSpec.MountPoint)
			configManagerBinaryPath = filepath.Join(buildParam.ToolsImageSpec.MountPoint, filepath.Base(constant.ConfigManagerToolPath))
		}
	}
	return toolContainers, configManagerBinaryPath
}

func checkAndCreateRenderedInitContainer(toolContainers []appsv1beta1.ToolConfig, mountPoint string) []appsv1beta1.ToolConfig {
	kbToolsImage := viper.GetString(constant.KBToolsImage)
	for _, container := range toolContainers {
		if container.Name == initSecRenderedToolContainerName {
			return nil
		}
	}
	toolContainers = append(toolContainers, appsv1beta1.ToolConfig{
		Name:    initSecRenderedToolContainerName,
		Image:   kbToolsImage,
		Command: []string{"cp", constant.ConfigManagerToolPath, mountPoint},
	})
	return toolContainers
}

func checkAndCreateConfigManagerToolsContainer(toolContainers []appsv1beta1.ToolConfig, mountPoint string) []appsv1beta1.ToolConfig {
	kbToolsImage := viper.GetString(constant.KBToolsImage)
	for _, container := range toolContainers {
		if container.Name == installConfigMangerToolContainerName {
			return nil
		}
	}
	toolContainers = append(toolContainers, appsv1beta1.ToolConfig{
		Name:    installConfigMangerToolContainerName,
		Image:   kbToolsImage,
		Command: []string{"cp", constant.TPLRenderToolPath, mountPoint},
	})
	return toolContainers
}

func replaceToolsImageHolder(toolConfig *appsv1beta1.ToolConfig, podSpec *corev1.PodSpec, volumeName string) {
	switch {
	case toolConfig.Image == constant.KBToolsImagePlaceHolder:
		toolConfig.Image = viper.GetString(constant.KBToolsImage)
	case toolConfig.Image == "":
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, volumeName)
		if len(usingContainers) != 0 {
			toolConfig.Image = usingContainers[0].Image
		}
	}
}

func buildToolsVolumeMount(cfgManagerParams *cfgcm.CfgManagerBuildParams, podSpec *corev1.PodSpec, volumeName string, mountPoint string) {
	if cfgcm.FindVolumeMount(cfgManagerParams.Volumes, toolsVolumeName) != nil {
		return
	}
	cfgManagerParams.ScriptVolume = append(cfgManagerParams.ScriptVolume, corev1.Volume{
		Name: toolsVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	n := len(cfgManagerParams.Volumes)
	cfgManagerParams.Volumes = append(cfgManagerParams.Volumes, corev1.VolumeMount{
		Name:      toolsVolumeName,
		MountPath: mountPoint,
	})

	usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, volumeName)
	for _, container := range usingContainers {
		container.VolumeMounts = append(container.VolumeMounts, cfgManagerParams.Volumes[n])
	}
}
