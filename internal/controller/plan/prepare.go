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
	"math"
	"strings"

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	cfgutil "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
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

	switch task.Component.WorkloadType {
	case appsv1alpha1.Stateless:
		if err := prepareComponentWorkloads(reqCtx, cli, task,
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildDeploy(reqCtx, task.GetBuilderParams())
			}); err != nil {
			return err
		}
	case appsv1alpha1.Stateful:
		if err := prepareComponentWorkloads(reqCtx, cli, task,
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildSts(reqCtx, task.GetBuilderParams(), envConfig.Name)
			}); err != nil {
			return err
		}
	case appsv1alpha1.Consensus:
		if err := prepareComponentWorkloads(reqCtx, cli, task,
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return buildConsensusSet(reqCtx, task, envConfig.Name)
			}); err != nil {
			return err
		}
	case appsv1alpha1.Replication:
		// get the number of existing statefulsets under the current component
		var existStsList = &appsv1.StatefulSetList{}
		if err := componentutil.GetObjectListByComponentName(reqCtx.Ctx, cli, *task.Cluster, existStsList, task.Component.Name); err != nil {
			return err
		}

		// If the statefulSets already exists, check whether there is an HA switching and the HA process is prioritized to handle.
		// TODO(xingran) After refactoring, HA switching will be handled in the replicationSet controller.
		if len(existStsList.Items) > 0 {
			primaryIndexChanged, _, err := replicationset.CheckPrimaryIndexChanged(reqCtx.Ctx, cli, task.Cluster,
				task.Component.Name, task.Component.GetPrimaryIndex())
			if err != nil {
				return err
			}
			if primaryIndexChanged {
				if err := replicationset.HandleReplicationSetHASwitch(reqCtx.Ctx, cli, task.Cluster,
					componentutil.GetClusterComponentSpecByName(*task.Cluster, task.Component.Name)); err != nil {
					return err
				}
			}
		}

		// get the maximum value of params.component.Replicas and the number of existing statefulsets under the current component,
		//  then construct statefulsets for creating replicationSet or handling horizontal scaling of the replicationSet.
		replicaCount := math.Max(float64(len(existStsList.Items)), float64(task.Component.Replicas))
		for index := int32(0); index < int32(replicaCount); index++ {
			if err := prepareComponentWorkloads(reqCtx, cli, task,
				func(envConfig *corev1.ConfigMap) (client.Object, error) {
					return buildReplicationSet(reqCtx, task, envConfig.Name, index)
				}); err != nil {
				return err
			}
		}
	}

	if needBuildPDB(task) {
		pdb, err := builder.BuildPDB(task.GetBuilderParams())
		if err != nil {
			return err
		}
		task.AppendResource(pdb)
	}

	svcList, err := builder.BuildSvcList(task.GetBuilderParams())
	if err != nil {
		return err
	}
	for _, svc := range svcList {
		if task.Component.WorkloadType == appsv1alpha1.Consensus {
			addLeaderSelectorLabels(svc, task.Component)
		}
		if task.Component.WorkloadType == appsv1alpha1.Replication {
			svc.Spec.Selector[constant.RoleLabelKey] = string(replicationset.Primary)
		}
		task.AppendResource(svc)
	}

	return nil
}

// needBuildPDB check whether the PodDisruptionBudget needs to be built
func needBuildPDB(task *intctrltypes.ReconcileTask) bool {
	// TODO: add ut
	comp := task.Component
	return comp.WorkloadType == appsv1alpha1.Consensus && comp.MaxUnavailable != nil
}

// TODO multi roles with same accessMode support
func addLeaderSelectorLabels(service *corev1.Service, component *component.SynthesizedComponent) {
	leader := component.ConsensusSpec.Leader
	if len(leader.Name) > 0 {
		service.Spec.Selector[constant.RoleLabelKey] = leader.Name
	}
}

func prepareComponentWorkloads(reqCtx intctrlutil.RequestCtx, cli client.Client, task *intctrltypes.ReconcileTask,
	customSetup func(*corev1.ConfigMap) (client.Object, error)) error {
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
	configs, err := BuildCfg(task, workload, podSpec, reqCtx.Ctx, cli)
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

// buildConsensusSet build on a stateful set
func buildConsensusSet(reqCtx intctrlutil.RequestCtx,
	task *intctrltypes.ReconcileTask,
	envConfigName string) (*appsv1.StatefulSet, error) {
	sts, err := builder.BuildSts(reqCtx, task.GetBuilderParams(), envConfigName)
	if err != nil {
		return sts, err
	}

	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	return sts, err
}

// buildReplicationSet builds a replication component on statefulSet.
func buildReplicationSet(reqCtx intctrlutil.RequestCtx,
	task *intctrltypes.ReconcileTask,
	envConfigName string,
	stsIndex int32) (*appsv1.StatefulSet, error) {
	sts, err := builder.BuildSts(reqCtx, task.GetBuilderParams(), envConfigName)
	if err != nil {
		return nil, err
	}
	// sts.Name renamed with suffix "-<stsIdx>" for subsequent sts workload
	if stsIndex != 0 {
		sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, stsIndex)
	}
	if stsIndex == task.Component.GetPrimaryIndex() {
		sts.Labels[constant.RoleLabelKey] = string(replicationset.Primary)
	} else {
		sts.Labels[constant.RoleLabelKey] = string(replicationset.Secondary)
	}
	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	// build replicationSet persistentVolumeClaim manually
	if err := buildReplicationSetPVC(task, sts); err != nil {
		return sts, err
	}
	return sts, nil
}

// buildReplicationSetPVC builds replicationSet persistentVolumeClaim manually,
// replicationSet does not manage pvc through volumeClaimTemplate defined on statefulSet,
// the purpose is convenient to convert between workloadTypes in the future (TODO).
func buildReplicationSetPVC(task *intctrltypes.ReconcileTask, sts *appsv1.StatefulSet) error {
	// generate persistentVolumeClaim objects used by replicationSet's pod from component.VolumeClaimTemplates
	// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
	pvcMap := replicationset.GeneratePVCFromVolumeClaimTemplates(sts, task.Component.VolumeClaimTemplates)
	for pvcTplName, pvc := range pvcMap {
		builder.BuildPersistentVolumeClaimLabels(sts, pvc, task.Component, pvcTplName)
		task.AppendResource(pvc)
	}

	// binding persistentVolumeClaim to podSpec.Volumes
	podSpec := &sts.Spec.Template.Spec
	if podSpec == nil {
		return nil
	}
	podVolumes := podSpec.Volumes
	for _, pvc := range pvcMap {
		volumeName := strings.Split(pvc.Name, "-")[0]
		podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, volumeName, func(volumeName string) corev1.Volume {
			return corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			}
		}, nil)
	}
	podSpec.Volumes = podVolumes
	return nil
}

// BuildCfg generate volumes for PodTemplate, volumeMount for container, and configmap for config files
func BuildCfg(task *intctrltypes.ReconcileTask,
	obj client.Object,
	podSpec *corev1.PodSpec,
	ctx context.Context,
	cli client.Client) ([]client.Object, error) {
	return BuildCfgLow(task.ClusterVersion, task.Cluster, task.Component, obj, podSpec, ctx, cli)
}

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
	// New ConfigTemplateBuilder
	cfgTemplateBuilder := newCfgTemplateBuilder(clusterName, namespaceName, cluster, clusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := cfgTemplateBuilder.injectBuiltInObjectsAndFunctions(podSpec, component.ConfigTemplates, component); err != nil {
		return nil, err
	}

	renderWrapper := newTemplateRenderWrapper(cfgTemplateBuilder, cluster, ctx, cli)
	if err := renderWrapper.renderConfigTemplate(cluster, component); err != nil {
		return nil, err
	}
	if err := renderWrapper.renderScriptTemplate(cluster, component); err != nil {
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
	deletedLabels := cfgcore.MapKeyDifference(existLabels, allTemplateAnnotations)
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

		volumeDirs          []corev1.VolumeMount
		configManagerParams *cfgcm.ConfigManagerParams
	)

	if volumeDirs = getUsingVolumesByCfgTemplates(podSpec, cfgTemplates); len(volumeDirs) == 0 {
		return nil
	}
	if configManagerParams, err = buildConfigManagerParams(cli, ctx, cluster, component, cfgTemplates, volumeDirs); err != nil {
		return err
	}
	if configManagerParams == nil {
		return nil
	}

	container, err := builder.BuildCfgManagerContainer(configManagerParams)
	if err != nil {
		return err
	}
	updateTPLScriptVolume(podSpec, configManagerParams)

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)

	// This sidecar container will be able to view and signal processes from other containers
	podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
	return nil
}

func updateTPLScriptVolume(podSpec *corev1.PodSpec, configManager *cfgcm.ConfigManagerParams) {
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
	comp *component.SynthesizedComponent, cfgTemplates []appsv1alpha1.ComponentConfigSpec, volumeDirs []corev1.VolumeMount) (*cfgcm.ConfigManagerParams, error) {
	configManagerParams := &cfgcm.ConfigManagerParams{
		ManagerName:   constant.ConfigSidecarName,
		CharacterType: comp.CharacterType,
		SecreteName:   component.GenerateConnCredential(cluster.Name),
		Image:         viper.GetString(constant.ConfigSidecarIMAGE),
		Volumes:       volumeDirs,
		Cluster:       cluster,
	}

	var err error
	var reloadOptions *appsv1alpha1.ReloadOptions
	if reloadOptions, err = cfgutil.GetReloadOptions(cli, ctx, cfgTemplates); err != nil {
		return nil, err
	}
	if reloadOptions == nil {
		return nil, nil
	}
	if err = cfgcm.BuildConfigManagerContainerArgs(reloadOptions, volumeDirs, cli, ctx, configManagerParams); err != nil {
		return nil, err
	}
	return configManagerParams, nil
}
