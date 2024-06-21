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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ComponentBuilder struct {
	BaseBuilder[appsv1alpha1.Component, *appsv1alpha1.Component, ComponentBuilder]
}

func NewComponentBuilder(namespace, name, compDef string) *ComponentBuilder {
	builder := &ComponentBuilder{}
	builder.init(namespace, name,
		&appsv1alpha1.Component{
			Spec: appsv1alpha1.ComponentSpec{
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

func (builder *ComponentBuilder) SetAffinity(affinity *appsv1alpha1.Affinity) *ComponentBuilder {
	builder.get().Spec.Affinity = affinity
	return builder
}

func (builder *ComponentBuilder) SetToleration(toleration corev1.Toleration) *ComponentBuilder {
	tolerations := builder.get().Spec.Tolerations
	if len(tolerations) == 0 {
		tolerations = []corev1.Toleration{}
	}
	tolerations = append(tolerations, toleration)
	builder.get().Spec.Tolerations = tolerations
	return builder
}

func (builder *ComponentBuilder) SetTolerations(tolerations []corev1.Toleration) *ComponentBuilder {
	builder.get().Spec.Tolerations = tolerations
	return builder
}

func (builder *ComponentBuilder) SetReplicas(replicas int32) *ComponentBuilder {
	builder.get().Spec.Replicas = replicas
	return builder
}

func (builder *ComponentBuilder) SetServiceAccountName(serviceAccountName string) *ComponentBuilder {
	builder.get().Spec.ServiceAccountName = serviceAccountName
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

func (builder *ComponentBuilder) SetEnabledLogs(logNames []string) *ComponentBuilder {
	builder.get().Spec.EnabledLogs = logNames
	return builder
}

func (builder *ComponentBuilder) SetTLSConfig(enable bool, issuer *appsv1alpha1.Issuer) *ComponentBuilder {
	if enable {
		builder.get().Spec.TLSConfig = &appsv1alpha1.TLSConfig{
			Enable: enable,
			Issuer: issuer,
		}
	}
	return builder
}

func (builder *ComponentBuilder) AddVolumeClaimTemplate(volumeName string,
	pvcSpec appsv1alpha1.PersistentVolumeClaimSpec) *ComponentBuilder {
	builder.get().Spec.VolumeClaimTemplates = append(builder.get().Spec.VolumeClaimTemplates, appsv1alpha1.ClusterComponentVolumeClaimTemplate{
		Name: volumeName,
		Spec: pvcSpec,
	})
	return builder
}

func (builder *ComponentBuilder) SetVolumeClaimTemplates(volumeClaimTemplates []appsv1alpha1.ClusterComponentVolumeClaimTemplate) *ComponentBuilder {
	builder.get().Spec.VolumeClaimTemplates = volumeClaimTemplates
	return builder
}

func (builder *ComponentBuilder) SetVolumes(volumes []corev1.Volume) *ComponentBuilder {
	builder.get().Spec.Volumes = volumes
	return builder
}

func (builder *ComponentBuilder) SetServices(services []appsv1alpha1.ClusterComponentService) *ComponentBuilder {
	toCompService := func(svc appsv1alpha1.ClusterComponentService) appsv1alpha1.ComponentService {
		return appsv1alpha1.ComponentService{
			Service: appsv1alpha1.Service{
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

func (builder *ComponentBuilder) SetSystemAccounts(systemAccounts []appsv1alpha1.ComponentSystemAccount) *ComponentBuilder {
	builder.get().Spec.SystemAccounts = systemAccounts
	return builder
}

func (builder *ComponentBuilder) SetServiceRefs(serviceRefs []appsv1alpha1.ServiceRef) *ComponentBuilder {
	builder.get().Spec.ServiceRefs = serviceRefs
	return builder
}

func (builder *ComponentBuilder) SetClassRef(classRef *appsv1alpha1.ClassDefRef) *ComponentBuilder {
	builder.get().Spec.ClassDefRef = classRef
	return builder
}

func (builder *ComponentBuilder) SetInstances(instances []appsv1alpha1.InstanceTemplate) *ComponentBuilder {
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
