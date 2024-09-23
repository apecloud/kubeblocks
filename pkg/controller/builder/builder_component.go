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

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type ComponentBuilder struct {
	BaseBuilder[appsv1.Component, *appsv1.Component, ComponentBuilder]
}

func NewComponentBuilder(namespace, name, compDef string) *ComponentBuilder {
	builder := &ComponentBuilder{}
	builder.init(namespace, name,
		&appsv1.Component{
			Spec: appsv1.ComponentSpec{
				CompDef: compDef,
			},
		}, builder)
	return builder
}

func (builder *ComponentBuilder) SetServiceVersion(serviceVersion string) *ComponentBuilder {
	builder.get().Spec.ServiceVersion = serviceVersion
	return builder
}

func (builder *ComponentBuilder) SetLabels(labels map[string]string) *ComponentBuilder {
	builder.get().Spec.Labels = labels
	return builder
}

func (builder *ComponentBuilder) SetAnnotations(annotations map[string]string) *ComponentBuilder {
	builder.get().Spec.Annotations = annotations
	return builder
}

func (builder *ComponentBuilder) SetEnv(env []corev1.EnvVar) *ComponentBuilder {
	builder.get().Spec.Env = env
	return builder
}

func (builder *ComponentBuilder) SetSchedulingPolicy(schedulingPolicy *appsv1.SchedulingPolicy) *ComponentBuilder {
	builder.get().Spec.SchedulingPolicy = schedulingPolicy
	return builder
}

func (builder *ComponentBuilder) SetReplicas(replicas int32) *ComponentBuilder {
	builder.get().Spec.Replicas = replicas
	return builder
}

func (builder *ComponentBuilder) SetConfigs(configs []appsv1.ClusterComponentConfig) *ComponentBuilder {
	builder.get().Spec.Configs = configs
	return builder
}

func (builder *ComponentBuilder) SetServiceAccountName(serviceAccountName string) *ComponentBuilder {
	builder.get().Spec.ServiceAccountName = serviceAccountName
	return builder
}

func (builder *ComponentBuilder) SetParallelPodManagementConcurrency(parallelPodManagementConcurrency *intstr.IntOrString) *ComponentBuilder {
	builder.get().Spec.ParallelPodManagementConcurrency = parallelPodManagementConcurrency
	return builder
}

func (builder *ComponentBuilder) SetPodUpdatePolicy(policy *appsv1.PodUpdatePolicyType) *ComponentBuilder {
	builder.get().Spec.PodUpdatePolicy = policy
	return builder
}

func (builder *ComponentBuilder) SetResources(resources corev1.ResourceRequirements) *ComponentBuilder {
	builder.get().Spec.Resources = resources
	return builder
}

func (builder *ComponentBuilder) SetDisableExporter(disableExporter *bool) *ComponentBuilder {
	builder.get().Spec.DisableExporter = disableExporter
	return builder
}

func (builder *ComponentBuilder) SetUserConfigTemplates(userConfigTemplates map[string]appsv1.ConfigTemplateExtension) *ComponentBuilder {
	builder.get().Spec.UserConfigTemplates = userConfigTemplates
	return builder
}

func (builder *ComponentBuilder) SetParameters(parameters appsv1.ComponentParameters) *ComponentBuilder {
	builder.get().Spec.ComponentParameters = parameters
	return builder
}

func (builder *ComponentBuilder) SetTLSConfig(enable bool, issuer *appsv1.Issuer) *ComponentBuilder {
	if enable {
		builder.get().Spec.TLSConfig = &appsv1.TLSConfig{
			Enable: enable,
			Issuer: issuer,
		}
	}
	return builder
}

func (builder *ComponentBuilder) AddVolumeClaimTemplate(volumeName string, pvcSpec appsv1.PersistentVolumeClaimSpec) *ComponentBuilder {
	builder.get().Spec.VolumeClaimTemplates = append(builder.get().Spec.VolumeClaimTemplates, appsv1.ClusterComponentVolumeClaimTemplate{
		Name: volumeName,
		Spec: pvcSpec,
	})
	return builder
}

func (builder *ComponentBuilder) SetVolumeClaimTemplates(volumeClaimTemplates []appsv1.ClusterComponentVolumeClaimTemplate) *ComponentBuilder {
	builder.get().Spec.VolumeClaimTemplates = volumeClaimTemplates
	return builder
}

func (builder *ComponentBuilder) SetVolumes(volumes []corev1.Volume) *ComponentBuilder {
	builder.get().Spec.Volumes = volumes
	return builder
}

func (builder *ComponentBuilder) SetServices(services []appsv1.ClusterComponentService) *ComponentBuilder {
	toCompService := func(svc appsv1.ClusterComponentService) appsv1.ComponentService {
		return appsv1.ComponentService{
			Service: appsv1.Service{
				Name:        svc.Name,
				Annotations: svc.Annotations,
				Spec: corev1.ServiceSpec{
					Type: svc.ServiceType,
				},
			},
			PodService: svc.PodService,
		}
	}
	for _, svc := range services {
		builder.get().Spec.Services = append(builder.get().Spec.Services, toCompService(svc))
	}
	return builder
}

func (builder *ComponentBuilder) SetSystemAccounts(systemAccounts []appsv1.ComponentSystemAccount) *ComponentBuilder {
	builder.get().Spec.SystemAccounts = systemAccounts
	return builder
}

func (builder *ComponentBuilder) SetServiceRefs(serviceRefs []appsv1.ServiceRef) *ComponentBuilder {
	builder.get().Spec.ServiceRefs = serviceRefs
	return builder
}

func (builder *ComponentBuilder) SetInstances(instances []appsv1.InstanceTemplate) *ComponentBuilder {
	builder.get().Spec.Instances = instances
	return builder
}

func (builder *ComponentBuilder) SetOfflineInstances(offlineInstances []string) *ComponentBuilder {
	builder.get().Spec.OfflineInstances = offlineInstances
	return builder
}

func (builder *ComponentBuilder) SetRuntimeClassName(runtimeClassName *string) *ComponentBuilder {
	if runtimeClassName != nil {
		className := *runtimeClassName
		builder.get().Spec.RuntimeClassName = &className
	}
	return builder
}

func (builder *ComponentBuilder) SetStop(stop *bool) *ComponentBuilder {
	builder.get().Spec.Stop = stop
	return builder
}
