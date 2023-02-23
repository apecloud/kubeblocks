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

package component

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// BuildComponent generates a new Component object, which is a mixture of
// component-related configs from input Cluster, ClusterDef and ClusterVersion.
func BuildComponent(
	reqCtx intctrlutil.RequestCtx,
	cluster appsv1alpha1.Cluster,
	clusterDef appsv1alpha1.ClusterDefinition,
	clusterCompDef appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpec appsv1alpha1.ClusterComponentSpec,
	clusterCompVers ...*appsv1alpha1.ClusterComponentVersion,
) *SynthesizedComponent {

	clusterCompDefObj := clusterCompDef.DeepCopy()
	component := &SynthesizedComponent{
		ClusterDefName:        clusterDef.Name,
		Name:                  clusterCompSpec.Name,
		Type:                  clusterCompDefObj.Name,
		CharacterType:         clusterCompDefObj.CharacterType,
		MaxUnavailable:        clusterCompDefObj.MaxUnavailable,
		Replicas:              0,
		WorkloadType:          clusterCompDefObj.WorkloadType,
		ConsensusSpec:         clusterCompDefObj.ConsensusSpec,
		PodSpec:               clusterCompDefObj.PodSpec,
		Service:               clusterCompDefObj.Service,
		Probes:                clusterCompDefObj.Probes,
		LogConfigs:            clusterCompDefObj.LogConfigs,
		HorizontalScalePolicy: clusterCompDefObj.HorizontalScalePolicy,
	}

	// resolve component.ConfigTemplates
	if clusterCompDefObj.ConfigSpec != nil {
		component.ConfigTemplates = clusterCompDefObj.ConfigSpec.ConfigTemplateRefs
	}

	if len(clusterCompVers) > 0 && clusterCompVers[0] != nil {
		// only accept 1st ClusterVersion override context
		clusterCompVer := clusterCompVers[0]
		component.ConfigTemplates = cfgcore.MergeConfigTemplates(clusterCompVer.ConfigTemplateRefs, component.ConfigTemplates)
		// override component.PodSpec.InitContainers and component.PodSpec.Containers
		for _, c := range clusterCompVer.VersionsCtx.InitContainers {
			component.PodSpec.InitContainers = appendOrOverrideContainerAttr(component.PodSpec.InitContainers, c)
		}
		for _, c := range clusterCompVer.VersionsCtx.Containers {
			component.PodSpec.Containers = appendOrOverrideContainerAttr(component.PodSpec.Containers, c)
		}
	}

	// set affinity and tolerations
	affinity := cluster.Spec.Affinity
	if clusterCompSpec.Affinity != nil {
		affinity = clusterCompSpec.Affinity
	}
	podAffinity := buildPodAffinity(&cluster, affinity, component)
	component.PodSpec.Affinity = patchBuiltInAffinity(podAffinity)
	component.PodSpec.TopologySpreadConstraints = buildPodTopologySpreadConstraints(&cluster, affinity, component)

	tolerations := cluster.Spec.Tolerations
	if len(clusterCompSpec.Tolerations) != 0 {
		tolerations = clusterCompSpec.Tolerations
	}
	component.PodSpec.Tolerations = patchBuiltInToleration(tolerations)

	// set others
	component.EnabledLogs = clusterCompSpec.EnabledLogs
	component.Replicas = clusterCompSpec.Replicas
	component.TLS = clusterCompSpec.TLS
	component.Issuer = clusterCompSpec.Issuer

	if clusterCompSpec.VolumeClaimTemplates != nil {
		component.VolumeClaimTemplates = appsv1alpha1.ToVolumeClaimTemplates(clusterCompSpec.VolumeClaimTemplates)
	}

	if clusterCompSpec.Resources.Requests != nil || clusterCompSpec.Resources.Limits != nil {
		component.PodSpec.Containers[0].Resources = clusterCompSpec.Resources
	}

	if clusterCompSpec.ServiceType != "" {
		if component.Service == nil {
			component.Service = &corev1.ServiceSpec{}
		}
		component.Service.Type = clusterCompSpec.ServiceType
	}
	component.PrimaryIndex = clusterCompSpec.PrimaryIndex

	// TODO(zhixu.zt) We need to reserve the VolumeMounts of the container for ConfigMap or Secret,
	// At present, it is possible to distinguish between ConfigMap volume and normal volume,
	// Compare the VolumeName of configTemplateRef and Name of VolumeMounts
	//
	// if component.VolumeClaimTemplates == nil {
	//	 for i := range component.PodSpec.Containers {
	//	 	component.PodSpec.Containers[i].VolumeMounts = nil
	//	 }
	// }

	buildMonitorConfig(&clusterCompDef, &clusterCompSpec, component)
	err := buildProbeContainers(reqCtx, component)
	if err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
		return nil
	}

	replaceContainerPlaceholderTokens(component, GetEnvReplacementMapForConnCredential(cluster.GetName()))
	
	return component
}

// appendOrOverrideContainerAttr is used to append targetContainer to compContainers or override the attributes of compContainers with a given targetContainer,
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
func GetEnvReplacementMapForConnCredential(clusterName string) map[string]string {
	return map[string]string{
		constant.ConnCredentialPlaceHolder: GenerateConnCredential(clusterName),
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

// ReplaceNamedVars replaces the value in needle with namedValues and returns new needle
func ReplaceNamedVars(namedValuesMap map[string]string, needle string, limits int, matchAll bool) string {
	for k, v := range namedValuesMap {
		r := strings.Replace(needle, k, v, limits)
		// early termination on matching, when matchAll = false
		if r != needle && !matchAll {
			return r
		}
		needle = r
	}
	return needle
}

// ReplaceSecretEnvVars replaces the env secret value with namedValues and returns new envs
func ReplaceSecretEnvVars(namedValuesMap map[string]string, envs []corev1.EnvVar) []corev1.EnvVar {
	newEnvs := make([]corev1.EnvVar, len(envs))
	for _, e := range envs {
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		secretRef := e.ValueFrom.SecretKeyRef
		name := ReplaceNamedVars(namedValuesMap, secretRef.Name, 1, false)
		if name != secretRef.Name {
			secretRef.Name = name
		}
		newEnvs = append(newEnvs, e)
	}
	return newEnvs
}

func GetClusterDefCompByName(clusterDef appsv1alpha1.ClusterDefinition,
	cluster appsv1alpha1.Cluster,
	compName string) *appsv1alpha1.ClusterComponentDefinition {
	for _, comp := range cluster.Spec.ComponentSpecs {
		if comp.Name != compName {
			continue
		}
		for _, compDef := range clusterDef.Spec.ComponentDefs {
			if compDef.Name == comp.ComponentDefRef {
				return &compDef
			}
		}
	}
	return nil
}

func GenerateConnCredential(clusterName string) string {
	return fmt.Sprintf("%s-conn-credential", clusterName)
}
