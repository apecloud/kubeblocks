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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	cfgutil "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// PrepareComponentResources generate all necessary sub-resources objects used in component,
// like Secret, ConfigMap, Service, StatefulSet, Deployment, Volume, PodDisruptionBudget etc.
// Generated resources are cached in task.applyObjs.
func PrepareComponentResources(reqCtx intctrlutil.RequestCtx, cli client.Client, task *intctrltypes.ReconcileTask) error {
	workloadProcessor := func(customSetup func(*corev1.ConfigMap) (client.Object, error)) error {
		envConfig, err := builder.BuildEnvConfig(task.GetBuilderParams(), reqCtx, cli)
		if err != nil {
			return err
		}
		task.AppendResource(envConfig)

		workload, err := customSetup(envConfig)
		if err != nil {
			return err
		}

		defer func() {
			// workload object should be appended last
			task.AppendResource(workload)
		}()

		svc, err := builder.BuildHeadlessSvc(task.GetBuilderParams())
		if err != nil {
			return err
		}
		task.AppendResource(svc)

		var podSpec *corev1.PodSpec
		sts, ok := workload.(*appsv1.StatefulSet)
		if ok {
			podSpec = &sts.Spec.Template.Spec
		} else {
			deploy, ok := workload.(*appsv1.Deployment)
			if ok {
				podSpec = &deploy.Spec.Template.Spec
			}
		}
		if podSpec == nil {
			return nil
		}

		defer func() {
			for _, cc := range []*[]corev1.Container{
				&podSpec.Containers,
				&podSpec.InitContainers,
			} {
				volumes := podSpec.Volumes
				for _, c := range *cc {
					for _, v := range c.VolumeMounts {
						// if persistence is not found, add emptyDir pod.spec.volumes[]
						volumes, _ = intctrlutil.CreateOrUpdateVolume(volumes, v.Name, func(volumeName string) corev1.Volume {
							return corev1.Volume{
								Name: v.Name,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							}
						}, nil)
					}
				}
				podSpec.Volumes = volumes
			}
		}()

		// render config template
		configs, err := renderConfigNScriptFiles(task, workload, podSpec, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			task.AppendResource(configs...)
		}
		// end render config

		// tls certs secret volume and volumeMount
		if err := updateTLSVolumeAndVolumeMount(podSpec, task.Cluster.Name, *task.Component); err != nil {
			return err
		}
		return nil
	}

	// pre-condition check
	if task.Component.WorkloadType == appsv1alpha1.Replication {
		// get the number of existing pods under the current component
		var existPodList = &corev1.PodList{}
		if err := componentutil.GetObjectListByComponentName(reqCtx.Ctx, cli, *task.Cluster, existPodList, task.Component.Name); err != nil {
			return err
		}

		// If the Pods already exists, check whether there is an HA switching and the HA process is prioritized to handle.
		// TODO: (xingran) After refactoring, HA switching will be handled in the replicationSet controller.
		if len(existPodList.Items) > 0 {
			primaryIndexChanged, _, err := replication.CheckPrimaryIndexChanged(reqCtx.Ctx, cli, task.Cluster,
				task.Component.Name, task.Component.GetPrimaryIndex())
			if err != nil {
				return err
			}
			if primaryIndexChanged {
				if err := replication.HandleReplicationSetHASwitch(reqCtx.Ctx, cli, task.Cluster,
					componentutil.GetClusterComponentSpecByName(*task.Cluster, task.Component.Name)); err != nil {
					return err
				}
			}
		}

	}

	// REVIEW/TODO:
	// - need higher level abstraction handling
	// - or move this module to part operator controller handling
	switch task.Component.WorkloadType {
	case appsv1alpha1.Stateless:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildDeploy(reqCtx, task.GetBuilderParams())
			}); err != nil {
			return err
		}
	case appsv1alpha1.Stateful, appsv1alpha1.Consensus, appsv1alpha1.Replication:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildSts(reqCtx, task.GetBuilderParams(), envConfig.Name)
			}); err != nil {
			return err
		}
	}

	// conditional build PodDisruptionBudget
	if task.Component.MaxUnavailable != nil {
		pdb, err := builder.BuildPDB(task.GetBuilderParams())
		if err != nil {
			return err
		}
		task.AppendResource(pdb)
	}

	svcList, err := builder.BuildSvcListWithCustomAttributes(task.GetBuilderParams(), func(svc *corev1.Service) {
		switch task.Component.WorkloadType {
		case appsv1alpha1.Consensus:
			addLeaderSelectorLabels(svc, task.Component)
		case appsv1alpha1.Replication:
			svc.Spec.Selector[constant.RoleLabelKey] = string(replication.Primary)
		}
	})
	if err != nil {
		return err
	}
	for _, svc := range svcList {
		task.AppendResource(svc)
	}
	return nil
}

// TODO multi roles with same accessMode support
func addLeaderSelectorLabels(service *corev1.Service, component *component.SynthesizedComponent) {
	leader := component.ConsensusSpec.Leader
	if len(leader.Name) > 0 {
		service.Spec.Selector[constant.RoleLabelKey] = leader.Name
	}
}

// renderConfigNScriptFiles generate volumes for PodTemplate, volumeMount for container, rendered configTemplate and scriptTemplate,
// and generate configManager sidecar for the reconfigure operation.
// TODO rename this function, this function name is not very reasonable, but there is no suitable name.
func renderConfigNScriptFiles(task *intctrltypes.ReconcileTask,
	obj client.Object,
	podSpec *corev1.PodSpec,
	ctx context.Context,
	cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	if len(task.Component.ConfigTemplates) == 0 && len(task.Component.ScriptTemplates) == 0 {
		return nil, nil
	}

	clusterName := task.Cluster.Name
	namespaceName := task.Cluster.Namespace
	// New ConfigTemplateBuilder
	templateBuilder := newTemplateBuilder(clusterName, namespaceName, task.Cluster, task.ClusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := templateBuilder.injectBuiltInObjectsAndFunctions(podSpec, task.Component.ConfigTemplates, task.Component, task); err != nil {
		return nil, err
	}

	renderWrapper := newTemplateRenderWrapper(templateBuilder, task.Cluster, task.GetBuilderParams(), ctx, cli)
	if err := renderWrapper.renderConfigTemplate(task); err != nil {
		return nil, err
	}
	if err := renderWrapper.renderScriptTemplate(task); err != nil {
		return nil, err
	}

	if len(renderWrapper.templateAnnotations) > 0 {
		updateResourceAnnotationsWithTemplate(obj, renderWrapper.templateAnnotations)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CreateOrUpdatePodVolumes(podSpec, renderWrapper.volumes); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := updateConfigManagerWithComponent(podSpec, task.Component.ConfigTemplates, ctx, cli, task.GetBuilderParams()); err != nil {
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
func updateConfigManagerWithComponent(podSpec *corev1.PodSpec, cfgTemplates []appsv1alpha1.ComponentConfigSpec, ctx context.Context, cli client.Client, params builder.BuilderParams) error {
	var (
		err error

		volumeDirs  []corev1.VolumeMount
		buildParams *cfgcm.CfgManagerBuildParams
	)

	if volumeDirs = getUsingVolumesByCfgTemplates(podSpec, cfgTemplates); len(volumeDirs) == 0 {
		return nil
	}
	if buildParams, err = buildConfigManagerParams(cli, ctx, cfgTemplates, volumeDirs, params); err != nil {
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

func buildConfigManagerParams(cli client.Client, ctx context.Context, configSpec []appsv1alpha1.ComponentConfigSpec, volumeDirs []corev1.VolumeMount, params builder.BuilderParams) (*cfgcm.CfgManagerBuildParams, error) {
	var (
		err             error
		reloadOptions   *appsv1alpha1.ReloadOptions
		formatterConfig *appsv1alpha1.FormatterConfig
	)

	configManagerParams := &cfgcm.CfgManagerBuildParams{
		ManagerName:   constant.ConfigSidecarName,
		CharacterType: params.Component.CharacterType,
		SecreteName:   component.GenerateConnCredential(params.Cluster.Name),
		Image:         viper.GetString(constant.KBToolsImage),
		Volumes:       volumeDirs,
		Cluster:       params.Cluster,
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
