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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	cfgutil "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
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
		envConfig, err := builder.BuildEnvConfig(task.GetBuilderParams())
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

		svc, err := builder.BuildSvc(task.GetBuilderParams(), true)
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
		configs, err := buildCfg(task, workload, podSpec, reqCtx.Ctx, cli)
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

	switch task.Component.WorkloadType {
	case appsv1alpha1.Stateless:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildDeploy(reqCtx, task.GetBuilderParams())
			}); err != nil {
			return err
		}
	case appsv1alpha1.Stateful:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildSts(reqCtx, task.GetBuilderParams(), envConfig.Name)
			}); err != nil {
			return err
		}
	case appsv1alpha1.Consensus:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return buildConsensusSet(reqCtx, task, envConfig.Name)
			}); err != nil {
			return err
		}
	case appsv1alpha1.Replication:
		// get the maximum value of params.component.Replicas and the number of existing statefulsets under the current component,
		// then construct statefulsets for creating replicationSet or handling horizontal scaling of the replicationSet.
		var existStsList = &appsv1.StatefulSetList{}
		if err := componentutil.GetObjectListByComponentName(reqCtx.Ctx, cli, task.Cluster, existStsList, task.Component.Name); err != nil {
			return err
		}
		replicaCount := math.Max(float64(len(existStsList.Items)), float64(task.Component.Replicas))

		for index := int32(0); index < int32(replicaCount); index++ {
			if err := workloadProcessor(
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

	if task.Component.Service != nil && len(task.Component.Service.Ports) > 0 {
		svc, err := builder.BuildSvc(task.GetBuilderParams(), false)
		if err != nil {
			return err
		}
		if task.Component.WorkloadType == appsv1alpha1.Consensus {
			addLeaderSelectorLabels(svc, task.Component)
		}
		if task.Component.WorkloadType == appsv1alpha1.Replication {
			svc.Spec.Selector[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
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
		service.Spec.Selector[intctrlutil.RoleLabelKey] = leader.Name
	}
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
	// inject replicationSet pod env and role label.
	if sts, err = injectReplicationSetPodEnvAndLabel(task, sts, stsIndex); err != nil {
		return nil, err
	}
	// sts.Name rename and add role label.
	sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, stsIndex)
	sts.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Secondary)
	if stsIndex == *task.Component.PrimaryIndex {
		sts.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
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
	for _, pvc := range pvcMap {
		buildPersistentVolumeClaimLabels(sts, pvc)
		task.AppendResource(pvc)
	}

	// binding persistentVolumeClaim to podSpec.Volumes
	podSpec := &sts.Spec.Template.Spec
	if podSpec == nil {
		return nil
	}
	podVolumes := podSpec.Volumes
	for _, pvc := range pvcMap {
		podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, pvc.Name, func(volumeName string) corev1.Volume {
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

func injectReplicationSetPodEnvAndLabel(task *intctrltypes.ReconcileTask, sts *appsv1.StatefulSet, index int32) (*appsv1.StatefulSet, error) {
	if task.Component.PrimaryIndex == nil {
		return nil, fmt.Errorf("component %s PrimaryIndex can not be nil", task.Component.Name)
	}
	svcName := strings.Join([]string{task.Cluster.Name, task.Component.Name, "headless"}, "-")
	for i := range sts.Spec.Template.Spec.Containers {
		c := &sts.Spec.Template.Spec.Containers[i]
		c.Env = append(c.Env, corev1.EnvVar{
			Name:      constant.KBPrefix + "_PRIMARY_POD_NAME",
			Value:     fmt.Sprintf("%s-%d-%d.%s", sts.Name, *task.Component.PrimaryIndex, 0, svcName),
			ValueFrom: nil,
		})
	}
	if index != *task.Component.PrimaryIndex {
		sts.Spec.Template.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Secondary)
	} else {
		sts.Spec.Template.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
	}
	return sts, nil
}

// buildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels on the sts to the pvc labels.
func buildPersistentVolumeClaimLabels(sts *appsv1.StatefulSet, pvc *corev1.PersistentVolumeClaim) {
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[intctrlutil.VolumeClaimTemplateNameLabelKey] = pvc.Name
	for k, v := range sts.Labels {
		if _, ok := pvc.Labels[k]; !ok {
			pvc.Labels[k] = v
		}
	}
}

// buildCfg generate volumes for PodTemplate, volumeMount for container, and configmap for config files
func buildCfg(task *intctrltypes.ReconcileTask,
	obj client.Object,
	podSpec *corev1.PodSpec,
	ctx context.Context,
	cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	tpls := task.Component.ConfigTemplates
	if len(tpls) == 0 {
		return nil, nil
	}

	clusterName := task.Cluster.Name
	namespaceName := task.Cluster.Namespace

	// New ConfigTemplateBuilder
	cfgTemplateBuilder := newCfgTemplateBuilder(clusterName, namespaceName, task.Cluster, task.ClusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := cfgTemplateBuilder.injectBuiltInObjectsAndFunctions(podSpec, tpls, task.Component); err != nil {
		return nil, err
	}

	configs := make([]client.Object, 0, len(tpls))
	volumes := make(map[string]appsv1alpha1.ConfigTemplate, len(tpls))
	// TODO Support Update ClusterVersionRef of Cluster
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	cfgLables := make(map[string]string, len(tpls))
	for _, tpl := range tpls {
		cmName := cfgcore.GetInstanceCMName(obj, &tpl)
		volumes[cmName] = tpl
		// Configuration.kubeblocks.io/cfg-tpl-${ctpl-name}: ${cm-instance-name}
		cfgLables[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = cmName

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(cfgTemplateBuilder, cmName, tpl, task, ctx, cli)
		if err != nil {
			return nil, err
		}
		updateCMConfigSelectorLabels(cm, tpl)

		// The owner of the configmap object is a cluster of users,
		// in order to manage the life cycle of configmap
		if err := controllerutil.SetOwnerReference(task.Cluster, cm, scheme); err != nil {
			return nil, err
		}
		configs = append(configs, cm)
	}
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		updateStatefulLabelsWithTemplate(sts, cfgLables)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CreateOrUpdatePodVolumes(podSpec, volumes); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := updateConfigurationManagerWithComponent(podSpec, tpls, ctx, cli); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate sidecar for configmap's reloader")
	}

	return configs, nil
}

func updateCMConfigSelectorLabels(cm *corev1.ConfigMap, tpl appsv1alpha1.ConfigTemplate) {
	if len(tpl.Keys) == 0 {
		return
	}
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[cfgcore.CMConfigurationCMKeysLabelKey] = strings.Join(tpl.Keys, ",")
}

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder,
	cmName string,
	tplCfg appsv1alpha1.ConfigTemplate,
	task *intctrltypes.ReconcileTask,
	ctx context.Context,
	cli client.Client) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := renderConfigMap(tplBuilder, tplCfg, ctx, cli)
	if err != nil {
		return nil, err
	}

	err = validateConfigMap(configs, tplCfg, ctx, cli)
	if err != nil {
		return nil, err
	}

	// Using ConfigMap cue template render to configmap of config
	return builder.BuildConfigMapWithTemplate(configs, task.GetBuilderParams(), cmName, tplCfg)
}

// renderConfigMap render config file using template engine
func renderConfigMap(
	tplBuilder *configTemplateBuilder,
	tplCfg appsv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tplCfg.Namespace,
		Name:      tplCfg.ConfigTplRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	tplBuilder.setTplName(tplCfg.ConfigTplRef)
	renderedCfg, err := tplBuilder.render(cmObj.Data)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to render configmap")
	}
	return renderedCfg, nil
}

// validateConfigMap validate config file against constraint
func validateConfigMap(
	renderedCfg map[string]string,
	tplCfg appsv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) error {
	cfgTemplate := &appsv1alpha1.ConfigConstraint{}
	if len(tplCfg.ConfigConstraintRef) > 0 {
		if err := cli.Get(ctx, client.ObjectKey{
			Namespace: "",
			Name:      tplCfg.ConfigConstraintRef,
		}, cfgTemplate); err != nil {
			return cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", tplCfg)
		}
	}

	// NOTE: not require checker configuration template status
	cfgChecker := cfgcore.NewConfigValidator(&cfgTemplate.Spec)

	// NOTE: It is necessary to verify the correctness of the data
	if err := cfgChecker.Validate(renderedCfg); err != nil {
		return cfgcore.WrapError(err, "failed to validate configmap")
	}

	return nil
}

func updateStatefulLabelsWithTemplate(sts *appsv1.StatefulSet, allLabels map[string]string) {
	// full configmap upgrade
	existLabels := make(map[string]string)
	for key, val := range sts.Labels {
		if strings.HasPrefix(key, cfgcore.ConfigurationTplLabelPrefixKey) {
			existLabels[key] = val
		}
	}

	// delete not exist configmap label
	deletedLabels := cfgcore.MapKeyDifference(existLabels, allLabels)
	for l := range deletedLabels.Iter() {
		delete(sts.Labels, l)
	}

	for key, val := range allLabels {
		sts.Labels[key] = val
	}
}

// updateConfigurationManagerWithComponent build the configmgr sidecar container and update it
// into PodSpec if configuration reload option is on
func updateConfigurationManagerWithComponent(
	podSpec *corev1.PodSpec,
	cfgTemplates []appsv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) error {
	var (
		volumeDirs     []corev1.VolumeMount
		managerSidecar *cfgcm.ConfigManagerSidecar
		err            error

		defaultVarRunVolumePath = "/var/run"
		criEndpointVolumeName   = "cri-runtime-endpoint"
		// criRuntimeEndpoint      = viper.GetString(cfgcore.CRIRuntimeEndpoint)
		// criType                 = viper.GetString(cfgcore.ConfigCRIType)
	)

	if volumeDirs = getUsingVolumesByCfgTemplates(podSpec, cfgTemplates); len(volumeDirs) == 0 {
		return nil
	}
	if managerSidecar, err = buildConfigManagerParams(cli, ctx, cfgTemplates, volumeDirs, defaultVarRunVolumePath, criEndpointVolumeName); err != nil {
		return err
	}
	if managerSidecar == nil {
		return nil
	}

	container, err := builder.BuildCfgManagerContainer(managerSidecar)
	if err != nil {
		return err
	}
	updateCRIContainerVolume(podSpec, defaultVarRunVolumePath, criEndpointVolumeName)

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)

	// This sidecar container will be able to view and signal processes from other containers
	podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
	return nil
}

func updateCRIContainerVolume(podSpec *corev1.PodSpec, volumePath string, volumeName string) {
	podVolumes := podSpec.Volumes
	podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, volumeName, func(volumeName string) corev1.Volume {
		return corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumePath,
				},
			},
		}
	}, nil)
	podSpec.Volumes = podVolumes
}

func getUsingVolumesByCfgTemplates(podSpec *corev1.PodSpec, cfgTemplates []appsv1alpha1.ConfigTemplate) []corev1.VolumeMount {
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

func buildConfigManagerParams(cli client.Client, ctx context.Context, cfgTemplates []appsv1alpha1.ConfigTemplate, volumeDirs []corev1.VolumeMount, volumePath string, volumeName string) (*cfgcm.ConfigManagerSidecar, error) {
	var (
		err               error
		reloadOptions     *appsv1alpha1.ReloadOptions
		configManagerArgs []string
	)

	if reloadOptions, err = cfgutil.GetReloadOptions(cli, ctx, cfgTemplates); err != nil {
		return nil, err
	}
	if reloadOptions == nil || reloadOptions.UnixSignalTrigger == nil {
		return nil, nil
	}

	unixSignalOption := reloadOptions.UnixSignalTrigger
	configManagerArgs = cfgcm.BuildSignalArgs(*unixSignalOption, volumeDirs)
	configManager := &cfgcm.ConfigManagerSidecar{
		ManagerName: cfgcore.ConfigSidecarName,
		Image:       viper.GetString(cfgcore.ConfigSidecarIMAGE),
		Args:        configManagerArgs,
		// add cri sock path
		Volumes: append(volumeDirs, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: volumePath,
		}),
	}
	return configManager, nil
}
