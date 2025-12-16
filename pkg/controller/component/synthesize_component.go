/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	defaultShmQuantity        = resource.MustParse("64Mi")
	defaultPVCRetentionPolicy = appsv1.PersistentVolumeClaimRetentionPolicy{
		WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
		WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
	}
)

// BuildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from ComponentDefinition and Component.
// TODO: remove @ctx & @cli
func BuildSynthesizedComponent(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, comp *appsv1.Component) (*SynthesizedComponent, error) {
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
	comp2CompDef, err := BuildComp2CompDefs(ctx, cli, comp.Namespace, clusterName)
	if err != nil {
		return nil, err
	}
	compDef2CompCnt, err := buildCompDef2CompCount(ctx, cli, comp.Namespace, clusterName)
	if err != nil {
		return nil, err
	}
	compDefObj := compDef.DeepCopy()
	synthesizeComp := &SynthesizedComponent{
		Namespace:                        comp.Namespace,
		ClusterName:                      clusterName,
		ClusterUID:                       clusterUID,
		Comp2CompDefs:                    comp2CompDef,
		CompDef2CompCnt:                  compDef2CompCnt,
		Name:                             compName,
		FullCompName:                     comp.Name,
		Generation:                       strconv.FormatInt(comp.Generation, 10),
		CompDefName:                      compDef.Name,
		ServiceKind:                      compDefObj.Spec.ServiceKind,
		ServiceVersion:                   comp.Spec.ServiceVersion,
		Labels:                           comp.Labels,
		StaticLabels:                     compDef.Spec.Labels,
		DynamicLabels:                    comp.Spec.Labels,
		Annotations:                      comp.Annotations,
		StaticAnnotations:                compDef.Spec.Annotations,
		DynamicAnnotations:               comp.Spec.Annotations,
		PodSpec:                          &compDef.Spec.Runtime,
		Network:                          comp.Spec.Network,
		HostNetwork:                      compDefObj.Spec.HostNetwork,
		ComponentServices:                compDefObj.Spec.Services,
		LogConfigs:                       compDefObj.Spec.LogConfigs,
		Roles:                            compDefObj.Spec.Roles,
		MinReadySeconds:                  compDefObj.Spec.MinReadySeconds,
		PolicyRules:                      compDefObj.Spec.PolicyRules,
		LifecycleActions:                 compDefObj.Spec.LifecycleActions,
		SystemAccounts:                   compDefObj.Spec.SystemAccounts,
		Replicas:                         comp.Spec.Replicas,
		Resources:                        comp.Spec.Resources,
		TLSConfig:                        comp.Spec.TLSConfig,
		ServiceAccountName:               comp.Spec.ServiceAccountName,
		Instances:                        comp.Spec.Instances,
		Ordinals:                         comp.Spec.Ordinals,
		FlatInstanceOrdinal:              comp.Spec.FlatInstanceOrdinal,
		InstanceImages:                   make(map[string]map[string]string),
		OfflineInstances:                 comp.Spec.OfflineInstances,
		DisableExporter:                  comp.Spec.DisableExporter,
		Stop:                             comp.Spec.Stop,
		PodManagementPolicy:              compDef.Spec.PodManagementPolicy,
		ParallelPodManagementConcurrency: comp.Spec.ParallelPodManagementConcurrency,
		PodUpdatePolicy:                  getPodUpdatePolicy(comp, compDef),
		PodUpgradePolicy:                 getPodUpgradePolicy(comp, compDef),
		UpdateStrategy:                   compDef.Spec.UpdateStrategy,
		InstanceUpdateStrategy:           comp.Spec.InstanceUpdateStrategy,
		EnableInstanceAPI:                comp.Spec.EnableInstanceAPI,
	}

	// build scheduling policy for workload
	scheduling.ApplySchedulingPolicyToPodSpec(synthesizeComp.PodSpec, comp.Spec.SchedulingPolicy)

	buildFileTemplates(synthesizeComp, compDef, comp)
	if err = overrideNCheckConfigTemplates(synthesizeComp, comp); err != nil {
		return nil, err
	}

	// update resources
	buildAndUpdateResources(synthesizeComp, comp)

	// build volumeClaimTemplates & volumes
	buildVolumeClaimTemplates(synthesizeComp, comp)
	if err = mergeUserDefinedVolumes(synthesizeComp, comp); err != nil {
		return nil, err
	}

	mergeNetworkSetting(synthesizeComp, comp)

	limitSharedMemoryVolumeSize(synthesizeComp, comp)

	// override componentService
	overrideComponentServices(synthesizeComp, comp)

	// build serviceAccountName
	buildServiceAccountName(synthesizeComp)

	// build runtimeClassName
	buildRuntimeClassName(synthesizeComp, comp)

	if err = buildSidecars(ctx, cli, synthesizeComp, comp); err != nil {
		return nil, err
	}

	if err = buildKBAgentContainer(synthesizeComp); err != nil {
		return nil, errors.Wrap(err, "build kb-agent container failed")
	}

	// build volume mounts after kb-agent containers
	buildVolumeMounts(synthesizeComp)

	if err = mergeUserDefinedEnv(synthesizeComp, comp); err != nil {
		return nil, err
	}

	if err = buildServiceReferences(ctx, cli, synthesizeComp, compDef, comp); err != nil {
		return nil, errors.Wrap(err, "build service references failed")
	}

	return synthesizeComp, nil
}

func BuildComp2CompDefs(ctx context.Context, cli client.Reader, namespace, clusterName string) (map[string]string, error) {
	if cli == nil {
		return nil, nil // for test
	}

	labels := constant.GetClusterLabels(clusterName)
	comps, err := listObjWithLabelsInNamespace(ctx, cli, generics.ComponentSignature, namespace, labels)
	if err != nil {
		return nil, err
	}

	mapping := make(map[string]string)
	for _, comp := range comps {
		if len(comp.Spec.CompDef) == 0 {
			continue
		}
		compName, err1 := ShortName(clusterName, comp.Name)
		if err1 != nil {
			return nil, err1
		}
		mapping[compName] = comp.Spec.CompDef
	}
	return mapping, nil
}

func buildCompDef2CompCount(ctx context.Context, cli client.Reader, namespace, clusterName string) (map[string]int32, error) {
	if cli == nil {
		return nil, nil // for test
	}

	clusterKey := types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	cluster := &appsv1.Cluster{}
	if err := cli.Get(ctx, clusterKey, cluster); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	result := make(map[string]int32)

	add := func(name string, cnt int32) {
		if len(name) > 0 {
			if val, ok := result[name]; !ok {
				result[name] = cnt
			} else {
				result[name] = val + cnt
			}
		}
	}

	for _, comp := range cluster.Spec.ComponentSpecs {
		add(comp.ComponentDef, 1)
	}
	for _, spec := range cluster.Spec.Shardings {
		add(spec.Template.ComponentDef, spec.Shards)
	}
	return result, nil
}

func mergeUserDefinedEnv(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
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

func buildVolumeClaimTemplates(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.VolumeClaimTemplates != nil {
		synthesizeComp.VolumeClaimTemplates = intctrlutil.ToCoreV1PVCTs(comp.Spec.VolumeClaimTemplates)
		for i := range synthesizeComp.VolumeClaimTemplates {
			vct := comp.Spec.VolumeClaimTemplates[i]
			if vct.PersistentVolumeClaimName != nil && len(*vct.PersistentVolumeClaimName) > 0 {
				if synthesizeComp.VolumeClaimTemplates[i].Annotations == nil {
					synthesizeComp.VolumeClaimTemplates[i].Annotations = map[string]string{}
				}
				synthesizeComp.VolumeClaimTemplates[i].Annotations[constant.PVCNamePrefixAnnotationKey] = *vct.PersistentVolumeClaimName
			}
		}
	}
	if comp.Spec.PersistentVolumeClaimRetentionPolicy != nil {
		synthesizeComp.PVCRetentionPolicy = *comp.Spec.PersistentVolumeClaimRetentionPolicy
	}
	if len(synthesizeComp.PVCRetentionPolicy.WhenDeleted) == 0 {
		synthesizeComp.PVCRetentionPolicy.WhenDeleted = defaultPVCRetentionPolicy.WhenDeleted
	}
	if len(synthesizeComp.PVCRetentionPolicy.WhenScaled) == 0 {
		synthesizeComp.PVCRetentionPolicy.WhenScaled = defaultPVCRetentionPolicy.WhenScaled
	}
}

func mergeUserDefinedVolumes(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
	if comp == nil {
		return nil
	}
	volumes := sets.New[string]()
	for _, vols := range [][]corev1.Volume{synthesizedComp.PodSpec.Volumes, comp.Spec.Volumes} {
		for _, vol := range vols {
			if volumes.Has(vol.Name) {
				return fmt.Errorf("duplicated volume %s", vol.Name)
			}
			volumes.Insert(vol.Name)
		}
	}
	for _, vct := range synthesizedComp.VolumeClaimTemplates {
		if volumes.Has(vct.Name) {
			return fmt.Errorf("duplicated volume %s", vct.Name)
		}
		volumes.Insert(vct.Name)
	}

	checkFileTemplate := func(name, volumeName string) error {
		if volumes.Has(volumeName) {
			return fmt.Errorf("duplicated volume %s for file template %s", volumeName, name)
		}
		volumes.Insert(volumeName)
		return nil
	}
	for _, tpl := range synthesizedComp.FileTemplates {
		if err := checkFileTemplate(tpl.Name, tpl.VolumeName); err != nil {
			return err
		}
	}
	synthesizedComp.PodSpec.Volumes = append(synthesizedComp.PodSpec.Volumes, comp.Spec.Volumes...)
	return nil
}

func buildVolumeMounts(synthesizedComp *SynthesizedComponent) {
	templateVolumes := sets.New[string]()
	for _, tpl := range synthesizedComp.FileTemplates {
		templateVolumes.Insert(tpl.VolumeName)
	}

	podSpec := synthesizedComp.PodSpec
	for _, cc := range []*[]corev1.Container{&podSpec.Containers, &podSpec.InitContainers} {
		volumes := podSpec.Volumes
		for _, c := range *cc {
			for _, v := range c.VolumeMounts {
				// if the volume mount belongs to the file template, skip
				if templateVolumes.Has(v.Name) {
					continue
				}
				// if persistence is not found, add an emptyDir to it
				createFn := func(_ string) corev1.Volume {
					return corev1.Volume{
						Name: v.Name,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}
				}
				volumes = intctrlutil.CreateVolumeIfNotExist(volumes, v.Name, createFn)
			}
		}
		podSpec.Volumes = volumes
	}
	synthesizedComp.PodSpec = podSpec
}

func mergeNetworkSetting(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.Network != nil {
		synthesizedComp.PodSpec.HostNetwork = comp.Spec.Network.HostNetwork
		synthesizedComp.PodSpec.HostAliases = comp.Spec.Network.HostAliases
		if comp.Spec.Network.DNSPolicy == nil {
			if synthesizedComp.PodSpec.HostNetwork {
				synthesizedComp.PodSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			} else {
				synthesizedComp.PodSpec.DNSPolicy = corev1.DNSClusterFirst
			}
		} else {
			synthesizedComp.PodSpec.DNSPolicy = *comp.Spec.Network.DNSPolicy
		}
		synthesizedComp.PodSpec.DNSConfig = comp.Spec.Network.DNSConfig
	}
}

// limitSharedMemoryVolumeSize limits the shared memory volume size to memory requests/limits.
func limitSharedMemoryVolumeSize(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
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

// buildAndUpdateResources updates podSpec resources from component
func buildAndUpdateResources(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.Resources.Requests != nil || comp.Spec.Resources.Limits != nil {
		synthesizeComp.PodSpec.Containers[0].Resources = comp.Spec.Resources
	}
}

func overrideComponentServices(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if len(synthesizeComp.ComponentServices) == 0 || len(comp.Spec.Services) == 0 {
		return
	}

	services := map[string]appsv1.ComponentService{}
	for i, svc := range comp.Spec.Services {
		services[svc.Name] = comp.Spec.Services[i]
	}

	override := func(svc *appsv1.ComponentService) {
		svc1, ok := services[svc.Name]
		if ok {
			svc.Spec.Type = svc1.Spec.Type
			svc.Annotations = svc1.Annotations
			svc.PodService = svc1.PodService
			if svc.DisableAutoProvision != nil {
				svc.DisableAutoProvision = ptr.To(false)
			}
		}
	}
	for i := range synthesizeComp.ComponentServices {
		override(&synthesizeComp.ComponentServices[i])
	}
}

func overrideNCheckConfigTemplates(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
	if comp == nil || len(comp.Spec.Configs) == 0 {
		return nil
	}

	templates := make(map[string]*SynthesizedFileTemplate)
	for i, tpl := range synthesizedComp.FileTemplates {
		if tpl.Config {
			templates[tpl.Name] = &synthesizedComp.FileTemplates[i]
		}
	}

	for _, config := range comp.Spec.Configs {
		if config.Name == nil || len(*config.Name) == 0 {
			continue // not supported now, ignore
		}
		tpl := templates[*config.Name]
		if tpl == nil {
			return fmt.Errorf("the config template %s is not defined", *config.Name)
		}

		specified := func() bool {
			return config.ConfigMap != nil && len(config.ConfigMap.Name) > 0
		}
		if specified() {
			tpl.Template = config.ConfigMap.Name
			tpl.Namespace = synthesizedComp.Namespace
			continue
		}

		if len(tpl.Template) == 0 && (tpl.ExternalManaged == nil || !*tpl.ExternalManaged) {
			return fmt.Errorf("there is no template provided for config template %s", *config.Name)
		}
	}
	return nil
}

func buildFileTemplates(synthesizedComp *SynthesizedComponent, compDef *appsv1.ComponentDefinition, comp *appsv1.Component) {
	templates := make([]SynthesizedFileTemplate, 0)
	for _, tpl := range compDef.Spec.Configs {
		templates = append(templates, synthesizeFileTemplate(comp, tpl, true))
	}
	for _, tpl := range compDef.Spec.Scripts {
		templates = append(templates, synthesizeFileTemplate(comp, tpl, false))
	}
	synthesizedComp.FileTemplates = templates
}

func synthesizeFileTemplate(comp *appsv1.Component, tpl appsv1.ComponentFileTemplate, config bool) SynthesizedFileTemplate {
	merge := func(tpl SynthesizedFileTemplate, utpl appsv1.ClusterComponentConfig) SynthesizedFileTemplate {
		tpl.Variables = utpl.Variables
		if utpl.ConfigMap != nil {
			tpl.Namespace = comp.Namespace
			tpl.Template = utpl.ConfigMap.Name
		}
		if utpl.ExternalManaged != nil {
			tpl.ExternalManaged = utpl.ExternalManaged
		}
		if ptr.Deref(tpl.ExternalManaged, false) {
			if utpl.ConfigMap == nil {
				// reset the template and wait the external system to provision it.
				tpl.Namespace = ""
				tpl.Template = ""
			}
		}
		if utpl.Reconfigure != nil {
			tpl.Reconfigure = utpl.Reconfigure // use the custom reconfigure action
		}
		return tpl
	}

	stpl := SynthesizedFileTemplate{
		ComponentFileTemplate: tpl,
		Config:                config,
	}
	if config {
		for _, utpl := range comp.Spec.Configs {
			if utpl.Name != nil && *utpl.Name == tpl.Name {
				return merge(stpl, utpl)
			}
		}
		return merge(stpl, appsv1.ClusterComponentConfig{})
	}
	return stpl
}

// buildServiceAccountName builds serviceAccountName for component and podSpec.
func buildServiceAccountName(synthesizeComp *SynthesizedComponent) {
	if synthesizeComp.ServiceAccountName != "" {
		synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
		return
	}
	if !viper.GetBool(constant.EnableRBACManager) {
		return
	}
	synthesizeComp.ServiceAccountName = constant.GenerateDefaultServiceAccountName(synthesizeComp.CompDefName)
	synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
}

func buildRuntimeClassName(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.RuntimeClassName == nil {
		return
	}
	synthesizeComp.PodSpec.RuntimeClassName = comp.Spec.RuntimeClassName
}

func getPodUpdatePolicy(comp *appsv1.Component, compDef *appsv1.ComponentDefinition) appsv1.PodUpdatePolicyType {
	policy := compDef.Spec.PodUpdatePolicy
	if policy != nil && *policy == appsv1.ReCreatePodUpdatePolicyType {
		return appsv1.ReCreatePodUpdatePolicyType
	}
	if comp.Spec.PodUpdatePolicy != nil {
		return *comp.Spec.PodUpdatePolicy
	}
	return appsv1.PreferInPlacePodUpdatePolicyType // default
}

func getPodUpgradePolicy(comp *appsv1.Component, compDef *appsv1.ComponentDefinition) appsv1.PodUpdatePolicyType {
	policy := compDef.Spec.PodUpgradePolicy
	if policy == nil {
		policy = compDef.Spec.PodUpdatePolicy
	}
	if policy != nil && *policy == appsv1.ReCreatePodUpdatePolicyType {
		return appsv1.ReCreatePodUpdatePolicyType
	}
	policy = comp.Spec.PodUpgradePolicy
	if policy == nil {
		policy = comp.Spec.PodUpdatePolicy
	}
	if policy != nil {
		return *policy
	}
	return appsv1.PreferInPlacePodUpdatePolicyType // default
}
