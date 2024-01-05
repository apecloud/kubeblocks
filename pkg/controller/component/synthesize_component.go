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

package component

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	defaultShmQuantity = resource.MustParse("64Mi")
)

// BuildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from ComponentDefinition and Component.
func BuildSynthesizedComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component) (*SynthesizedComponent, error) {
	return buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, nil, cluster, nil)
}

// BuildSynthesizedComponent4Generated builds SynthesizedComponent for generated Component which w/o ComponentDefinition.
func BuildSynthesizedComponent4Generated(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component) (*appsv1alpha1.ComponentDefinition, *SynthesizedComponent, error) {
	clusterDef, clusterVer, err := getClusterReferencedResources(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return nil, nil, err
	}
	clusterCompSpec, err := getClusterCompSpec4Component(clusterDef, cluster, comp)
	if err != nil {
		return nil, nil, err
	}
	if clusterCompSpec == nil {
		return nil, nil, fmt.Errorf("cluster component spec is not found: %s", comp.Name)
	}
	compDef, err := getOrBuildComponentDefinition(reqCtx.Ctx, cli, clusterDef, clusterVer, cluster, clusterCompSpec)
	if err != nil {
		return nil, nil, err
	}
	synthesizedComp, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, clusterDef, clusterVer, cluster, clusterCompSpec)
	if err != nil {
		return nil, nil, err
	}
	return compDef, synthesizedComp, nil
}

// BuildSynthesizedComponentWrapper builds a new SynthesizedComponent object with a given ClusterComponentSpec.
// TODO: remove this
func BuildSynthesizedComponentWrapper(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*SynthesizedComponent, error) {
	clusterDef, clusterVer, err := getClusterReferencedResources(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	return BuildSynthesizedComponentWrapper4Test(reqCtx, cli, clusterDef, clusterVer, cluster, clusterCompSpec)
}

// BuildSynthesizedComponentWrapper4Test builds a new SynthesizedComponent object with a given ClusterComponentSpec.
func BuildSynthesizedComponentWrapper4Test(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*SynthesizedComponent, error) {
	if clusterCompSpec == nil {
		clusterCompSpec = apiconversion.HandleSimplifiedClusterAPI(clusterDef, cluster)
	}
	if clusterCompSpec == nil {
		return nil, nil
	}
	compDef, err := getOrBuildComponentDefinition(reqCtx.Ctx, cli, clusterDef, clusterVer, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	comp, err := BuildComponent(cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	return buildSynthesizedComponent(reqCtx, cli, compDef, comp, clusterDef, clusterVer, cluster, clusterCompSpec)
}

// buildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from ComponentDefinition and Component.
// !!! Do not use @clusterDef, @clusterVer, @cluster and @clusterCompSpec since they are used for the backward compatibility only.
// TODO: remove @reqCtx & @cli
func buildSynthesizedComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*SynthesizedComponent, error) {
	if compDef == nil || comp == nil {
		return nil, nil
	}

	clusterName, err := GetClusterName(comp)
	if err != nil {
		return nil, err
	}
	clusterUID, err := GetClusterUID(comp)
	if err != nil {
		return nil, err
	}
	compName, err := ShortName(clusterName, comp.Name)
	if err != nil {
		return nil, err
	}
	compDefObj := compDef.DeepCopy()
	synthesizeComp := &SynthesizedComponent{
		Namespace:          comp.Namespace,
		ClusterName:        clusterName,
		ClusterUID:         clusterUID,
		Comp2CompDefs:      buildComp2CompDefs(cluster, clusterCompSpec),
		Name:               compName,
		FullCompName:       comp.Name,
		CompDefName:        compDef.Name,
		ClusterGeneration:  clusterGeneration(cluster, comp),
		PodSpec:            &compDef.Spec.Runtime,
		LogConfigs:         compDefObj.Spec.LogConfigs,
		ConfigTemplates:    compDefObj.Spec.Configs,
		ScriptTemplates:    compDefObj.Spec.Scripts,
		Labels:             compDefObj.Spec.Labels,
		Roles:              compDefObj.Spec.Roles,
		UpdateStrategy:     compDefObj.Spec.UpdateStrategy,
		PolicyRules:        compDefObj.Spec.PolicyRules,
		LifecycleActions:   compDefObj.Spec.LifecycleActions,
		SystemAccounts:     compDefObj.Spec.SystemAccounts,
		RoleArbitrator:     compDefObj.Spec.RoleArbitrator,
		Replicas:           comp.Spec.Replicas,
		EnabledLogs:        comp.Spec.EnabledLogs,
		TLSConfig:          comp.Spec.TLSConfig,
		ServiceAccountName: comp.Spec.ServiceAccountName,
		Nodes:              comp.Spec.Nodes,
		Instances:          comp.Spec.Instances,
		RsmTransformPolicy: comp.Spec.RsmTransformPolicy,
	}

	// build backward compatible fields, including workload, services, componentRefEnvs, clusterDefName, clusterCompDefName, and clusterCompVer, etc.
	// if cluster referenced a clusterDefinition and clusterVersion, for backward compatibility, we need to merge the clusterDefinition and clusterVersion into the component
	// TODO(xingran): it will be removed in the future
	if clusterDef != nil && cluster != nil && clusterCompSpec != nil {
		if err = buildBackwardCompatibleFields(reqCtx, clusterDef, clusterVer, cluster, clusterCompSpec, synthesizeComp); err != nil {
			return nil, err
		}
	}

	// build affinity and tolerations
	if err := buildAffinitiesAndTolerations(comp, synthesizeComp); err != nil {
		reqCtx.Log.Error(err, "build affinities and tolerations failed.")
		return nil, err
	}

	// build and update resources
	// TODO(xingran): remove the dependency of SynthesizedComponent.ClusterDefName and SynthesizedComponent.ClusterCompDefName in the future
	if err := buildAndUpdateResources(reqCtx, cli, synthesizeComp, comp); err != nil {
		reqCtx.Log.Error(err, "build and update resources failed.")
		return nil, err
	}

	// build volumeClaimTemplates
	buildVolumeClaimTemplates(synthesizeComp, comp)

	limitSharedMemoryVolumeSize(synthesizeComp, comp)

	// build componentService
	buildComponentServices(synthesizeComp, compDefObj)

	// build monitor
	buildMonitorConfig(compDefObj.Spec.Monitor, comp.Spec.Monitor, &compDefObj.Spec.Runtime, synthesizeComp)

	// build serviceAccountName
	buildServiceAccountName(synthesizeComp)

	// build lorryContainer
	if err := buildLorryContainers(reqCtx, synthesizeComp, clusterCompSpec); err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
		return nil, err
	}

	// build serviceReferences
	if err = buildServiceReferences(reqCtx, cli, synthesizeComp, compDef, comp); err != nil {
		reqCtx.Log.Error(err, "build service references failed.")
		return nil, err
	}

	// replace podSpec containers env default credential placeholder
	replaceContainerPlaceholderTokens(synthesizeComp, GetEnvReplacementMapForConnCredential(synthesizeComp.ClusterName))

	return synthesizeComp, nil
}

func clusterGeneration(cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component) string {
	if comp != nil && comp.Annotations != nil {
		if generation, ok := comp.Annotations[constant.KubeBlocksGenerationKey]; ok {
			return generation
		}
	}
	// back-off to use cluster.Generation
	return strconv.FormatInt(cluster.Generation, 10)
}

func buildComp2CompDefs(cluster *appsv1alpha1.Cluster, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) map[string]string {
	if cluster == nil {
		return nil
	}
	if len(cluster.Spec.ComponentSpecs) == 0 {
		if clusterCompSpec == nil || len(clusterCompSpec.ComponentDef) == 0 {
			return nil
		}
		return map[string]string{
			clusterCompSpec.Name: clusterCompSpec.ComponentDef,
		}
	}
	mapping := make(map[string]string)
	for _, comp := range cluster.Spec.ComponentSpecs {
		if len(comp.ComponentDef) > 0 {
			mapping[comp.Name] = comp.ComponentDef
		}
	}
	return mapping
}

// buildAffinitiesAndTolerations builds affinities and tolerations for component.
func buildAffinitiesAndTolerations(comp *appsv1alpha1.Component, synthesizeComp *SynthesizedComponent) error {
	podAffinity, err := BuildPodAffinity(synthesizeComp.ClusterName, synthesizeComp.Name, comp.Spec.Affinity)
	if err != nil {
		return err
	}
	synthesizeComp.PodSpec.Affinity = podAffinity
	synthesizeComp.PodSpec.TopologySpreadConstraints =
		BuildPodTopologySpreadConstraints(synthesizeComp.ClusterName, synthesizeComp.Name, comp.Spec.Affinity)
	synthesizeComp.PodSpec.Tolerations = comp.Spec.Tolerations
	return nil
}

func buildVolumeClaimTemplates(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if comp.Spec.VolumeClaimTemplates != nil {
		synthesizeComp.VolumeClaimTemplates = toVolumeClaimTemplates(&comp.Spec)
	}
}

// limitSharedMemoryVolumeSize limits the shared memory volume size to memory requests/limits.
func limitSharedMemoryVolumeSize(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	shm := defaultShmQuantity
	if comp.Spec.Resources.Limits != nil {
		if comp.Spec.Resources.Limits.Memory().Cmp(shm) > 0 {
			shm = *comp.Spec.Resources.Limits.Memory()
		}
	}
	if comp.Spec.Resources.Requests != nil {
		if comp.Spec.Resources.Requests.Memory().Cmp(shm) > 0 {
			shm = *comp.Spec.Resources.Requests.Memory()
		}
	}
	for i, vol := range synthesizeComp.PodSpec.Volumes {
		if vol.EmptyDir == nil {
			continue
		}
		if vol.EmptyDir.Medium != corev1.StorageMediumMemory {
			continue
		}
		if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
			continue
		}
		synthesizeComp.PodSpec.Volumes[i].EmptyDir.SizeLimit = &shm
	}
}

func toVolumeClaimTemplates(compSpec *appsv1alpha1.ComponentSpec) []corev1.PersistentVolumeClaimTemplate {
	var ts []corev1.PersistentVolumeClaimTemplate
	for _, t := range compSpec.VolumeClaimTemplates {
		ts = append(ts, corev1.PersistentVolumeClaimTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.Name,
			},
			Spec: t.Spec.ToV1PersistentVolumeClaimSpec(),
		})
	}
	return ts
}

// buildResources builds and updates podSpec resources for component.
// TODO(xingran): remove the dependency of SynthesizedComponent.ClusterDefName and SynthesizedComponent.ClusterCompDefName in the future
func buildAndUpdateResources(reqCtx intctrlutil.RequestCtx, cli client.Reader, synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	if comp.Spec.Resources.Requests != nil || comp.Spec.Resources.Limits != nil {
		synthesizeComp.PodSpec.Containers[0].Resources = comp.Spec.Resources
	}
	clsMgr, err := getClassManager(reqCtx.Ctx, cli, synthesizeComp)
	if err != nil {
		return err
	}
	if err := updateResources(synthesizeComp, comp, clsMgr); err != nil {
		reqCtx.Log.Error(err, "update class resources failed")
		return err
	}
	return nil
}

// buildServiceReferences builds serviceReferences for component.
func buildServiceReferences(reqCtx intctrlutil.RequestCtx, cli client.Reader,
	synthesizeComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component) error {
	serviceReferences, err := GenServiceReferences(reqCtx, cli, synthesizeComp.Namespace, synthesizeComp.ClusterName, compDef, comp)
	if err != nil {
		return err
	}
	synthesizeComp.ServiceReferences = serviceReferences

	return resolveServiceReferences(reqCtx.Ctx, cli, synthesizeComp)
}

// buildComponentRef builds componentServices for component.
func buildComponentServices(synthesizeComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition) {
	if len(compDef.Spec.Services) > 0 {
		synthesizeComp.ComponentServices = compDef.Spec.Services
	}
}

// buildServiceAccountName builds serviceAccountName for component and podSpec.
func buildServiceAccountName(synthesizeComp *SynthesizedComponent) {
	// lorry container requires a service account with adequate privileges.
	// If lorry required and the serviceAccountName is not set, a default serviceAccountName will be assigned.
	if synthesizeComp.ServiceAccountName != "" {
		synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
		return
	}
	if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.RoleProbe == nil {
		return
	}
	synthesizeComp.ServiceAccountName = constant.GenerateDefaultServiceAccountName(synthesizeComp.ClusterName)
	// set component.PodSpec.ServiceAccountName
	synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
}

// buildBackwardCompatibleFields builds backward compatible fields for component which referenced a clusterComponentDefinition and clusterComponentVersion before KubeBlocks Version 0.7.0
// TODO(xingran): it will be removed in the future
func buildBackwardCompatibleFields(reqCtx intctrlutil.RequestCtx,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	synthesizeComp *SynthesizedComponent) error {
	if clusterCompSpec.ComponentDefRef == "" {
		return nil // no need to build backward compatible fields
	}

	clusterCompDef := clusterDef.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
	if clusterCompDef == nil {
		return fmt.Errorf("referenced cluster component definition does not exist, cluster: %s, component: %s, component definition ref:%s",
			cluster.Name, clusterCompSpec.Name, clusterCompSpec.ComponentDefRef)
	}

	buildWorkload := func() {
		synthesizeComp.ClusterDefName = clusterDef.Name
		synthesizeComp.ClusterCompDefName = clusterCompDef.Name
		synthesizeComp.WorkloadType = clusterCompDef.WorkloadType
		synthesizeComp.CharacterType = clusterCompDef.CharacterType
		synthesizeComp.HorizontalScalePolicy = clusterCompDef.HorizontalScalePolicy
		synthesizeComp.Probes = clusterCompDef.Probes
		synthesizeComp.VolumeTypes = clusterCompDef.VolumeTypes
		synthesizeComp.VolumeProtection = clusterCompDef.VolumeProtectionSpec
		synthesizeComp.MinAvailable = clusterCompSpec.GetMinAvailable(clusterCompDef.GetMinAvailable())
		// TODO(xingran): this is to ensure compatibility with instances prior to version 0.8.0.
		// The old implementation relies on ClusterCompDef for environmental variables and sets them in labels on the rsm and pod.
		// All places relying on the `app.kubernetes.io/component` label need to be refactored.
		if synthesizeComp.CompDefName == "" {
			synthesizeComp.CompDefName = clusterCompDef.Name
		}
		// TLS is a backward compatible field, which is used in configuration rendering before version 0.8.0.
		if synthesizeComp.TLSConfig != nil {
			synthesizeComp.TLS = true
		}
	}

	// Services is a backward compatible field, which will be replaced with ComponentServices in the future.
	buildServices := func() {
		if clusterCompDef.Service != nil {
			service := corev1.Service{Spec: clusterCompDef.Service.ToSVCSpec()}
			service.Spec.Type = corev1.ServiceTypeClusterIP
			synthesizeComp.Services = append(synthesizeComp.Services, service)
			for _, item := range clusterCompSpec.Services {
				service = corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        item.Name,
						Annotations: item.Annotations,
					},
					Spec: service.Spec,
				}
				service.Spec.Type = item.ServiceType
				synthesizeComp.Services = append(synthesizeComp.Services, service)
			}
		}
	}

	mergeClusterCompVersion := func() {
		var clusterCompVer *appsv1alpha1.ClusterComponentVersion
		if clusterVer != nil {
			clusterCompVer = clusterVer.Spec.GetDefNameMappingComponents()[clusterCompSpec.ComponentDefRef]
		}
		if clusterCompVer != nil {
			// only accept 1st ClusterVersion override context
			synthesizeComp.ConfigTemplates = cfgcore.MergeConfigTemplates(clusterCompVer.ConfigSpecs, synthesizeComp.ConfigTemplates)
			// override component.PodSpec.InitContainers and component.PodSpec.Containers
			for _, c := range clusterCompVer.VersionsCtx.InitContainers {
				synthesizeComp.PodSpec.InitContainers = appendOrOverrideContainerAttr(synthesizeComp.PodSpec.InitContainers, c)
			}
			for _, c := range clusterCompVer.VersionsCtx.Containers {
				synthesizeComp.PodSpec.Containers = appendOrOverrideContainerAttr(synthesizeComp.PodSpec.Containers, c)
			}
		}
	}

	// build workload
	buildWorkload()

	// merge clusterCompVersion
	mergeClusterCompVersion()

	// build services
	buildServices()

	// build componentRefEnvs
	if err := buildComponentRef(clusterDef, cluster, clusterCompDef, synthesizeComp); err != nil {
		reqCtx.Log.Error(err, "failed to merge componentRef")
		return err
	}

	return nil
}

// appendOrOverrideContainerAttr appends targetContainer to compContainers or overrides the attributes of compContainers with a given targetContainer,
// if targetContainer does not exist in compContainers, it will be appended. otherwise it will be updated with the attributes of the target container.
func appendOrOverrideContainerAttr(compContainers []corev1.Container, targetContainer corev1.Container) []corev1.Container {
	index, compContainer := intctrlutil.GetContainerByName(compContainers, targetContainer.Name)
	if compContainer == nil {
		compContainers = append(compContainers, targetContainer)
	} else {
		doContainerAttrOverride(&compContainers[index], targetContainer)
	}
	return compContainers
}

// doContainerAttrOverride overrides the attributes in compContainer with the attributes in container.
func doContainerAttrOverride(compContainer *corev1.Container, container corev1.Container) {
	if compContainer == nil {
		return
	}
	if container.Image != "" {
		compContainer.Image = container.Image
	}
	if len(container.Command) != 0 {
		compContainer.Command = container.Command
	}
	if len(container.Args) != 0 {
		compContainer.Args = container.Args
	}
	if container.WorkingDir != "" {
		compContainer.WorkingDir = container.WorkingDir
	}
	if len(container.Ports) != 0 {
		compContainer.Ports = container.Ports
	}
	if len(container.EnvFrom) != 0 {
		compContainer.EnvFrom = container.EnvFrom
	}
	if len(container.Env) != 0 {
		compContainer.Env = container.Env
	}
	if container.Resources.Limits != nil || container.Resources.Requests != nil {
		compContainer.Resources = container.Resources
	}
	if len(container.VolumeMounts) != 0 {
		compContainer.VolumeMounts = container.VolumeMounts
	}
	if len(container.VolumeDevices) != 0 {
		compContainer.VolumeDevices = container.VolumeDevices
	}
	if container.LivenessProbe != nil {
		compContainer.LivenessProbe = container.LivenessProbe
	}
	if container.ReadinessProbe != nil {
		compContainer.ReadinessProbe = container.ReadinessProbe
	}
	if container.StartupProbe != nil {
		compContainer.StartupProbe = container.StartupProbe
	}
	if container.Lifecycle != nil {
		compContainer.Lifecycle = container.Lifecycle
	}
	if container.TerminationMessagePath != "" {
		compContainer.TerminationMessagePath = container.TerminationMessagePath
	}
	if container.TerminationMessagePolicy != "" {
		compContainer.TerminationMessagePolicy = container.TerminationMessagePolicy
	}
	if container.ImagePullPolicy != "" {
		compContainer.ImagePullPolicy = container.ImagePullPolicy
	}
	if container.SecurityContext != nil {
		compContainer.SecurityContext = container.SecurityContext
	}
}

// GetEnvReplacementMapForConnCredential gets the replacement map for connect credential
// TODO: deprecated, will be removed later.
func GetEnvReplacementMapForConnCredential(clusterName string) map[string]string {
	return map[string]string{
		constant.KBConnCredentialPlaceHolder: constant.GenerateDefaultConnCredential(clusterName),
	}
}

func replaceContainerPlaceholderTokens(component *SynthesizedComponent, namedValuesMap map[string]string) {
	// replace env[].valueFrom.secretKeyRef.name variables
	for _, cc := range [][]corev1.Container{component.PodSpec.InitContainers, component.PodSpec.Containers} {
		for _, c := range cc {
			c.Env = ReplaceSecretEnvVars(namedValuesMap, c.Env)
		}
	}
}

// GetReplacementMapForBuiltInEnv gets the replacement map for KubeBlocks built-in environment variables.
func GetReplacementMapForBuiltInEnv(clusterName, clusterUID, componentName string) map[string]string {
	cc := constant.GenerateClusterComponentName(clusterName, componentName)
	replacementMap := map[string]string{
		constant.EnvPlaceHolder(constant.KBEnvClusterName):     clusterName,
		constant.EnvPlaceHolder(constant.KBEnvCompName):        componentName,
		constant.EnvPlaceHolder(constant.KBEnvClusterCompName): cc,
		constant.KBComponentEnvCMPlaceHolder:                   constant.GenerateClusterComponentEnvPattern(clusterName, componentName),
	}
	clusterUIDPostfix := clusterUID
	if len(clusterUID) > 8 {
		clusterUIDPostfix = clusterUID[len(clusterUID)-8:]
	}
	replacementMap[constant.EnvPlaceHolder(constant.KBEnvClusterUIDPostfix8Deprecated)] = clusterUIDPostfix
	return replacementMap
}

// ReplaceNamedVars replaces the placeholder in targetVar if it is match and returns the replaced result
func ReplaceNamedVars(namedValuesMap map[string]string, targetVar string, limits int, matchAll bool) string {
	for placeHolderKey, mappingValue := range namedValuesMap {
		r := strings.Replace(targetVar, placeHolderKey, mappingValue, limits)
		// early termination on matching, when matchAll = false
		if r != targetVar && !matchAll {
			return r
		}
		targetVar = r
	}
	return targetVar
}

// ReplaceSecretEnvVars replaces the env secret value with namedValues and returns new envs
func ReplaceSecretEnvVars(namedValuesMap map[string]string, envs []corev1.EnvVar) []corev1.EnvVar {
	newEnvs := make([]corev1.EnvVar, 0, len(envs))
	for _, e := range envs {
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		name := ReplaceNamedVars(namedValuesMap, e.ValueFrom.SecretKeyRef.Name, 1, false)
		if name != e.ValueFrom.SecretKeyRef.Name {
			e.ValueFrom.SecretKeyRef.Name = name
		}
		newEnvs = append(newEnvs, e)
	}
	return newEnvs
}

// overrideSwitchoverSpecAttr overrides the attributes in switchoverSpec with the attributes of SwitchoverShortSpec in clusterVersion.
func overrideSwitchoverSpecAttr(switchoverSpec *appsv1alpha1.SwitchoverSpec, cvSwitchoverSpec *appsv1alpha1.SwitchoverShortSpec) {
	if switchoverSpec == nil || cvSwitchoverSpec == nil || cvSwitchoverSpec.CmdExecutorConfig == nil {
		return
	}
	applyCmdExecutorConfig := func(cmdExecutorConfig *appsv1alpha1.CmdExecutorConfig) {
		if cmdExecutorConfig == nil {
			return
		}
		if len(cvSwitchoverSpec.CmdExecutorConfig.Image) > 0 {
			cmdExecutorConfig.Image = cvSwitchoverSpec.CmdExecutorConfig.Image
		}
		if len(cvSwitchoverSpec.CmdExecutorConfig.Env) > 0 {
			cmdExecutorConfig.Env = cvSwitchoverSpec.CmdExecutorConfig.Env
		}
	}
	if switchoverSpec.WithCandidate != nil {
		applyCmdExecutorConfig(switchoverSpec.WithCandidate.CmdExecutorConfig)
	}
	if switchoverSpec.WithoutCandidate != nil {
		applyCmdExecutorConfig(switchoverSpec.WithoutCandidate.CmdExecutorConfig)
	}
}

func GetConfigSpecByName(synthesizedComp *SynthesizedComponent, configSpec string) *appsv1alpha1.ComponentConfigSpec {
	for i := range synthesizedComp.ConfigTemplates {
		template := &synthesizedComp.ConfigTemplates[i]
		if template.Name == configSpec {
			return template
		}
	}
	return nil
}

// getClassManager gets the class manager for build resource constraint.
// TODO(xingran): remove the dependency of SynthesizedComponent.ClusterDefName and SynthesizedComponent.ClusterCompDefName in the future
func getClassManager(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent) (*Manager, error) {
	var (
		classDefinitionList appsv1alpha1.ComponentClassDefinitionList
		ml                  []client.ListOption
	)
	if synthesizedComp.ClusterDefName != "" {
		ml = []client.ListOption{
			client.MatchingLabels{constant.ClusterDefLabelKey: synthesizedComp.ClusterDefName},
		}
	} else {
		ml = []client.ListOption{
			client.MatchingLabels{constant.ComponentDefinitionLabelKey: synthesizedComp.CompDefName},
		}
	}

	if err := cli.List(ctx, &classDefinitionList, ml...); err != nil {
		return nil, err
	}

	var constraintList appsv1alpha1.ComponentResourceConstraintList
	if err := cli.List(ctx, &constraintList); err != nil {
		return nil, err
	}
	return NewManager(classDefinitionList, constraintList)
}

// updateResources updates the resources of pod spec with the expected resources from class manager.
// TODO(xingran): remove the dependency of SynthesizedComponent.ClusterDefName and SynthesizedComponent.ClusterCompDefName in the future
func updateResources(synthesizedComp *SynthesizedComponent, comp *appsv1alpha1.Component, clsMgr *Manager) error {
	var (
		expectResources corev1.ResourceList
		err             error
	)

	if ignoreResourceConstraint(comp) {
		return nil
	}

	if clsMgr == nil {
		return nil
	}

	expectResources, err = clsMgr.GetResources(synthesizedComp, comp)
	if err != nil || expectResources == nil {
		return err
	}

	actualResources := synthesizedComp.PodSpec.Containers[0].Resources
	if actualResources.Requests == nil {
		actualResources.Requests = corev1.ResourceList{}
	}
	if actualResources.Limits == nil {
		actualResources.Limits = corev1.ResourceList{}
	}
	for k, v := range expectResources {
		actualResources.Requests[k] = v
		actualResources.Limits[k] = v
	}
	synthesizedComp.PodSpec.Containers[0].Resources = actualResources
	return nil
}
