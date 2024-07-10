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
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
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
	clusterDef, clusterVer, err := getClusterReferencedResources(reqCtx.Ctx, cli, cluster)
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
	compDef, err := getOrBuildComponentDefinition(reqCtx.Ctx, cli, clusterDef, clusterVer, cluster, clusterCompSpec)
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
	comp, err := BuildComponent(cluster, clusterCompSpec, nil, nil)
	if err != nil {
		return nil, err
	}
	return buildSynthesizedComponent(reqCtx, cli, compDef, comp, clusterDef, cluster, clusterCompSpec)
}

// buildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from ComponentDefinition and Component.
// !!! Do not use @clusterDef, @clusterVer, @cluster and @clusterCompSpec since they are used for the backward compatibility only.
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
		Namespace:              comp.Namespace,
		ClusterName:            clusterName,
		ClusterUID:             clusterUID,
		Comp2CompDefs:          comp2CompDef,
		Name:                   compName,
		FullCompName:           comp.Name,
		CompDefName:            compDef.Name,
		ServiceVersion:         comp.Spec.ServiceVersion,
		ClusterGeneration:      clusterGeneration(cluster, comp),
		UserDefinedLabels:      comp.Spec.Labels,
		UserDefinedAnnotations: comp.Spec.Annotations,
		PodSpec:                &compDef.Spec.Runtime,
		HostNetwork:            compDefObj.Spec.HostNetwork,
		ComponentServices:      compDefObj.Spec.Services,
		LogConfigs:             compDefObj.Spec.LogConfigs,
		ConfigTemplates:        compDefObj.Spec.Configs,
		ScriptTemplates:        compDefObj.Spec.Scripts,
		Roles:                  compDefObj.Spec.Roles,
		UpdateStrategy:         compDefObj.Spec.UpdateStrategy,
		MinReadySeconds:        compDefObj.Spec.MinReadySeconds,
		PolicyRules:            compDefObj.Spec.PolicyRules,
		LifecycleActions:       compDefObj.Spec.LifecycleActions,
		SystemAccounts:         mergeSystemAccounts(compDefObj.Spec.SystemAccounts, comp.Spec.SystemAccounts),
		Replicas:               comp.Spec.Replicas,
		Resources:              comp.Spec.Resources,
		TLSConfig:              comp.Spec.TLSConfig,
		ServiceAccountName:     comp.Spec.ServiceAccountName,
		Instances:              comp.Spec.Instances,
		OfflineInstances:       comp.Spec.OfflineInstances,
		DisableExporter:        comp.Spec.DisableExporter,
		PodManagementPolicy:    compDef.Spec.PodManagementPolicy,
	}

	// build backward compatible fields, including workload, services, componentRefEnvs, clusterDefName, clusterCompDefName, and clusterCompVer, etc.
	// if cluster referenced a clusterDefinition and clusterVersion, for backward compatibility, we need to merge the clusterDefinition and clusterVersion into the component
	// TODO(xingran): it will be removed in the future
	if clusterDef != nil && cluster != nil && clusterCompSpec != nil {
		if err = buildBackwardCompatibleFields(reqCtx, clusterDef, cluster, clusterCompSpec, synthesizeComp); err != nil {
			return nil, err
		}
	}
	if synthesizeComp.HorizontalScalePolicy == nil {
		buildCompatibleHorizontalScalePolicy(compDefObj, synthesizeComp)
	}

	if err = mergeUserDefinedEnv(synthesizeComp, comp); err != nil {
		return nil, err
	}

	// build scheduling policy for workload
	if err = buildSchedulingPolicy(synthesizeComp, comp); err != nil {
		reqCtx.Log.Error(err, "failed to build scheduling policy")
		return nil, err
	}

	// update resources
	buildAndUpdateResources(synthesizeComp, comp)

	// build labels and annotations
	buildLabelsAndAnnotations(compDef, comp, synthesizeComp)

	// build volumes & volumeClaimTemplates
	buildVolumeClaimTemplates(synthesizeComp, comp)
	if err = mergeUserDefinedVolumes(synthesizeComp, comp); err != nil {
		return nil, err
	}

	limitSharedMemoryVolumeSize(synthesizeComp, comp)

	// build componentService
	buildComponentServices(synthesizeComp, comp)

	if err = overrideConfigTemplates(synthesizeComp, comp); err != nil {
		return nil, err
	}

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
		shardingComps, err := intctrlutil.ListShardingComponents(ctx, cli, cluster, shardingSpec.Name)
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

func mergeUserDefinedEnv(synthesizedComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	if comp == nil || len(comp.Spec.Env) == 0 {
		return nil
	}

	vars := sets.New[string]()
	for _, v := range comp.Spec.Env {
		if vars.Has(v.Name) {
			return fmt.Errorf("duplicated user-defined env var %s", v.Name)
		}
		vars.Insert(v.Name)
	}

	for i := range synthesizedComp.PodSpec.InitContainers {
		synthesizedComp.PodSpec.InitContainers[i].Env = append(synthesizedComp.PodSpec.InitContainers[i].Env, comp.Spec.Env...)
	}
	for i := range synthesizedComp.PodSpec.Containers {
		synthesizedComp.PodSpec.Containers[i].Env = append(synthesizedComp.PodSpec.Containers[i].Env, comp.Spec.Env...)
	}
	return nil
}

func mergeSystemAccounts(compDefAccounts []appsv1alpha1.SystemAccount,
	compAccounts []appsv1alpha1.ComponentSystemAccount) []appsv1alpha1.SystemAccount {
	if len(compAccounts) == 0 {
		return compDefAccounts
	}

	override := func(compAccount appsv1alpha1.ComponentSystemAccount, idx int) {
		if compAccount.PasswordConfig != nil {
			compDefAccounts[idx].PasswordGenerationPolicy = *compAccount.PasswordConfig
		}
		compDefAccounts[idx].SecretRef = compAccount.SecretRef
	}

	tbl := make(map[string]int)
	for i, account := range compDefAccounts {
		tbl[account.Name] = i
	}

	for _, account := range compAccounts {
		idx, ok := tbl[account.Name]
		if !ok {
			continue // ignore it silently
		}
		override(account, idx)
	}

	return compDefAccounts
}

func buildSchedulingPolicy(synthesizedComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	var (
		schedulingPolicy = comp.Spec.SchedulingPolicy
		err              error
	)
	if schedulingPolicy == nil {
		// for compatibility, we need to build scheduling policy from component's affinity and tolerations
		schedulingPolicy, err = scheduling.BuildSchedulingPolicy4Component(synthesizedComp.ClusterName,
			synthesizedComp.Name, comp.Spec.Affinity, comp.Spec.Tolerations)
		if err != nil {
			return err
		}
	}
	synthesizedComp.PodSpec.SchedulerName = schedulingPolicy.SchedulerName
	synthesizedComp.PodSpec.NodeSelector = schedulingPolicy.NodeSelector
	synthesizedComp.PodSpec.NodeName = schedulingPolicy.NodeName
	synthesizedComp.PodSpec.Affinity = schedulingPolicy.Affinity
	synthesizedComp.PodSpec.Tolerations = schedulingPolicy.Tolerations
	synthesizedComp.PodSpec.TopologySpreadConstraints = schedulingPolicy.TopologySpreadConstraints
	return nil
}

func buildVolumeClaimTemplates(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if comp.Spec.VolumeClaimTemplates != nil {
		synthesizeComp.VolumeClaimTemplates = toVolumeClaimTemplates(&comp.Spec)
	}
}

func mergeUserDefinedVolumes(synthesizedComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	if comp == nil {
		return nil
	}
	volumes := map[string]bool{}
	for _, vols := range [][]corev1.Volume{synthesizedComp.PodSpec.Volumes, comp.Spec.Volumes} {
		for _, vol := range vols {
			if volumes[vol.Name] {
				return fmt.Errorf("duplicated volume %s", vol.Name)
			}
			volumes[vol.Name] = true
		}
	}
	for _, vct := range synthesizedComp.VolumeClaimTemplates {
		if volumes[vct.Name] {
			return fmt.Errorf("duplicated volume %s", vct.Name)
		}
		volumes[vct.Name] = true
	}

	checkConfigNScriptTemplate := func(tpl appsv1alpha1.ComponentTemplateSpec) error {
		if volumes[tpl.VolumeName] {
			return fmt.Errorf("duplicated volume %s for template %s", tpl.VolumeName, tpl.Name)
		}
		volumes[tpl.VolumeName] = true
		return nil
	}
	for _, tpl := range synthesizedComp.ConfigTemplates {
		if err := checkConfigNScriptTemplate(tpl.ComponentTemplateSpec); err != nil {
			return err
		}
	}
	for _, tpl := range synthesizedComp.ScriptTemplates {
		if err := checkConfigNScriptTemplate(tpl); err != nil {
			return err
		}
	}

	// for _, cc := range [][]corev1.Container{synthesizedComp.PodSpec.InitContainers, synthesizedComp.PodSpec.Containers} {
	//	for _, c := range cc {
	//		missed := make([]string, 0)
	//		for _, mount := range c.VolumeMounts {
	//			if !volumes[mount.Name] {
	//				missed = append(missed, mount.Name)
	//			}
	//		}
	//		if len(missed) > 0 {
	//			return fmt.Errorf("volumes should be provided for mounts %s", strings.Join(missed, ","))
	//		}
	//	}
	// }
	synthesizedComp.PodSpec.Volumes = append(synthesizedComp.PodSpec.Volumes, comp.Spec.Volumes...)
	return nil
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

func buildComponentServices(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if len(synthesizeComp.ComponentServices) == 0 || len(comp.Spec.Services) == 0 {
		return
	}

	services := map[string]appsv1alpha1.ComponentService{}
	for i, svc := range comp.Spec.Services {
		services[svc.Name] = comp.Spec.Services[i]
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

func overrideConfigTemplates(synthesizedComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	if comp == nil || len(comp.Spec.Configs) == 0 {
		return nil
	}

	templates := make(map[string]*appsv1alpha1.ComponentConfigSpec)
	for i, template := range synthesizedComp.ConfigTemplates {
		templates[template.Name] = &synthesizedComp.ConfigTemplates[i]
	}

	for _, config := range comp.Spec.Configs {
		if config.Name == nil || len(*config.Name) == 0 {
			continue // not supported now
		}
		template := templates[*config.Name]
		if template == nil {
			return fmt.Errorf("the config template %s is not defined in definition", *config.Name)
		}

		specified := func() bool {
			return config.ConfigMap != nil && len(config.ConfigMap.Name) > 0
		}
		switch {
		case len(template.TemplateRef) == 0 && !specified():
			return fmt.Errorf("there is no content provided for config template %s", *config.Name)
		case len(template.TemplateRef) > 0 && specified():
			return fmt.Errorf("partial overriding is not supported, config template: %s", *config.Name)
		case specified():
			template.TemplateRef = config.ConfigMap.Name
		default:
			// do nothing
		}
	}
	return nil
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

func buildRuntimeClassName(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if comp.Spec.RuntimeClassName == nil {
		return
	}
	synthesizeComp.PodSpec.RuntimeClassName = comp.Spec.RuntimeClassName
}

// buildBackwardCompatibleFields builds backward compatible fields for component which referenced a clusterComponentDefinition and clusterComponentVersion
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
		synthesizeComp.CharacterType = clusterCompDef.CharacterType
		synthesizeComp.HorizontalScalePolicy = clusterCompDef.HorizontalScalePolicy
		synthesizeComp.VolumeTypes = clusterCompDef.VolumeTypes
	}

	buildClusterCompServices := func() {
		if len(synthesizeComp.ComponentServices) > 0 {
			service := corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: synthesizeComp.ComponentServices[0].Spec.Ports,
				},
			}
			for _, item := range clusterCompSpec.Services {
				svc := appsv1alpha1.ComponentService{
					Service: appsv1alpha1.Service{
						Name:        item.Name,
						ServiceName: item.Name,
						Annotations: item.Annotations,
						Spec:        *service.Spec.DeepCopy(),
					},
				}
				svc.Spec.Type = item.ServiceType
				synthesizeComp.ComponentServices = append(synthesizeComp.ComponentServices, svc)
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

	buildClusterCompServices()

	// build pod management policy
	buildPodManagementPolicy()

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
