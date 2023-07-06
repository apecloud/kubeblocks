/*
Copyright ApeCloud, Inc.

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

package plan

import (
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
	toolSets := cfgutil.NewSet()
	for _, buildParam := range cfgManagerParams.ConfigSpecsBuildParams {
		if buildParam.ToolsImageSpec == nil {
			continue
		}
		for _, toolConfig := range buildParam.ToolsImageSpec.ToolConfigs {
			if !toolSets.InArray(toolConfig.Name) {
				replaceToolsImageHolder(&toolConfig, podSpec, buildParam.ConfigSpec.VolumeName)
				toolContainers = append(toolContainers, toolConfig)
				toolSets.Add(toolConfig.Name)
			}
		}
		buildToolsVolumeMount(cfgManagerParams, podSpec, buildParam.ConfigSpec.VolumeName, buildParam.ToolsImageSpec.MountPoint)
	}

	// Ensure that the order in which iniContainers are generated does not change
	toolContainers = checkAndInstallToolsImageVolume(toolContainers, cfgManagerParams.ConfigSpecsBuildParams)
	if len(toolContainers) == 0 {
		return nil
	}

	containers, err := builder.BuildCfgManagerToolsContainer(cfgManagerParams, comp, toolContainers)
	if err == nil {
		cfgManagerParams.ToolsContainers = containers
	}
	return err
}

func checkAndInstallToolsImageVolume(toolContainers []appsv1alpha1.ToolConfig, buildParams []cfgcm.ConfigSpecMeta) []appsv1alpha1.ToolConfig {
	for _, buildParam := range buildParams {
		if buildParam.ToolsImageSpec != nil && buildParam.ConfigSpec.LazyRenderedConfigSpec != nil {
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
