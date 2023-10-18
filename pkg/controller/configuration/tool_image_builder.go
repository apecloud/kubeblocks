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
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	toolsVolumeName                  = "kb-tools"
	initSecRenderedToolContainerName = "init-secondary-rendered-tool"

	tplRenderToolPath = "/bin/config_render"
)

func buildConfigToolsContainer(cfgManagerParams *cfgcm.CfgManagerBuildParams, podSpec *corev1.PodSpec, comp *component.SynthesizedComponent) error {
	if len(cfgManagerParams.ConfigSpecsBuildParams) == 0 {
		return nil
	}

	// construct config manager tools volume
	toolContainers := make([]appsv1alpha1.ToolConfig, 0)
	toolsMap := make(map[string]cfgcm.ConfigSpecMeta)
	for _, buildParam := range cfgManagerParams.ConfigSpecsBuildParams {
		if buildParam.ToolsImageSpec == nil {
			continue
		}
		for _, toolConfig := range buildParam.ToolsImageSpec.ToolConfigs {
			if _, ok := toolsMap[toolConfig.Name]; !ok {
				replaceToolsImageHolder(&toolConfig, podSpec, buildParam.ConfigSpec.VolumeName)
				toolContainers = append(toolContainers, toolConfig)
				toolsMap[toolConfig.Name] = buildParam
			}
		}
		buildToolsVolumeMount(cfgManagerParams, podSpec, buildParam.ConfigSpec.VolumeName, buildParam.ToolsImageSpec.MountPoint)
	}

	// Ensure that the order in which iniContainers are generated does not change
	toolContainers = checkAndInstallToolsImageVolume(toolContainers, cfgManagerParams.ConfigSpecsBuildParams)
	if len(toolContainers) == 0 {
		return nil
	}

	containers, err := factory.BuildCfgManagerToolsContainer(cfgManagerParams, comp, toolContainers, toolsMap)
	if err == nil {
		cfgManagerParams.ToolsContainers = containers
	}
	return err
}

func checkAndInstallToolsImageVolume(toolContainers []appsv1alpha1.ToolConfig, buildParams []cfgcm.ConfigSpecMeta) []appsv1alpha1.ToolConfig {
	for _, buildParam := range buildParams {
		if buildParam.ToolsImageSpec != nil && buildParam.ConfigSpec.LegacyRenderedConfigSpec != nil {
			// auto install config_render tool
			toolContainers = checkAndCreateRenderedInitContainer(toolContainers, buildParam.ToolsImageSpec.MountPoint)
		}
	}
	return toolContainers
}

func checkAndCreateRenderedInitContainer(toolContainers []appsv1alpha1.ToolConfig, mountPoint string) []appsv1alpha1.ToolConfig {
	kbToolsImage := viper.GetString(constant.KBToolsImage)
	for _, container := range toolContainers {
		if container.Name == initSecRenderedToolContainerName {
			return nil
		}
	}
	toolContainers = append(toolContainers, appsv1alpha1.ToolConfig{
		Name:    initSecRenderedToolContainerName,
		Image:   kbToolsImage,
		Command: []string{"cp", tplRenderToolPath, mountPoint},
	})
	return toolContainers
}

func replaceToolsImageHolder(toolConfig *appsv1alpha1.ToolConfig, podSpec *corev1.PodSpec, volumeName string) {
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
