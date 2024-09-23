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
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func createConfigObjects(cli client.Client, ctx context.Context, objs []client.Object, excludeObjs []client.Object) error {
	for _, obj := range objs {
		if slices.Contains(excludeObjs, obj) {
			continue
		}
		if err := cli.Create(ctx, obj, inDataContext()); err != nil {

			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			// for update script cm
			if core.IsSchedulableConfigResource(obj) {
				continue
			}
			if err := cli.Update(ctx, obj, inDataContext()); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildConfigManagerWithComponent build the configmgr sidecar container and update it
// into PodSpec if configuration reload option is on
func buildConfigManagerWithComponent(podSpec *corev1.PodSpec, configSpecs []appsv1.ComponentConfigSpec,
	ctx context.Context, cli client.Client, cluster *appsv1.Cluster, synthesizedComp *component.SynthesizedComponent) error {
	var err error
	var buildParams *cfgcm.CfgManagerBuildParams

	volumeDirs, usingConfigSpecs := getUsingVolumesByConfigSpecs(podSpec, configSpecs)
	if len(volumeDirs) == 0 {
		return nil
	}
	configSpecMetas, err := cfgcm.GetSupportReloadConfigSpecs(usingConfigSpecs, cli, ctx)
	if err != nil {
		return err
	}
	// Configmap uses subPath case: https://github.com/kubernetes/kubernetes/issues/50345
	// The files are being updated on the host VM, but can't be updated in the container.
	configSpecMetas = cfgcm.FilterSupportReloadActionConfigSpecs(configSpecMetas, volumeDirs)
	if len(configSpecMetas) == 0 {
		return nil
	}
	if buildParams, err = buildConfigManagerParams(cli, ctx, cluster, synthesizedComp, configSpecMetas, volumeDirs, podSpec); err != nil {
		return err
	}
	if buildParams == nil {
		return nil
	}

	// This sidecar container will be able to view and signal processes from other containers
	checkAndUpdateSharProcessNamespace(podSpec, buildParams, configSpecMetas)
	container, err := factory.BuildCfgManagerContainer(buildParams)
	if err != nil {
		return err
	}
	updateEnvPath(container, buildParams)
	updateCfgManagerVolumes(podSpec, buildParams)

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)
	if len(buildParams.ToolsContainers) > 0 {
		podSpec.InitContainers = append(podSpec.InitContainers, buildParams.ToolsContainers...)
	}
	filter := func(c *corev1.Container) bool {
		names := []string{container.Name}
		for _, cc := range buildParams.ToolsContainers {
			names = append(names, cc.Name)
		}
		return slices.Contains(names, c.Name)
	}
	component.InjectEnvVars4Containers(synthesizedComp, synthesizedComp.EnvVars, synthesizedComp.EnvFromSources, filter)
	return nil
}

func checkAndUpdateSharProcessNamespace(podSpec *corev1.PodSpec, buildParams *cfgcm.CfgManagerBuildParams, configSpecMetas []cfgcm.ConfigSpecMeta) {
	shared := cfgcm.NeedSharedProcessNamespace(configSpecMetas)
	if shared {
		podSpec.ShareProcessNamespace = cfgutil.ToPointer(true)
	}
	buildParams.ShareProcessNamespace = shared
}

func updateEnvPath(container *corev1.Container, params *cfgcm.CfgManagerBuildParams) {
	if len(params.ScriptVolume) == 0 {
		return
	}
	scriptPath := make([]string, 0, len(params.ScriptVolume))
	for _, volume := range params.ScriptVolume {
		if vm := cfgcm.FindVolumeMount(params.Volumes, volume.Name); vm != nil {
			scriptPath = append(scriptPath, vm.MountPath)
		}
	}
	if len(scriptPath) != 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  cfgcm.KBConfigManagerPathEnv,
			Value: strings.Join(scriptPath, ":"),
		})
	}
}

func updateCfgManagerVolumes(podSpec *corev1.PodSpec, configManager *cfgcm.CfgManagerBuildParams) {
	scriptVolumes := configManager.ScriptVolume
	if len(scriptVolumes) == 0 && len(configManager.CMConfigVolumes) == 0 {
		return
	}

	podVolumes := podSpec.Volumes
	for _, vm := range []*[]corev1.Volume{
		&configManager.ScriptVolume,
		&configManager.CMConfigVolumes,
	} {
		for i := range *vm {
			podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, (*vm)[i].Name, func(string) corev1.Volume {
				return (*vm)[i]
			}, nil)
		}
	}
	podSpec.Volumes = podVolumes

	for volumeName, volume := range configManager.ConfigLazyRenderedVolumes {
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, volumeName)
		for _, container := range usingContainers {
			container.VolumeMounts = append(container.VolumeMounts, volume)
		}
	}
}

func getUsingVolumesByConfigSpecs(podSpec *corev1.PodSpec, configSpecs []appsv1.ComponentConfigSpec) ([]corev1.VolumeMount, []appsv1.ComponentConfigSpec) {
	// Ignore useless configTemplate
	usingConfigSpecs := make([]appsv1.ComponentConfigSpec, 0, len(configSpecs))
	config2Containers := make(map[string][]*corev1.Container)
	for _, configSpec := range configSpecs {
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, configSpec.VolumeName)
		if len(usingContainers) == 0 {
			continue
		}
		usingConfigSpecs = append(usingConfigSpecs, configSpec)
		config2Containers[configSpec.Name] = usingContainers
	}

	// No container using any config template
	if len(usingConfigSpecs) == 0 {
		log.Log.Info(fmt.Sprintf("configSpec config is not used by any container, and pass. configSpec configs: %v", configSpecs))
		return nil, nil
	}

	// Find out which configurations are used by the container
	volumeDirs := make([]corev1.VolumeMount, 0, len(configSpecs)+1)
	for _, configSpec := range usingConfigSpecs {
		// Ignore config template, e.g scripts configmap
		if !core.NeedReloadVolume(configSpec) {
			continue
		}
		sets := cfgutil.NewSet()
		for _, container := range config2Containers[configSpec.Name] {
			volume := intctrlutil.GetVolumeMountByVolume(container, configSpec.VolumeName)
			if volume != nil && !sets.InArray(volume.Name) {
				volumeDirs = append(volumeDirs, *volume)
				sets.Add(volume.Name)
			}
		}
	}
	return volumeDirs, usingConfigSpecs
}

func buildConfigManagerParams(cli client.Client, ctx context.Context, cluster *appsv1.Cluster, comp *component.SynthesizedComponent, configSpecBuildParams []cfgcm.ConfigSpecMeta, volumeDirs []corev1.VolumeMount, podSpec *corev1.PodSpec) (*cfgcm.CfgManagerBuildParams, error) {
	cfgManagerParams := &cfgcm.CfgManagerBuildParams{
		ManagerName:               constant.ConfigSidecarName,
		ComponentName:             comp.Name,
		Image:                     viper.GetString(constant.KBToolsImage),
		Volumes:                   volumeDirs,
		Cluster:                   cluster,
		ConfigSpecsBuildParams:    configSpecBuildParams,
		ConfigLazyRenderedVolumes: make(map[string]corev1.VolumeMount),
		ContainerPort:             viper.GetInt32(constant.ConfigManagerGPRCPortEnv),
	}

	if podSpec.HostNetwork {
		containerPort, err := allocConfigManagerHostPort(comp)
		if err != nil {
			return nil, err
		}
		cfgManagerParams.ContainerPort = containerPort
	}

	if err := cfgcm.BuildConfigManagerContainerParams(cli, ctx, cfgManagerParams, volumeDirs); err != nil {
		return nil, err
	}
	if err := buildReloadToolsContainer(cfgManagerParams, podSpec); err != nil {
		return nil, err
	}
	return cfgManagerParams, nil
}

func GetConfigManagerGRPCPort(containers []corev1.Container) (int32, error) {
	for _, container := range containers {
		if container.Name != constant.ConfigSidecarName {
			continue
		}
		if port, ok := findPortByPortName(container); ok {
			return port, nil
		}
	}
	return constant.InvalidContainerPort, core.MakeError("failed to find config manager grpc port, please add named config-manager port")
}

func allocConfigManagerHostPort(comp *component.SynthesizedComponent) (int32, error) {
	pm := intctrlutil.GetPortManager()
	portKey := intctrlutil.BuildHostPortName(comp.ClusterName, comp.Name, constant.ConfigSidecarName, constant.ConfigManagerPortName)
	port, err := pm.AllocatePort(portKey)
	if err != nil {
		return constant.InvalidContainerPort, err
	}
	return port, nil
}

func findPortByPortName(container corev1.Container) (int32, bool) {
	for _, port := range container.Ports {
		if port.Name == constant.ConfigManagerPortName {
			return port.ContainerPort, true
		}
	}
	return constant.InvalidContainerPort, false
}

// UpdateConfigPayload updates the configuration payload
func UpdateConfigPayload(config *configurationv1alpha1.ComponentParameterSpec, component *component.SynthesizedComponent) (bool, error) {
	updated := false
	for i := range config.ConfigItemDetails {
		configSpec := &config.ConfigItemDetails[i]
		// check v-scale operation
		if enableVScaleTrigger(configSpec.ConfigSpec) {
			resourcePayload := intctrlutil.ResourcesPayloadForComponent(component.Resources)
			ret, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.ComponentResourcePayload, resourcePayload)
			if err != nil {
				return false, err
			}
			updated = updated || ret
		}
		// check h-scale operation
		if enableHScaleTrigger(configSpec.ConfigSpec) {
			ret, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.ReplicasPayload, component.Replicas)
			if err != nil {
				return false, err
			}
			updated = updated || ret
		}
	}
	return updated, nil
}

func validRerenderResources(configSpec *appsv1.ComponentConfigSpec) bool {
	return configSpec != nil && len(configSpec.ReRenderResourceTypes) != 0
}

func enableHScaleTrigger(configSpec *appsv1.ComponentConfigSpec) bool {
	return validRerenderResources(configSpec) && slices.Contains(configSpec.ReRenderResourceTypes, appsv1.ComponentHScaleType)
}

func enableVScaleTrigger(configSpec *appsv1.ComponentConfigSpec) bool {
	return validRerenderResources(configSpec) && slices.Contains(configSpec.ReRenderResourceTypes, appsv1.ComponentVScaleType)
}

func configSetFromComponent(templates []appsv1.ComponentConfigSpec) []string {
	configSet := make([]string, 0)
	for _, template := range templates {
		configSet = append(configSet, template.Name)
	}
	return configSet
}
