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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func getContainerByName(containers []corev1.Container, name string) (int, *corev1.Container) {
	for i, container := range containers {
		if container.Name == name {
			return i, &container
		}
	}
	return -1, nil
}

func toK8sVolumeClaimTemplate(template dbaasv1alpha1.ClusterComponentVolumeClaimTemplate) corev1.PersistentVolumeClaimTemplate {
	t := corev1.PersistentVolumeClaimTemplate{}
	t.ObjectMeta.Name = template.Name
	if template.Spec != nil {
		t.Spec = *template.Spec
	}
	return t
}

func toK8sVolumeClaimTemplates(templates []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaimTemplate {
	ts := []corev1.PersistentVolumeClaimTemplate{}
	for _, template := range templates {
		ts = append(ts, toK8sVolumeClaimTemplate(template))
	}
	return ts
}

func buildAffinityLabelSelector(clusterName string, componentName string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			intctrlutil.AppInstanceLabelKey:  clusterName,
			intctrlutil.AppComponentLabelKey: componentName,
		},
	}
}

func buildPodTopologySpreadConstraints(
	cluster *dbaasv1alpha1.Cluster,
	comAffinity *dbaasv1alpha1.Affinity,
	component *Component,
) []corev1.TopologySpreadConstraint {
	var topologySpreadConstraints []corev1.TopologySpreadConstraint

	var whenUnsatisfiable corev1.UnsatisfiableConstraintAction
	if comAffinity.PodAntiAffinity == dbaasv1alpha1.Required {
		whenUnsatisfiable = corev1.DoNotSchedule
	} else {
		whenUnsatisfiable = corev1.ScheduleAnyway
	}
	for _, topologyKey := range comAffinity.TopologyKeys {
		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			WhenUnsatisfiable: whenUnsatisfiable,
			TopologyKey:       topologyKey,
			LabelSelector:     buildAffinityLabelSelector(cluster.Name, component.Name),
		})
	}
	return topologySpreadConstraints
}

func buildPodAffinity(
	cluster *dbaasv1alpha1.Cluster,
	comAffinity *dbaasv1alpha1.Affinity,
	component *Component,
) *corev1.Affinity {
	affinity := new(corev1.Affinity)
	// Build NodeAffinity
	var matchExpressions []corev1.NodeSelectorRequirement
	for key, value := range comAffinity.NodeLabels {
		values := strings.Split(value, ",")
		matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   values,
		})
	}
	if len(matchExpressions) > 0 {
		nodeSelectorTerm := corev1.NodeSelectorTerm{
			MatchExpressions: matchExpressions,
		}
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{nodeSelectorTerm},
			},
		}
	}
	// Build PodAntiAffinity
	var podAntiAffinity *corev1.PodAntiAffinity
	var podAffinityTerms []corev1.PodAffinityTerm
	for _, topologyKey := range comAffinity.TopologyKeys {
		podAffinityTerms = append(podAffinityTerms, corev1.PodAffinityTerm{
			TopologyKey:   topologyKey,
			LabelSelector: buildAffinityLabelSelector(cluster.Name, component.Name),
		})
	}
	if comAffinity.PodAntiAffinity == dbaasv1alpha1.Required {
		podAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinityTerms,
		}
	} else {
		var weightedPodAffinityTerms []corev1.WeightedPodAffinityTerm
		for _, podAffinityTerm := range podAffinityTerms {
			weightedPodAffinityTerms = append(weightedPodAffinityTerms, corev1.WeightedPodAffinityTerm{
				Weight:          100,
				PodAffinityTerm: podAffinityTerm,
			})
		}
		podAntiAffinity = &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: weightedPodAffinityTerms,
		}
	}
	affinity.PodAntiAffinity = podAntiAffinity
	return affinity
}

func disableMonitor(component *Component) {
	component.Monitor = &MonitorConfig{
		Enable: false,
	}
}

func mergeMonitorConfig(
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent,
	component *Component) {
	monitorEnable := false
	if clusterComp != nil {
		monitorEnable = clusterComp.Monitor
	}

	monitorConfig := clusterDefComp.Monitor
	if !monitorEnable || monitorConfig == nil {
		disableMonitor(component)
		return
	}

	if !monitorConfig.BuiltIn {
		if monitorConfig.Exporter == nil {
			disableMonitor(component)
			return
		}
		component.Monitor = &MonitorConfig{
			Enable:     true,
			ScrapePath: monitorConfig.Exporter.ScrapePath,
			ScrapePort: monitorConfig.Exporter.ScrapePort,
		}
		return
	}

	characterType := clusterDefComp.CharacterType
	if !isWellKnownCharacterType(characterType) {
		disableMonitor(component)
		return
	}

	switch characterType {
	case kMysql:
		err := wellKnownCharacterTypeFunc[kMysql](cluster, component)
		if err != nil {
			disableMonitor(component)
		}
	default:
		disableMonitor(component)
	}
}

// MergeComponents generates a new Component object, which is a mixture of
// component-related configs from input Cluster, ClusterDef and ClusterVersion.
func MergeComponents(
	reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	clusterVersionComp *dbaasv1alpha1.ClusterVersionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent) *Component {
	if clusterDefComp == nil {
		return nil
	}

	clusterDefCompObj := clusterDefComp.DeepCopy()
	component := &Component{
		ClusterDefName:        clusterDef.Name,
		Name:                  clusterDefCompObj.Name, // initial name for the component will be same as Name
		Type:                  clusterDefCompObj.Name,
		CharacterType:         clusterDefCompObj.CharacterType,
		MaxUnavailable:        clusterDefCompObj.MaxUnavailable,
		Replicas:              0,
		AntiAffinity:          clusterDefCompObj.AntiAffinity,
		ComponentType:         clusterDefCompObj.WorkloadType,
		ConsensusSpec:         clusterDefCompObj.ConsensusSpec,
		PodSpec:               clusterDefCompObj.PodSpec,
		Service:               clusterDefCompObj.Service,
		Probes:                clusterDefCompObj.Probes,
		LogConfigs:            clusterDefCompObj.LogConfigs,
		HorizontalScalePolicy: clusterDefCompObj.HorizontalScalePolicy,
	}

	if clusterDefCompObj.ConfigSpec != nil {
		component.ConfigTemplates = clusterDefCompObj.ConfigSpec.ConfigTemplateRefs
	}

	if clusterVersionComp != nil {
		component.ConfigTemplates = cfgcore.MergeConfigTemplates(clusterVersionComp.ConfigTemplateRefs, component.ConfigTemplates)
		if clusterVersionComp.PodSpec != nil {
			for _, c := range clusterVersionComp.PodSpec.InitContainers {
				component.PodSpec.InitContainers = appendOrOverrideContainerAttr(component.PodSpec.InitContainers, c)
			}
			for _, c := range clusterVersionComp.PodSpec.Containers {
				component.PodSpec.Containers = appendOrOverrideContainerAttr(component.PodSpec.Containers, c)
			}
		}
	}
	affinity := cluster.Spec.Affinity
	tolerations := cluster.Spec.Tolerations
	if clusterComp != nil {
		component.Name = clusterComp.Name // component name gets overrided
		component.EnabledLogs = clusterComp.EnabledLogs

		// user can scale in replicas to 0
		if clusterComp.Replicas != nil {
			component.Replicas = *clusterComp.Replicas
		}

		if clusterComp.VolumeClaimTemplates != nil {
			component.VolumeClaimTemplates = toK8sVolumeClaimTemplates(clusterComp.VolumeClaimTemplates)
		}

		if clusterComp.Resources.Requests != nil || clusterComp.Resources.Limits != nil {
			component.PodSpec.Containers[0].Resources = clusterComp.Resources
		}

		if clusterComp.ServiceType != "" {
			if component.Service == nil {
				component.Service = &corev1.ServiceSpec{}
			}
			component.Service.Type = clusterComp.ServiceType
		}

		if clusterComp.Affinity != nil {
			affinity = clusterComp.Affinity
		}
		if len(clusterComp.Tolerations) != 0 {
			tolerations = clusterComp.Tolerations
		}

		component.PrimaryIndex = clusterComp.PrimaryIndex
	}
	if affinity != nil {
		component.PodSpec.Affinity = buildPodAffinity(cluster, affinity, component)
		component.PodSpec.TopologySpreadConstraints = buildPodTopologySpreadConstraints(cluster, affinity, component)
	}
	if tolerations != nil {
		component.PodSpec.Tolerations = tolerations
	}

	// TODO(zhixu.zt) We need to reserve the VolumeMounts of the container for ConfigMap or Secret,
	// At present, it is possible to distinguish between ConfigMap volume and normal volume,
	// Compare the VolumeName of configTemplateRef and Name of VolumeMounts
	//
	// if component.VolumeClaimTemplates == nil {
	//	 for i := range component.PodSpec.Containers {
	//	 	component.PodSpec.Containers[i].VolumeMounts = nil
	//	 }
	// }

	mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
	err := buildProbeContainers(reqCtx, component)
	if err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
	}
	replacePlaceholderTokens(cluster, component)

	return component
}

// appendOrOverrideContainerAttr is used to append targetContainer to compContainers or override the attributes of compContainers with a given targetContainer,
// if targetContainer does not exist in compContainers, it will be appended. otherwise it will be updated with the attributes of the target container.
func appendOrOverrideContainerAttr(compContainers []corev1.Container, targetContainer corev1.Container) []corev1.Container {
	index, compContainer := getContainerByName(compContainers, targetContainer.Name)
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

func replacePlaceholderTokens(cluster *dbaasv1alpha1.Cluster, component *Component) {
	namedValues := getEnvReplacementMapForConnCredential(cluster.GetName())

	// replace env[].valueFrom.secretKeyRef.name variables
	for _, cc := range [][]corev1.Container{component.PodSpec.InitContainers, component.PodSpec.Containers} {
		for _, c := range cc {
			for _, e := range c.Env {
				if e.ValueFrom == nil {
					continue
				}
				if e.ValueFrom.SecretKeyRef == nil {
					continue
				}
				secretRef := e.ValueFrom.SecretKeyRef
				for k, v := range namedValues {
					r := strings.Replace(secretRef.Name, k, v, 1)
					if r == secretRef.Name {
						continue
					}
					secretRef.Name = r
					break
				}
			}
		}
	}
}

func getEnvReplacementMapForConnCredential(clusterName string) map[string]string {
	return map[string]string{
		"$(CONN_CREDENTIAL_SECRET_NAME)": fmt.Sprintf("%s-conn-credential", clusterName),
	}
}
