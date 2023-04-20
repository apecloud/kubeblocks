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

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
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
	cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	if len(component.ConfigTemplates) == 0 && len(component.ScriptTemplates) == 0 {
		return nil, nil
	}

	clusterName := cluster.Name
	namespaceName := cluster.Namespace
	templateBuilder := newTemplateBuilder(clusterName, namespaceName, cluster, clusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := templateBuilder.injectBuiltInObjectsAndFunctions(podSpec, component.ConfigTemplates, component, localObjs); err != nil {
		return nil, err
	}

	renderWrapper := newTemplateRenderWrapper(templateBuilder, cluster, ctx, cli)
	if err := renderWrapper.renderConfigTemplate(cluster, component, localObjs); err != nil {
		return nil, err
	}
	if err := renderWrapper.renderScriptTemplate(cluster, component, localObjs); err != nil {
		return nil, err
	}

	if len(renderWrapper.templateAnnotations) > 0 {
		updateResourceAnnotationsWithTemplate(obj, renderWrapper.templateAnnotations)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CreateOrUpdatePodVolumes(podSpec, renderWrapper.volumes); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := buildConfigManagerWithComponent(podSpec, component.ConfigTemplates, ctx, cli, cluster, component); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate sidecar for configmap's reloader")
	}

	return renderWrapper.renderedObjs, nil
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
func buildConfigManagerWithComponent(podSpec *corev1.PodSpec, cfgTemplates []appsv1alpha1.ComponentConfigSpec,
	ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) error {
	var (
		err error

		volumeDirs  []corev1.VolumeMount
		buildParams *cfgcm.CfgManagerBuildParams
	)

	if volumeDirs = getUsingVolumesByConfigSpecs(podSpec, cfgTemplates); len(volumeDirs) == 0 {
		return nil
	}
	if buildParams, err = buildConfigManagerParams(cli, ctx, cluster, component, cfgTemplates, volumeDirs); err != nil {
		return err
	}
	if buildParams == nil {
		return nil
	}

	container, err := builder.BuildCfgManagerContainer(buildParams)
	if err != nil {
		return err
	}
	updateTPLScriptVolume(podSpec, buildParams)

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)

	// This sidecar container will be able to view and signal processes from other containers
	podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
	return nil
}

func updateTPLScriptVolume(podSpec *corev1.PodSpec, configManager *cfgcm.CfgManagerBuildParams) {
	scriptVolume := configManager.ScriptVolume
	if scriptVolume == nil {
		return
	}

	// Ignore useless configtemplate
	podVolumes := podSpec.Volumes
	podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, scriptVolume.Name, func(volumeName string) corev1.Volume {
		return *scriptVolume
	}, nil)
	podSpec.Volumes = podVolumes
}

func getUsingVolumesByConfigSpecs(podSpec *corev1.PodSpec, cfgTemplates []appsv1alpha1.ComponentConfigSpec) []corev1.VolumeMount {
	var usingContainers []*corev1.Container

	// Ignore useless configTemplate
	firstCfg := 0
	for i, tpl := range cfgTemplates {
		usingContainers = intctrlutil.GetPodContainerWithVolumeMount(podSpec, tpl.VolumeName)
		if len(usingContainers) > 0 {
			firstCfg = i
			break
		}
	}

	// No container using any config template
	if len(usingContainers) == 0 {
		log.Log.Info(fmt.Sprintf("tpl config is not used by any container, and pass. tpl configs: %v", cfgTemplates))
		return nil
	}

	// Find first container using
	// Find out which configurations are used by the container
	volumeDirs := make([]corev1.VolumeMount, 0, len(cfgTemplates)+1)
	container := usingContainers[0]
	for i := firstCfg; i < len(cfgTemplates); i++ {
		tpl := cfgTemplates[i]
		// Ignore config template, e.g scripts configmap
		if !cfgcore.NeedReloadVolume(tpl) {
			continue
		}
		volume := intctrlutil.GetVolumeMountByVolume(container, tpl.VolumeName)
		if volume != nil {
			volumeDirs = append(volumeDirs, *volume)
		}
	}
	return volumeDirs
}

func buildConfigManagerParams(cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster,
	comp *component.SynthesizedComponent, configSpec []appsv1alpha1.ComponentConfigSpec, volumeDirs []corev1.VolumeMount) (*cfgcm.CfgManagerBuildParams, error) {
	var (
		err             error
		reloadOptions   *appsv1alpha1.ReloadOptions
		formatterConfig *appsv1alpha1.FormatterConfig
	)

	configManagerParams := &cfgcm.CfgManagerBuildParams{
		ManagerName:   constant.ConfigSidecarName,
		CharacterType: comp.CharacterType,
		SecreteName:   component.GenerateConnCredential(cluster.Name),
		Image:         viper.GetString(constant.KBToolsImage),
		Volumes:       volumeDirs,
		Cluster:       cluster,
	}

	if reloadOptions, formatterConfig, err = cfgcore.GetReloadOptions(cli, ctx, configSpec); err != nil {
		return nil, err
	}
	if reloadOptions == nil || formatterConfig == nil {
		return nil, nil
	}
	if err = cfgcm.BuildConfigManagerContainerArgs(reloadOptions, volumeDirs, cli, ctx, configManagerParams, formatterConfig); err != nil {
		return nil, err
	}
	return configManagerParams, nil
}
