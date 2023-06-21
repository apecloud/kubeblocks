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

package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/StudioSol/set"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// RenderConfigNScriptFiles generates volumes for PodTemplate, volumeMount for container, rendered configTemplate and scriptTemplate,
// and generates configManager sidecar for the reconfigure operation.
// TODO rename this function, this function name is not very reasonable, but there is no suitable name.
func RenderConfigNScriptFiles(clusterVersion *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	obj client.Object,
	podSpec *corev1.PodSpec,
	localObjs []client.Object,
	ctx context.Context,
	cli client.Client) error {
	// Need to Merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	if len(component.ConfigTemplates) == 0 && len(component.ScriptTemplates) == 0 {
		return nil
	}

	clusterName := cluster.Name
	namespaceName := cluster.Namespace
	templateBuilder := newTemplateBuilder(clusterName, namespaceName, cluster, clusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := templateBuilder.injectBuiltInObjectsAndFunctions(podSpec, component.ConfigTemplates, component, localObjs); err != nil {
		return err
	}

	renderWrapper := newTemplateRenderWrapper(templateBuilder, cluster, ctx, cli)
	if err := renderWrapper.renderConfigTemplate(cluster, component, localObjs); err != nil {
		return err
	}
	if err := renderWrapper.renderScriptTemplate(cluster, component, localObjs); err != nil {
		return err
	}

	if len(renderWrapper.templateAnnotations) > 0 {
		updateResourceAnnotationsWithTemplate(obj, renderWrapper.templateAnnotations)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CreateOrUpdatePodVolumes(podSpec, renderWrapper.volumes); err != nil {
		return cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := buildConfigManagerWithComponent(podSpec, component.ConfigTemplates, ctx, cli, cluster, component); err != nil {
		return cfgcore.WrapError(err, "failed to generate sidecar for configmap's reloader")
	}
	// TODO config resource objects are updated by the operator
	return createConfigObjects(cli, ctx, renderWrapper.renderedObjs)
}

func createConfigObjects(cli client.Client, ctx context.Context, objs []client.Object) error {
	for _, obj := range objs {
		if err := cli.Create(ctx, obj); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			// for update script cm
			if cfgcore.IsSchedulableConfigResource(obj) {
				continue
			}
			if err := cli.Update(ctx, obj); err != nil {
				return err
			}
		}
	}
	return nil
}

func updateResourceAnnotationsWithTemplate(obj client.Object, allTemplateAnnotations map[string]string) {
	// full configmap upgrade
	existLabels := make(map[string]string)
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for key, val := range annotations {
		if strings.HasPrefix(key, constant.ConfigurationTplLabelPrefixKey) {
			existLabels[key] = val
		}
	}

	// delete not exist configmap label
	deletedLabels := util.MapKeyDifference(existLabels, allTemplateAnnotations)
	for l := range deletedLabels.Iter() {
		delete(annotations, l)
	}

	for key, val := range allTemplateAnnotations {
		annotations[key] = val
	}
	obj.SetAnnotations(annotations)
}

// buildConfigManagerWithComponent build the configmgr sidecar container and update it
// into PodSpec if configuration reload option is on
func buildConfigManagerWithComponent(podSpec *corev1.PodSpec, configSpecs []appsv1alpha1.ComponentConfigSpec,
	ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) error {
	var (
		err error

		buildParams *cfgcm.CfgManagerBuildParams
		// volumeDirs       []corev1.VolumeMount
		// usingConfigSpecs []appsv1alpha1.ComponentConfigSpec
	)

	volumeDirs, usingConfigSpecs := getUsingVolumesByConfigSpecs(podSpec, configSpecs)
	if len(volumeDirs) == 0 {
		return nil
	}
	configSpecMetas, err := cfgcm.GetSupportReloadConfigSpecs(usingConfigSpecs, cli, ctx)
	if err != nil {
		return err
	}
	if len(configSpecMetas) == 0 {
		return nil
	}
	if buildParams, err = buildConfigManagerParams(cli, ctx, cluster, component, configSpecMetas, volumeDirs, podSpec); err != nil {
		return err
	}
	if buildParams == nil {
		return nil
	}

	// This sidecar container will be able to view and signal processes from other containers
	checkAndUpdateSharProcessNamespace(podSpec, buildParams, configSpecMetas)
	container, err := builder.BuildCfgManagerContainer(buildParams, component)
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
	return nil
}

func checkAndUpdateSharProcessNamespace(podSpec *corev1.PodSpec, buildParams *cfgcm.CfgManagerBuildParams, configSpecMetas []cfgcm.ConfigSpecMeta) {
	shared := cfgcm.NeedSharedProcessNamespace(configSpecMetas)
	if shared {
		podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
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
			Name:  "TOOLS_PATH",
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

func getUsingVolumesByConfigSpecs(podSpec *corev1.PodSpec, configSpecs []appsv1alpha1.ComponentConfigSpec) ([]corev1.VolumeMount, []appsv1alpha1.ComponentConfigSpec) {
	// Ignore useless configTemplate
	usingConfigSpecs := make([]appsv1alpha1.ComponentConfigSpec, 0, len(configSpecs))
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
		if !cfgcore.NeedReloadVolume(configSpec) {
			continue
		}
		sets := set.NewLinkedHashSetString()
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

func buildConfigManagerParams(cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster, comp *component.SynthesizedComponent, configSpecBuildParams []cfgcm.ConfigSpecMeta, volumeDirs []corev1.VolumeMount, podSpec *corev1.PodSpec) (*cfgcm.CfgManagerBuildParams, error) {
	cfgManagerParams := &cfgcm.CfgManagerBuildParams{
		ManagerName:               constant.ConfigSidecarName,
		CharacterType:             comp.CharacterType,
		ComponentName:             comp.Name,
		SecreteName:               component.GenerateConnCredential(cluster.Name),
		EnvConfigName:             component.GenerateComponentEnvName(cluster.Name, comp.Name),
		Image:                     viper.GetString(constant.KBToolsImage),
		Volumes:                   volumeDirs,
		Cluster:                   cluster,
		ConfigSpecsBuildParams:    configSpecBuildParams,
		ConfigLazyRenderedVolumes: make(map[string]corev1.VolumeMount),
	}

	if err := cfgcm.BuildConfigManagerContainerParams(cli, ctx, cfgManagerParams, volumeDirs); err != nil {
		return nil, err
	}
	if err := buildConfigToolsContainer(cfgManagerParams, podSpec, comp); err != nil {
		return nil, err
	}
	return cfgManagerParams, nil
}
