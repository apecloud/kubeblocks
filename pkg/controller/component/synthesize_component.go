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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	defaultShmQuantity = resource.MustParse("64Mi")
)

// BuildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from ComponentDefinition and Component.
// TODO: remove @ctx & @cli
func BuildSynthesizedComponent(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, comp *appsv1.Component, cluster *appsv1.Cluster) (*SynthesizedComponent, error) {
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
	comp2CompDef, err := buildComp2CompDefs(ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	compDefObj := compDef.DeepCopy()
	synthesizeComp := &SynthesizedComponent{
		Namespace:                        comp.Namespace,
		ClusterName:                      clusterName,
		ClusterUID:                       clusterUID,
		Comp2CompDefs:                    comp2CompDef,
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
		HostNetwork:                      compDefObj.Spec.HostNetwork,
		ComponentServices:                compDefObj.Spec.Services,
		LogConfigs:                       compDefObj.Spec.LogConfigs,
		ConfigTemplates:                  compDefObj.Spec.Configs,
		ScriptTemplates:                  compDefObj.Spec.Scripts,
		Roles:                            compDefObj.Spec.Roles,
		UpdateStrategy:                   compDefObj.Spec.UpdateStrategy,
		MinReadySeconds:                  compDefObj.Spec.MinReadySeconds,
		PolicyRules:                      compDefObj.Spec.PolicyRules,
		LifecycleActions:                 compDefObj.Spec.LifecycleActions,
		SystemAccounts:                   mergeSystemAccounts(compDefObj.Spec.SystemAccounts, comp.Spec.SystemAccounts),
		Replicas:                         comp.Spec.Replicas,
		Resources:                        comp.Spec.Resources,
		TLSConfig:                        comp.Spec.TLSConfig,
		ServiceAccountName:               comp.Spec.ServiceAccountName,
		Instances:                        comp.Spec.Instances,
		OfflineInstances:                 comp.Spec.OfflineInstances,
		DisableExporter:                  comp.Spec.DisableExporter,
		Stop:                             comp.Spec.Stop,
		PodManagementPolicy:              compDef.Spec.PodManagementPolicy,
		ParallelPodManagementConcurrency: comp.Spec.ParallelPodManagementConcurrency,
		PodUpdatePolicy:                  comp.Spec.PodUpdatePolicy,
	}

	buildCompatibleHorizontalScalePolicy(compDefObj, synthesizeComp)

	if err = mergeUserDefinedEnv(synthesizeComp, comp); err != nil {
		return nil, err
	}

	// build scheduling policy for workload
	buildSchedulingPolicy(synthesizeComp, comp)

	// update resources
	buildAndUpdateResources(synthesizeComp, comp)

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

	if err = buildKBAgentContainer(synthesizeComp); err != nil {
		return nil, errors.Wrap(err, "build kb-agent container failed")
	}

	if err = buildServiceReferences(ctx, cli, synthesizeComp, compDef, comp); err != nil {
		return nil, errors.Wrap(err, "build service references failed")
	}

	return synthesizeComp, nil
}

func buildComp2CompDefs(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) (map[string]string, error) {
	if cluster == nil {
		return nil, nil
	}
	mapping := make(map[string]string)

	// build from componentSpecs
	for _, comp := range cluster.Spec.ComponentSpecs {
		if len(comp.ComponentDef) > 0 {
			mapping[comp.Name] = comp.ComponentDef
		}
	}

	// build from shardings
	for _, spec := range cluster.Spec.Shardings {
		shardingComps, err := intctrlutil.ListShardingComponents(ctx, cli, cluster, spec.Name)
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

func mergeSystemAccounts(compDefAccounts []appsv1.SystemAccount,
	compAccounts []appsv1.ComponentSystemAccount) []appsv1.SystemAccount {
	if len(compAccounts) == 0 {
		return compDefAccounts
	}

	override := func(compAccount appsv1.ComponentSystemAccount, idx int) {
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

func buildSchedulingPolicy(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.SchedulingPolicy != nil {
		schedulingPolicy := comp.Spec.SchedulingPolicy
		synthesizedComp.PodSpec.SchedulerName = schedulingPolicy.SchedulerName
		synthesizedComp.PodSpec.NodeSelector = schedulingPolicy.NodeSelector
		synthesizedComp.PodSpec.NodeName = schedulingPolicy.NodeName
		synthesizedComp.PodSpec.Affinity = schedulingPolicy.Affinity
		synthesizedComp.PodSpec.Tolerations = schedulingPolicy.Tolerations
		synthesizedComp.PodSpec.TopologySpreadConstraints = schedulingPolicy.TopologySpreadConstraints
	}
}

func buildVolumeClaimTemplates(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.VolumeClaimTemplates != nil {
		synthesizeComp.VolumeClaimTemplates = ToVolumeClaimTemplates(comp.Spec.VolumeClaimTemplates)
	}
}

func mergeUserDefinedVolumes(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
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

	checkConfigNScriptTemplate := func(tpl appsv1.ComponentTemplateSpec) error {
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

func ToVolumeClaimTemplates(vcts []appsv1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaimTemplate {
	storageClassName := func(spec appsv1.PersistentVolumeClaimSpec, defaultStorageClass string) *string {
		if spec.StorageClassName != nil && *spec.StorageClassName != "" {
			return spec.StorageClassName
		}
		if defaultStorageClass != "" {
			return &defaultStorageClass
		}
		return nil
	}
	var ts []corev1.PersistentVolumeClaimTemplate
	for _, t := range vcts {
		ts = append(ts, corev1.PersistentVolumeClaimTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.Name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      t.Spec.AccessModes,
				Resources:        t.Spec.Resources,
				StorageClassName: storageClassName(t.Spec, viper.GetString(constant.CfgKeyDefaultStorageClass)),
				VolumeMode:       t.Spec.VolumeMode,
			},
		})
	}
	return ts
}

// buildAndUpdateResources updates podSpec resources from component
func buildAndUpdateResources(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.Resources.Requests != nil || comp.Spec.Resources.Limits != nil {
		synthesizeComp.PodSpec.Containers[0].Resources = comp.Spec.Resources
	}
}

func buildComponentServices(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
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
				svc.DisableAutoProvision = func() *bool { b := false; return &b }()
			}
		}
	}
	for i := range synthesizeComp.ComponentServices {
		override(&synthesizeComp.ComponentServices[i])
	}
}

func overrideConfigTemplates(synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
	if comp == nil || len(comp.Spec.Configs) == 0 {
		return nil
	}

	templates := make(map[string]*appsv1.ComponentConfigSpec)
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
	if synthesizeComp.ServiceAccountName != "" {
		synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
		return
	}
	if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.RoleProbe == nil {
		return
	}
	synthesizeComp.ServiceAccountName = constant.GenerateDefaultServiceAccountName(synthesizeComp.ClusterName, synthesizeComp.Name)
	synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
}

func buildRuntimeClassName(synthesizeComp *SynthesizedComponent, comp *appsv1.Component) {
	if comp.Spec.RuntimeClassName == nil {
		return
	}
	synthesizeComp.PodSpec.RuntimeClassName = comp.Spec.RuntimeClassName
}

func buildCompatibleHorizontalScalePolicy(compDef *appsv1.ComponentDefinition, synthesizeComp *SynthesizedComponent) {
	if compDef.Annotations != nil {
		templateName, ok := compDef.Annotations[constant.HorizontalScaleBackupPolicyTemplateKey]
		if ok {
			synthesizeComp.HorizontalScaleBackupPolicyTemplate = &templateName
		}
	}
}

func GetConfigSpecByName(synthesizedComp *SynthesizedComponent, configSpec string) *appsv1.ComponentConfigSpec {
	for i := range synthesizedComp.ConfigTemplates {
		template := &synthesizedComp.ConfigTemplates[i]
		if template.Name == configSpec {
			return template
		}
	}
	return nil
}
