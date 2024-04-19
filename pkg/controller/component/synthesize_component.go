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

package component

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
	return buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, cluster, nil)
}

// BuildSynthesizedComponent4Generated builds SynthesizedComponent for generated Component which w/o ComponentDefinition.
func BuildSynthesizedComponent4Generated(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component) (*appsv1alpha1.ComponentDefinition, *SynthesizedComponent, error) {
	clusterDef, err := getClusterReferencedResources(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return nil, nil, err
	}
	clusterCompSpec, err := getClusterCompSpec4Component(reqCtx.Ctx, cli, clusterDef, cluster, comp)
	if err != nil {
		return nil, nil, err
	}
	if clusterCompSpec == nil {
		return nil, nil, fmt.Errorf("cluster component spec is not found: %s", comp.Name)
	}
	compDef, err := getOrBuildComponentDefinition(reqCtx.Ctx, cli, clusterDef, cluster, clusterCompSpec)
	if err != nil {
		return nil, nil, err
	}
	synthesizedComp, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, clusterDef, cluster, clusterCompSpec)
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
	clusterDef, err := getClusterReferencedResources(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	return BuildSynthesizedComponentWrapper4Test(reqCtx, cli, clusterDef, cluster, clusterCompSpec)
}

// BuildSynthesizedComponentWrapper4Test builds a new SynthesizedComponent object with a given ClusterComponentSpec.
func BuildSynthesizedComponentWrapper4Test(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*SynthesizedComponent, error) {
	if clusterCompSpec == nil {
		clusterCompSpec = apiconversion.HandleSimplifiedClusterAPI(clusterDef, cluster)
	}
	if clusterCompSpec == nil {
		return nil, nil
	}
	compDef, err := getOrBuildComponentDefinition(reqCtx.Ctx, cli, clusterDef, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	comp, err := BuildComponent(cluster, clusterCompSpec, nil, nil)
	if err != nil {
		return nil, err
	}
	return buildSynthesizedComponent(reqCtx, cli, compDef, comp, clusterDef, cluster, clusterCompSpec)
}

// buildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from ComponentDefinition and Component.
// !!! Do not use @clusterDef, @cluster and @clusterCompSpec since they are used for the backward compatibility only.
// TODO: remove @reqCtx & @cli
func buildSynthesizedComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component,
	clusterDef *appsv1alpha1.ClusterDefinition,
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
	comp2CompDef, err := buildComp2CompDefs(reqCtx.Ctx, cli, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	compDefObj := compDef.DeepCopy()
	synthesizeComp := &SynthesizedComponent{
		Namespace:          comp.Namespace,
		ClusterName:        clusterName,
		ClusterUID:         clusterUID,
		Comp2CompDefs:      comp2CompDef,
		Name:               compName,
		FullCompName:       comp.Name,
		CompDefName:        compDef.Name,
		ServiceVersion:     comp.Spec.ServiceVersion,
		ClusterGeneration:  clusterGeneration(cluster, comp),
		PodSpec:            &compDef.Spec.Runtime,
		HostNetwork:        compDefObj.Spec.HostNetwork,
		LogConfigs:         compDefObj.Spec.LogConfigs,
		ConfigTemplates:    compDefObj.Spec.Configs,
		ScriptTemplates:    compDefObj.Spec.Scripts,
		Roles:              compDefObj.Spec.Roles,
		UpdateStrategy:     compDefObj.Spec.UpdateStrategy,
		MinReadySeconds:    compDefObj.Spec.MinReadySeconds,
		PolicyRules:        compDefObj.Spec.PolicyRules,
		LifecycleActions:   compDefObj.Spec.LifecycleActions,
		SystemAccounts:     compDefObj.Spec.SystemAccounts,
		RoleArbitrator:     compDefObj.Spec.RoleArbitrator,
		Replicas:           comp.Spec.Replicas,
		Resources:          comp.Spec.Resources,
		TLSConfig:          comp.Spec.TLSConfig,
		ServiceAccountName: comp.Spec.ServiceAccountName,
		Instances:          comp.Spec.Instances,
		OfflineInstances:   comp.Spec.OfflineInstances,
	}

	// build backward compatible fields, including workload, services, componentRefEnvs, clusterDefName, clusterCompDefName, etc.
	// if cluster referenced a clusterDefinition, for backward compatibility, we need to merge the clusterDefinition into the component
	// TODO(xingran): it will be removed in the future
	if clusterDef != nil && cluster != nil && clusterCompSpec != nil {
		if err = buildBackwardCompatibleFields(reqCtx, clusterDef, cluster, clusterCompSpec, synthesizeComp); err != nil {
			return nil, err
		}
	}
	if synthesizeComp.HorizontalScalePolicy == nil {
		buildCompatibleHorizontalScalePolicy(compDefObj, synthesizeComp)
	}

	// build affinity and tolerations
	if err := buildAffinitiesAndTolerations(comp, synthesizeComp); err != nil {
		reqCtx.Log.Error(err, "build affinities and tolerations failed.")
		return nil, err
	}

	// update resources
	buildAndUpdateResources(synthesizeComp, comp)

	// build labels and annotations
	buildLabelsAndAnnotations(compDef, comp, synthesizeComp)

	// build volumeClaimTemplates
	buildVolumeClaimTemplates(synthesizeComp, comp)

	limitSharedMemoryVolumeSize(synthesizeComp, comp)

	// build componentService
	buildComponentServices(synthesizeComp, compDefObj, comp)

	// build monitor
	buildMonitorConfig(compDefObj.Spec.Monitor, comp.Spec.Monitor, &compDefObj.Spec.Runtime, synthesizeComp)

	// build serviceAccountName
	buildServiceAccountName(synthesizeComp)

	// build runtimeClassName
	buildRuntimeClassName(synthesizeComp, comp)

	// build lorryContainer
	// TODO(xingran): buildLorryContainers relies on synthesizeComp.CharacterType and synthesizeComp.WorkloadType, which will be deprecated in the future.
	if err := buildLorryContainers(reqCtx, synthesizeComp, clusterCompSpec); err != nil {
		reqCtx.Log.Error(err, "build lorry containers failed.")
		return nil, err
	}

	if err = buildServiceReferences(reqCtx.Ctx, cli, synthesizeComp, compDef, comp); err != nil {
		reqCtx.Log.Error(err, "build service references failed.")
		return nil, err
	}

	// replace podSpec containers env default credential placeholder
	replaceContainerPlaceholderTokens(synthesizeComp, GetEnvReplacementMapForConnCredential(synthesizeComp.ClusterName))

	return synthesizeComp, nil
}

func buildRuntimeClassName(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if comp.Spec.RuntimeClassName == nil {
		return
	}
	synthesizeComp.PodSpec.RuntimeClassName = comp.Spec.RuntimeClassName
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

func buildComp2CompDefs(ctx context.Context, cli client.Reader, cluster *appsv1alpha1.Cluster, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (map[string]string, error) {
	if cluster == nil {
		return nil, nil
	}
	mapping := make(map[string]string)

	// Build from ComponentSpecs
	if len(cluster.Spec.ComponentSpecs) == 0 {
		if clusterCompSpec != nil && len(clusterCompSpec.ComponentDef) > 0 {
			mapping[clusterCompSpec.Name] = clusterCompSpec.ComponentDef
		}
	} else {
		for _, comp := range cluster.Spec.ComponentSpecs {
			if len(comp.ComponentDef) > 0 {
				mapping[comp.Name] = comp.ComponentDef
			}
		}
	}

	// Build from ShardingSpecs
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		shardingComps, err := intctrlutil.ListShardingComponents(ctx, cli, cluster, &shardingSpec)
		if err != nil {
			return nil, err
		}
		for _, shardingComp := range shardingComps {
			if len(shardingComp.Spec.CompDef) > 0 {
				compShortName, err := ShortName(cluster.Name, shardingComp.Name)
				if err != nil {
					return nil, err
				}
				mapping[compShortName] = shardingComp.Spec.CompDef
			}
		}
	}

	return mapping, nil
}

// buildLabelsAndAnnotations builds labels and annotations for synthesizedComponent.
func buildLabelsAndAnnotations(compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component, synthesizeComp *SynthesizedComponent) {
	replaceEnvPlaceholderTokens := func(clusterName, uid, componentName string, kvMap map[string]string) map[string]string {
		replacedMap := make(map[string]string, len(kvMap))
		builtInEnvMap := GetReplacementMapForBuiltInEnv(clusterName, uid, componentName)
		for k, v := range kvMap {
			replacedMap[ReplaceNamedVars(builtInEnvMap, k, -1, true)] = ReplaceNamedVars(builtInEnvMap, v, -1, true)
		}
		return replacedMap
	}

	mergeMaps := func(baseMap, overrideMap map[string]string) map[string]string {
		for k, v := range overrideMap {
			baseMap[k] = v
		}
		return baseMap
	}

	if compDef.Spec.Labels != nil || comp.Labels != nil {
		baseLabels := make(map[string]string)
		if compDef.Spec.Labels != nil {
			baseLabels = replaceEnvPlaceholderTokens(synthesizeComp.ClusterName, synthesizeComp.ClusterUID, synthesizeComp.Name, compDef.Spec.Labels)
		}
		// override labels from component
		synthesizeComp.Labels = mergeMaps(baseLabels, comp.Labels)
	}

	if compDef.Spec.Annotations != nil || comp.Annotations != nil {
		baseAnnotations := make(map[string]string)
		if compDef.Spec.Annotations != nil {
			baseAnnotations = replaceEnvPlaceholderTokens(synthesizeComp.ClusterName, synthesizeComp.ClusterUID, synthesizeComp.Name, compDef.Spec.Annotations)
		}
		// override annotations from component
		synthesizeComp.Annotations = mergeMaps(baseAnnotations, comp.Annotations)
	}
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

// buildAndUpdateResources updates podSpec resources from component
func buildAndUpdateResources(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if comp.Spec.Resources.Requests != nil || comp.Spec.Resources.Limits != nil {
		synthesizeComp.PodSpec.Containers[0].Resources = comp.Spec.Resources
	}
}

func buildComponentServices(synthesizeComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component) {
	services := map[string]appsv1alpha1.ComponentService{}
	for i, svc := range comp.Spec.Services {
		services[svc.Name] = comp.Spec.Services[i]
	}

	synthesizeComp.ComponentServices = compDef.Spec.Services
	if len(synthesizeComp.ComponentServices) == 0 || len(services) == 0 {
		return
	}

	override := func(svc *appsv1alpha1.ComponentService) {
		svc1, ok := services[svc.Name]
		if ok {
			svc.Spec.Type = svc1.Spec.Type
			svc.Annotations = svc1.Annotations
			svc.PodService = svc1.PodService
			if svc.DisableAutoProvision != nil {
				svc.DisableAutoProvision = func() *bool { b := false; return &b }()
			}
		}
	}
	for i := range synthesizeComp.ComponentServices {
		override(&synthesizeComp.ComponentServices[i])
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

	buildPodManagementPolicy := func() {
		var podManagementPolicy appsv1.PodManagementPolicyType
		w := clusterCompDef.GetStatefulSetWorkload()
		if w == nil {
			podManagementPolicy = ""
		} else {
			podManagementPolicy, _ = w.FinalStsUpdateStrategy()
		}
		synthesizeComp.PodManagementPolicy = &podManagementPolicy
	}

	// build workload
	buildWorkload()

	// build services
	buildServices()

	// build pod management policy
	buildPodManagementPolicy()

	// build componentRefEnvs
	if err := buildComponentRef(clusterDef, cluster, clusterCompDef, synthesizeComp); err != nil {
		reqCtx.Log.Error(err, "failed to merge componentRef")
		return err
	}

	return nil
}

func buildCompatibleHorizontalScalePolicy(compDef *appsv1alpha1.ComponentDefinition, synthesizeComp *SynthesizedComponent) {
	if compDef.Annotations != nil {
		if templateName, ok := compDef.Annotations[constant.HorizontalScaleBackupPolicyTemplateKey]; ok {
			synthesizeComp.HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
				Type:                     appsv1alpha1.HScaleDataClonePolicyCloneVolume,
				BackupPolicyTemplateName: templateName,
			}
		}
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

func GetConfigSpecByName(synthesizedComp *SynthesizedComponent, configSpec string) *appsv1alpha1.ComponentConfigSpec {
	for i := range synthesizedComp.ConfigTemplates {
		template := &synthesizedComp.ConfigTemplates[i]
		if template.Name == configSpec {
			return template
		}
	}
	return nil
}
