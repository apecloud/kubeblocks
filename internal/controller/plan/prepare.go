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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type CreateParams struct {
	ClusterDefinition *appsv1alpha1.ClusterDefinition
	ClusterVersion    *appsv1alpha1.ClusterVersion
	Cluster           *appsv1alpha1.Cluster
	Component         *component.SynthesizedComponent
	ApplyObjs         *[]client.Object
	CacheCtx          *map[string]interface{}
}

func (params CreateParams) ToBuilderParams() builder.BuilderParams {
	return builder.BuilderParams{
		ClusterDefinition: params.ClusterDefinition,
		ClusterVersion:    params.ClusterVersion,
		Cluster:           params.Cluster,
		Component:         params.Component,
	}
}

// needBuildPDB check whether the PodDisruptionBudget needs to be built
func needBuildPDB(params *CreateParams) bool {
	if params.Component.WorkloadType == appsv1alpha1.Consensus {
		// if MinReplicas is non-zero, build pdb
		// TODO: add ut
		return len(params.Component.MaxUnavailable) > 0
	}
	return intctrlutil.ExistsPDBSpec(params.Component.PodDisruptionBudgetSpec)
}

// PrepareComponentObjs generate all necessary sub-resources objects used in component,
// like Secret, ConfigMap, Service, StatefulSet, Deployment, Volume, PodDisruptionBudget etc.
// Generated resources are cached in (obj.(*createParams)).applyObjs.
func PrepareComponentObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*CreateParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	workloadProcessor := func(customSetup func(*corev1.ConfigMap) (client.Object, error)) error {
		envConfig, err := builder.BuildEnvConfig(params.ToBuilderParams())
		if err != nil {
			return err
		}
		*params.ApplyObjs = append(*params.ApplyObjs, envConfig)

		workload, err := customSetup(envConfig)
		if err != nil {
			return err
		}

		defer func() {
			// workload object should be appended last
			*params.ApplyObjs = append(*params.ApplyObjs, workload)
		}()

		svc, err := builder.BuildSvc(params.ToBuilderParams(), true)
		if err != nil {
			return err
		}
		*params.ApplyObjs = append(*params.ApplyObjs, svc)

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
						volumes, _ = intctrlutil.CheckAndUpdateVolume(volumes, v.Name, func(volumeName string) corev1.Volume {
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
		configs, err := buildCfg(*params, workload, podSpec, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			*params.ApplyObjs = append(*params.ApplyObjs, configs...)
		}
		// end render config
		return nil
	}

	switch params.Component.WorkloadType {
	case appsv1alpha1.Stateless:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildDeploy(reqCtx, params.ToBuilderParams())
			}); err != nil {
			return err
		}
	case appsv1alpha1.Stateful:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildSts(reqCtx, params.ToBuilderParams(), envConfig.Name)
			}); err != nil {
			return err
		}
	case appsv1alpha1.Consensus:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return buildConsensusSet(reqCtx, *params, envConfig.Name)
			}); err != nil {
			return err
		}
	case appsv1alpha1.Replication:
		// get the maximum value of params.component.Replicas and the number of existing statefulsets under the current component,
		// then construct statefulsets for creating replicationSet or handling horizontal scaling of the replicationSet.
		var existStsList = &appsv1.StatefulSetList{}
		if err := componentutil.GetObjectListByComponentName(reqCtx.Ctx, cli, params.Cluster, existStsList, params.Component.Name); err != nil {
			return err
		}
		replicaCount := math.Max(float64(len(existStsList.Items)), float64(params.Component.Replicas))

		for index := int32(0); index < int32(replicaCount); index++ {
			if err := workloadProcessor(
				func(envConfig *corev1.ConfigMap) (client.Object, error) {
					return buildReplicationSet(reqCtx, *params, envConfig.Name, index)
				}); err != nil {
				return err
			}
		}
	}

	if needBuildPDB(params) {
		pdb, err := builder.BuildPDB(params.ToBuilderParams())
		if err != nil {
			return err
		}
		*params.ApplyObjs = append(*params.ApplyObjs, pdb)
	}

	if params.Component.Service != nil && len(params.Component.Service.Ports) > 0 {
		svc, err := builder.BuildSvc(params.ToBuilderParams(), false)
		if err != nil {
			return err
		}
		if params.Component.WorkloadType == appsv1alpha1.Consensus {
			addLeaderSelectorLabels(svc, params.Component)
		}
		if params.Component.WorkloadType == appsv1alpha1.Replication {
			svc.Spec.Selector[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
		}
		*params.ApplyObjs = append(*params.ApplyObjs, svc)
	}

	return nil
}

// TODO multi roles with same accessMode support
func addLeaderSelectorLabels(service *corev1.Service, component *component.SynthesizedComponent) {
	leader := component.ConsensusSpec.Leader
	if len(leader.Name) > 0 {
		service.Spec.Selector[intctrlutil.RoleLabelKey] = leader.Name
	}
}

// buildReplicationSet builds a replication component on statefulSet.
func buildReplicationSet(reqCtx intctrlutil.RequestCtx,
	params CreateParams,
	envConfigName string,
	stsIndex int32) (*appsv1.StatefulSet, error) {
	sts, err := builder.BuildSts(reqCtx, params.ToBuilderParams(), envConfigName)
	if err != nil {
		return nil, err
	}
	// inject replicationSet pod env and role label.
	if sts, err = injectReplicationSetPodEnvAndLabel(params, sts, stsIndex); err != nil {
		return nil, err
	}
	// sts.Name rename and add role label.
	sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, stsIndex)
	sts.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Secondary)
	if stsIndex == *params.Component.PrimaryIndex {
		sts.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
	}
	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	// build replicationSet persistentVolumeClaim manually
	if err := buildReplicationSetPVC(params, sts); err != nil {
		return sts, err
	}
	return sts, nil
}

// buildReplicationSetPVC builds replicationSet persistentVolumeClaim manually,
// replicationSet does not manage pvc through volumeClaimTemplate defined on statefulSet,
// the purpose is convenient to convert between workloadTypes in the future (TODO).
func buildReplicationSetPVC(params CreateParams, sts *appsv1.StatefulSet) error {
	// generate persistentVolumeClaim objects used by replicationSet's pod from component.VolumeClaimTemplates
	// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
	pvcMap := replicationset.GeneratePVCFromVolumeClaimTemplates(sts, params.Component.VolumeClaimTemplates)
	for _, pvc := range pvcMap {
		buildPersistentVolumeClaimLabels(sts, pvc)
		*params.ApplyObjs = append(*params.ApplyObjs, pvc)
	}

	// binding persistentVolumeClaim to podSpec.Volumes
	podSpec := &sts.Spec.Template.Spec
	if podSpec == nil {
		return nil
	}
	podVolumes := podSpec.Volumes
	for _, pvc := range pvcMap {
		podVolumes, _ = intctrlutil.CheckAndUpdateVolume(podVolumes, pvc.Name, func(volumeName string) corev1.Volume {
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

func injectReplicationSetPodEnvAndLabel(params CreateParams, sts *appsv1.StatefulSet, index int32) (*appsv1.StatefulSet, error) {
	if params.Component.PrimaryIndex == nil {
		return nil, fmt.Errorf("component %s PrimaryIndex can not be nil", params.Component.Name)
	}
	svcName := strings.Join([]string{params.Cluster.Name, params.Component.Name, "headless"}, "-")
	for i := range sts.Spec.Template.Spec.Containers {
		c := &sts.Spec.Template.Spec.Containers[i]
		c.Env = append(c.Env, corev1.EnvVar{
			Name:      constant.KBPrefix + "_PRIMARY_POD_NAME",
			Value:     fmt.Sprintf("%s-%d-%d.%s", sts.Name, *params.Component.PrimaryIndex, 0, svcName),
			ValueFrom: nil,
		})
	}
	if index != *params.Component.PrimaryIndex {
		sts.Spec.Template.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Secondary)
	} else {
		sts.Spec.Template.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
	}
	return sts, nil
}

// buildConsensusSet build on a stateful set
func buildConsensusSet(reqCtx intctrlutil.RequestCtx,
	params CreateParams,
	envConfigName string) (*appsv1.StatefulSet, error) {
	sts, err := builder.BuildSts(reqCtx, params.ToBuilderParams(), envConfigName)
	if err != nil {
		return sts, err
	}

	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	return sts, err
}

// buildCfg generate volumes for PodTemplate, volumeMount for container, and configmap for config files
func buildCfg(params CreateParams,
	obj client.Object,
	podSpec *corev1.PodSpec,
	ctx context.Context,
	cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	tpls := params.Component.ConfigTemplates
	if len(tpls) == 0 {
		return nil, nil
	}

	clusterName := params.Cluster.Name
	namespaceName := params.Cluster.Namespace

	// New ConfigTemplateBuilder
	cfgTemplateBuilder := newCfgTemplateBuilder(clusterName, namespaceName, params.Cluster, params.ClusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := cfgTemplateBuilder.injectBuiltInObjectsAndFunctions(podSpec, tpls, params.Component); err != nil {
		return nil, err
	}

	configs := make([]client.Object, 0, len(tpls))
	volumes := make(map[string]appsv1alpha1.ConfigTemplate, len(tpls))
	// TODO Support Update ClusterVersionRef of Cluster
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	cfgLables := make(map[string]string, len(tpls))
	for _, tpl := range tpls {
		// Check config cm already exists
		cmName := cfgcore.GetInstanceCMName(obj, &tpl)
		volumes[cmName] = tpl
		// Configuration.kubeblocks.io/cfg-tpl-${ctpl-name}: ${cm-instance-name}
		cfgLables[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = cmName
		isExist, err := isAlreadyExists(cmName, params.Cluster.Namespace, ctx, cli)
		if err != nil {
			return nil, err
		}
		if isExist {
			continue
		}

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(cfgTemplateBuilder, cmName, tpl, params, ctx, cli)
		if err != nil {
			return nil, err
		}

		// The owner of the configmap object is a cluster of users,
		// in order to manage the life cycle of configmap
		if err := controllerutil.SetOwnerReference(params.Cluster, cm, scheme); err != nil {
			return nil, err
		}
		configs = append(configs, cm)
	}
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		updateStatefulLabelsWithTemplate(sts, cfgLables)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CheckAndUpdatePodVolumes(podSpec, volumes); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := updateConfigurationManagerWithComponent(params, podSpec, tpls, ctx, cli); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate sidecar for configmap's reloader")
	}

	return configs, nil
}

func updateConfigurationManagerWithComponent(
	params CreateParams,
	podSpec *corev1.PodSpec,
	cfgTemplates []appsv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) error {
	var (
		firstCfg        = 0
		usingContainers []*corev1.Container

		defaultVarRunVolumePath = "/var/run"
		criEndpointVolumeName   = "cri-runtime-endpoint"
		// criRuntimeEndpoint      = viper.GetString(cfgcore.CRIRuntimeEndpoint)
		// criType                 = viper.GetString(cfgcore.ConfigCRIType)
	)

	reloadOptions, err := cfgutil.GetReloadOptions(cli, ctx, cfgTemplates)
	if err != nil {
		return err
	}
	if reloadOptions == nil {
		return nil
	}
	if reloadOptions.UnixSignalTrigger == nil {
		// TODO support other reload type
		log.Log.Info("only unix signal type is supported!")
		return nil
	}

	// Ignore useless configtemplate
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

	// If you do not need to watch any configmap volume
	if len(volumeDirs) == 0 {
		log.Log.Info(fmt.Sprintf("volume for configmap is not used by any container, and pass. cm name: %v", cfgTemplates[firstCfg]))
		return nil
	}

	unixSignalOption := reloadOptions.UnixSignalTrigger
	configManagerArgs := cfgcm.BuildSignalArgs(*unixSignalOption, volumeDirs)

	mountPath := defaultVarRunVolumePath
	managerSidecar := &cfgcm.ConfigManagerSidecar{
		ManagerName: cfgcore.ConfigSidecarName,
		Image:       viper.GetString(cfgcore.ConfigSidecarIMAGE),
		Args:        configManagerArgs,
		// add cri sock path
		Volumes: append(volumeDirs, corev1.VolumeMount{
			Name:      criEndpointVolumeName,
			MountPath: mountPath,
		}),
	}

	if container, err = builder.BuildCfgManagerContainer(managerSidecar); err != nil {
		return err
	}

	podVolumes := podSpec.Volumes
	podVolumes, _ = intctrlutil.CheckAndUpdateVolume(podVolumes, criEndpointVolumeName, func(volumeName string) corev1.Volume {
		return corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mountPath,
				},
			},
		}
	}, nil)
	podSpec.Volumes = podVolumes

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)

	// This sidecar container will be able to view and signal processes from other containers
	podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
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

func isAlreadyExists(cmName string, namespace string, ctx context.Context, cli client.Client) (bool, error) {
	cmKey := client.ObjectKey{
		Name:      cmName,
		Namespace: namespace,
	}

	cmObj := &corev1.ConfigMap{}
	cmErr := cli.Get(ctx, cmKey, cmObj)
	if cmErr != nil && apierrors.IsNotFound(cmErr) {
		// Config is not exists
		return false, nil
	} else if cmErr != nil {
		// An unexpected error occurs
		// TODO process unexpected error
		return true, cmErr
	}

	return true, nil
}

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder,
	cmName string,
	tplCfg appsv1alpha1.ConfigTemplate,
	params CreateParams,
	ctx context.Context,
	cli client.Client) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := processConfigMapTemplate(tplBuilder, tplCfg, ctx, cli)
	if err != nil {
		return nil, err
	}

	// Using ConfigMap cue template render to configmap of config
	return builder.BuildConfigMapWithTemplate(configs, params.ToBuilderParams(), cmName, tplCfg)
}

// processConfigMapTemplate Render config file using template engine
func processConfigMapTemplate(
	tplBuilder *configTemplateBuilder,
	tplCfg appsv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cfgTemplate := &appsv1alpha1.ConfigConstraint{}
	if len(tplCfg.ConfigConstraintRef) > 0 {
		if err := cli.Get(ctx, client.ObjectKey{
			Namespace: "",
			Name:      tplCfg.ConfigConstraintRef,
		}, cfgTemplate); err != nil {
			return nil, cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", tplCfg)
		}
	}

	// NOTE: not require checker configuration template status
	cfgChecker := cfgcore.NewConfigValidator(&cfgTemplate.Spec)
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

	// NOTE: It is necessary to verify the correctness of the data
	if err := cfgChecker.Validate(renderedCfg); err != nil {
		return nil, cfgcore.WrapError(err, "failed to validate configmap")
	}

	return renderedCfg, nil
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
