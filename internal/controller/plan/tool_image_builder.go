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
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	toolsVolumeName                  = "kb-tools"
	initSecRenderedToolContainerName = "init-secondary-rendered-tool"
)

func buildConfigToolsContainer(cfgManagerParams *cfgcm.CfgManagerBuildParams, podSpec *corev1.PodSpec, comp *component.SynthesizedComponent) {
	if len(cfgManagerParams.ConfigSpecsBuildParams) == 0 {
		return
	}

	// construct config manager tools volume
	toolContainers := make(map[string]appsv1alpha1.ToolConfig)
	for _, buildParam := range cfgManagerParams.ConfigSpecsBuildParams {
		if buildParam.ToolImageSpec == nil {
			continue
		}
		for _, toolConfig := range buildParam.ToolImageSpec.ToolConfigs {
			if _, ok := toolContainers[toolConfig.Name]; !ok {
				replaceToolImageHolder(&toolConfig, podSpec, buildParam.ConfigSpec.VolumeName)
				toolContainers[toolConfig.Name] = toolConfig
			}
		}
		buildToolsVolumeMount(cfgManagerParams, podSpec, buildParam.ConfigSpec.VolumeName, buildParam.ToolImageSpec.MountPoint)
		if buildParam.ConfigSpec.SecondaryRenderedConfigSpec != nil {
			// auto install config_render tool
			checkAndCreateRenderedInitContainer(toolContainers, buildParam.ToolImageSpec.MountPoint)
		}
	}
	if len(toolContainers) != 0 {
		cfgManagerParams.ToolsContainers = builder.BuildCfgManagerToolsContainer(cfgManagerParams, comp, toolContainers)
	}
}

func checkAndCreateRenderedInitContainer(toolContainers map[string]appsv1alpha1.ToolConfig, mountPoint string) {
	kbToolsImage := viper.GetString(constant.KBToolsImage)
	for name := range toolContainers {
		if name == initSecRenderedToolContainerName {
			return
		}
	}
	toolContainers[initSecRenderedToolContainerName] = appsv1alpha1.ToolConfig{
		Name:  initSecRenderedToolContainerName,
		Image: kbToolsImage,
		Command: []string{
			"cp",
			"/bin/config_render",
			mountPoint,
		},
	}
}

func replaceToolImageHolder(toolConfig *appsv1alpha1.ToolConfig, podSpec *corev1.PodSpec, volumeName string) {
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
