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
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func BuildCfgLow(clusterVersion *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	obj client.Object,
	podSpec *corev1.PodSpec,
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
	// TODO(refactor): need task parameter
	if err := templateBuilder.injectBuiltInObjectsAndFunctions(podSpec, component.ConfigTemplates, component, nil); err != nil {
		return nil, err
	}

	renderWrapper := newTemplateRenderWrapper(templateBuilder, cluster, ctx, cli)
	if err := renderWrapper.renderConfigTemplate(cluster, component, nil); err != nil {
		return nil, err
	}
	if err := renderWrapper.renderScriptTemplate(cluster, component, nil); err != nil {
		return nil, err
	}

	if len(renderWrapper.templateAnnotations) > 0 {
		updateResourceAnnotationsWithTemplate(obj, renderWrapper.templateAnnotations)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CreateOrUpdatePodVolumes(podSpec, renderWrapper.volumes); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := updateConfigManagerWithComponent(podSpec, component.ConfigTemplates, ctx, cli, cluster, component); err != nil {
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

// updateConfigManagerWithComponent build the configmgr sidecar container and update it
// into PodSpec if configuration reload option is on
func updateConfigManagerWithComponent(podSpec *corev1.PodSpec, cfgTemplates []appsv1alpha1.ComponentConfigSpec,
	ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) error {
	var (
		err error

		volumeDirs  []corev1.VolumeMount
		buildParams *cfgcm.CfgManagerBuildParams
	)

	if volumeDirs = getUsingVolumesByCfgTemplates(podSpec, cfgTemplates); len(volumeDirs) == 0 {
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

func getUsingVolumesByCfgTemplates(podSpec *corev1.PodSpec, cfgTemplates []appsv1alpha1.ComponentConfigSpec) []corev1.VolumeMount {
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
		if !cfgutil.NeedReloadVolume(tpl) {
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

	if reloadOptions, formatterConfig, err = cfgutil.GetReloadOptions(cli, ctx, configSpec); err != nil {
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
