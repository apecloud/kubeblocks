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
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterDefComp *appsv1alpha1.ClusterComponentDefinition,
	clusterVersionComp *appsv1alpha1.ClusterComponentVersion,
	clusterComp *appsv1alpha1.ClusterComponentSpec) *SynthesizedComponent {
	if clusterDefComp == nil {
		reqCtx.Log.Error(nil, "build probe container failed.")
		return nil
	}

	clusterDefCompObj := clusterDefComp.DeepCopy()
	component := &SynthesizedComponent{
		ClusterDefName:        clusterDef.Name,
		Name:                  clusterDefCompObj.Name, // initial name for the component will be same as Name
		Type:                  clusterDefCompObj.Name,
		CharacterType:         clusterDefCompObj.CharacterType,
		MaxUnavailable:        clusterDefCompObj.MaxUnavailable,
		Replicas:              0,
		WorkloadType:          clusterDefCompObj.WorkloadType,
		ConsensusSpec:         clusterDefCompObj.ConsensusSpec,
		PodSpec:               clusterDefCompObj.PodSpec,
		Service:               clusterDefCompObj.Service,
		Probes:                clusterDefCompObj.Probes,
		LogConfigs:            clusterDefCompObj.LogConfigs,
		HorizontalScalePolicy: clusterDefCompObj.HorizontalScalePolicy,
	}

	// resolve component.ConfigTemplates
	if clusterDefCompObj.ConfigSpec != nil {
		component.ConfigTemplates = clusterDefCompObj.ConfigSpec.ConfigTemplateRefs
	}
	if clusterVersionComp != nil {
		component.ConfigTemplates = cfgcore.MergeConfigTemplates(clusterVersionComp.ConfigTemplateRefs, component.ConfigTemplates)
	}

	// set component.PodSpec.InitContainers and component.PodSpec.Containers
	if clusterVersionComp != nil {
		if clusterVersionComp.PodSpec != nil {
			for _, c := range clusterVersionComp.PodSpec.InitContainers {
				component.PodSpec.InitContainers = appendOrOverrideContainerAttr(component.PodSpec.InitContainers, c)
			}
			for _, c := range clusterVersionComp.PodSpec.Containers {
				component.PodSpec.Containers = appendOrOverrideContainerAttr(component.PodSpec.Containers, c)
			}
		}
	}

	// set affinity and tolerations
	affinity := cluster.Spec.Affinity
	tolerations := cluster.Spec.Tolerations
	if clusterComp != nil {
		if clusterComp.Affinity != nil {
			affinity = clusterComp.Affinity
		}
		if len(clusterComp.Tolerations) != 0 {
			tolerations = clusterComp.Tolerations
		}
	}
	if affinity != nil {
		component.PodSpec.Affinity = buildPodAffinity(cluster, affinity, component)
		component.PodSpec.TopologySpreadConstraints = buildPodTopologySpreadConstraints(cluster, affinity, component)
	}
	if tolerations != nil {
		component.PodSpec.Tolerations = tolerations
	}

	// set others
	if clusterComp != nil {
		component.EnabledLogs = clusterComp.EnabledLogs

		if len(clusterComp.Name) > 0 {
			component.Name = clusterComp.Name
		}

		// user can scale in replicas to 0
		if clusterComp.Replicas != nil {
			component.Replicas = *clusterComp.Replicas
		}

		if clusterComp.VolumeClaimTemplates != nil {
			component.VolumeClaimTemplates = appsv1alpha1.ToVolumeClaimTemplates(clusterComp.VolumeClaimTemplates)
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
		component.PrimaryIndex = clusterComp.PrimaryIndex
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

	buildMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
	err := buildProbeContainers(reqCtx, component)
	if err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
		return nil
	}

	replacePlaceholderTokens(component, map[string]string{
		constant.ConnCredentialPlaceHolder: fmt.Sprintf("%s-conn-credential", cluster.GetName()),
	})
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

func replacePlaceholderTokens(component *SynthesizedComponent, namedValues map[string]string) {
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
