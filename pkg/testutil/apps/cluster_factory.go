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

package apps

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type MockClusterFactory struct {
	BaseFactory[appsv1alpha1.Cluster, *appsv1alpha1.Cluster, MockClusterFactory]
}

func NewClusterFactory(namespace, name, cdRef, cvRef string) *MockClusterFactory {
	f := &MockClusterFactory{}
	f.Init(namespace, name,
		&appsv1alpha1.Cluster{
			Spec: appsv1alpha1.ClusterSpec{
				ClusterDefRef:     cdRef,
				ClusterVersionRef: cvRef,
				ComponentSpecs:    []appsv1alpha1.ClusterComponentSpec{},
				TerminationPolicy: appsv1alpha1.WipeOut,
			},
		}, f)
	return f
}

func (factory *MockClusterFactory) SetTopology(topology string) *MockClusterFactory {
	factory.Get().Spec.Topology = topology
	return factory
}

func (factory *MockClusterFactory) SetTerminationPolicy(policyType appsv1alpha1.TerminationPolicyType) *MockClusterFactory {
	factory.Get().Spec.TerminationPolicy = policyType
	return factory
}

func (factory *MockClusterFactory) SetClusterAffinity(affinity *appsv1alpha1.Affinity) *MockClusterFactory {
	factory.Get().Spec.Affinity = affinity
	return factory
}

func (factory *MockClusterFactory) AddClusterToleration(toleration corev1.Toleration) *MockClusterFactory {
	tolerations := factory.Get().Spec.Tolerations
	if len(tolerations) == 0 {
		tolerations = []corev1.Toleration{}
	}
	tolerations = append(tolerations, toleration)
	factory.Get().Spec.Tolerations = tolerations
	return factory
}

func (factory *MockClusterFactory) AddShardingSpec(shardingName string, compDefName string) *MockClusterFactory {
	shardingSpec := appsv1alpha1.ShardingSpec{
		Template: appsv1alpha1.ClusterComponentSpec{
			Name:            "fake",
			ComponentDefRef: compDefName,
			Replicas:        1,
		},
		Name:   shardingName,
		Shards: 1,
	}
	factory.Get().Spec.ShardingSpecs = append(factory.Get().Spec.ShardingSpecs, shardingSpec)
	return factory
}

func (factory *MockClusterFactory) AddShardingSpecV2(shardingName string, compDefName string) *MockClusterFactory {
	shardingSpec := appsv1alpha1.ShardingSpec{
		Template: appsv1alpha1.ClusterComponentSpec{
			Name:         "fake",
			ComponentDef: compDefName,
			Replicas:     1,
		},
		Name:   shardingName,
		Shards: 1,
	}
	factory.Get().Spec.ShardingSpecs = append(factory.Get().Spec.ShardingSpecs, shardingSpec)
	return factory
}

func (factory *MockClusterFactory) AddComponent(compName string, compDefName string) *MockClusterFactory {
	comp := appsv1alpha1.ClusterComponentSpec{
		Name:            compName,
		ComponentDefRef: compDefName,
	}
	factory.Get().Spec.ComponentSpecs = append(factory.Get().Spec.ComponentSpecs, comp)
	return factory
}

func (factory *MockClusterFactory) AddComponentV2(compName string, compDefName string) *MockClusterFactory {
	comp := appsv1alpha1.ClusterComponentSpec{
		Name:         compName,
		ComponentDef: compDefName,
	}
	factory.Get().Spec.ComponentSpecs = append(factory.Get().Spec.ComponentSpecs, comp)
	return factory
}

func (factory *MockClusterFactory) AddMultipleTemplateComponent(compName string, compDefName string) *MockClusterFactory {
	comp := appsv1alpha1.ClusterComponentSpec{
		Name:         compName,
		ComponentDef: compDefName,
		Instances: []appsv1alpha1.InstanceTemplate{{
			Name:     "foo",
			Replicas: func() *int32 { replicas := int32(1); return &replicas }(),
		}},
	}
	factory.Get().Spec.ComponentSpecs = append(factory.Get().Spec.ComponentSpecs, comp)
	return factory
}

func (factory *MockClusterFactory) AddService(service appsv1alpha1.ClusterService) *MockClusterFactory {
	services := factory.Get().Spec.Services
	if len(services) == 0 {
		services = []appsv1alpha1.ClusterService{}
	}
	services = append(services, service)
	factory.Get().Spec.Services = services
	return factory
}

type updateFn func(comp *appsv1alpha1.ClusterComponentSpec)

type shardingUpdateFn func(shardingSpec *appsv1alpha1.ShardingSpec)

func (factory *MockClusterFactory) lastComponentRef(update updateFn) *MockClusterFactory {
	comps := factory.Get().Spec.ComponentSpecs
	if len(comps) > 0 {
		update(&comps[len(comps)-1])
	}
	factory.Get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) lastShardingSpec(update shardingUpdateFn) *MockClusterFactory {
	shardingSpecs := factory.Get().Spec.ShardingSpecs
	if len(shardingSpecs) > 0 {
		update(&shardingSpecs[len(shardingSpecs)-1])
	}
	factory.Get().Spec.ShardingSpecs = shardingSpecs
	return factory
}

func (factory *MockClusterFactory) SetShards(shards int32) *MockClusterFactory {
	return factory.lastShardingSpec(func(shardingSpec *appsv1alpha1.ShardingSpec) {
		shardingSpec.Shards = shards
	})
}

func (factory *MockClusterFactory) SetCompDef(compDef string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.ComponentDef = compDef
	})
}

func (factory *MockClusterFactory) SetServiceVersion(serviceVersion string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.ServiceVersion = serviceVersion
	})
}

func (factory *MockClusterFactory) SetReplicas(replicas int32) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Replicas = replicas
	})
}

func (factory *MockClusterFactory) SetServiceAccountName(serviceAccountName string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.ServiceAccountName = serviceAccountName
	})
}

func (factory *MockClusterFactory) SetResources(resources corev1.ResourceRequirements) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Resources = resources
	})
}

func (factory *MockClusterFactory) SetComponentAffinity(affinity *appsv1alpha1.Affinity) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Affinity = affinity
	})
}

func (factory *MockClusterFactory) SetEnabledLogs(logName ...string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.EnabledLogs = logName
	})
}

func (factory *MockClusterFactory) AddComponentToleration(toleration corev1.Toleration) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		tolerations := comp.Tolerations
		if len(tolerations) == 0 {
			tolerations = []corev1.Toleration{}
		}
		tolerations = append(tolerations, toleration)
		comp.Tolerations = tolerations
	})
}

func (factory *MockClusterFactory) AddVolumeClaimTemplate(volumeName string,
	pvcSpec appsv1alpha1.PersistentVolumeClaimSpec) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.VolumeClaimTemplates = append(comp.VolumeClaimTemplates,
			appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				Name: volumeName,
				Spec: pvcSpec,
			})
	})
}

// func (factory *MockClusterFactory) SetMonitor(monitor bool) *MockClusterFactory {
// 	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
// 		comp.Monitor = monitor
// 	})
// }

func (factory *MockClusterFactory) SetSwitchPolicy(switchPolicy *appsv1alpha1.ClusterSwitchPolicy) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.SwitchPolicy = switchPolicy
	})
}

func (factory *MockClusterFactory) SetTLS(tls bool) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.TLS = tls
	})
}

func (factory *MockClusterFactory) SetIssuer(issuer *appsv1alpha1.Issuer) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Issuer = issuer
	})
}

func (factory *MockClusterFactory) AddComponentService(serviceName string, serviceType corev1.ServiceType) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Services = append(comp.Services,
			appsv1alpha1.ClusterComponentService{
				Name:        serviceName,
				ServiceType: serviceType,
			})
	})
}

func (factory *MockClusterFactory) AddSystemAccount(name string, passwordConfig *appsv1alpha1.PasswordConfig, secretRef *appsv1alpha1.ProvisionSecretRef) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.SystemAccounts = append(comp.SystemAccounts,
			appsv1alpha1.ComponentSystemAccount{
				Name:           name,
				PasswordConfig: passwordConfig,
				SecretRef:      secretRef,
			})
	})
}

func (factory *MockClusterFactory) SetBackup(backup *appsv1alpha1.ClusterBackup) *MockClusterFactory {
	factory.Get().Spec.Backup = backup
	return factory
}

func (factory *MockClusterFactory) SetServiceRefs(serviceRefs []appsv1alpha1.ServiceRef) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		comp.ServiceRefs = serviceRefs
	})
}

func (factory *MockClusterFactory) AddUserSecretVolume(name, mountPoint, resName, containerName string) *MockClusterFactory {
	secretResource := appsv1alpha1.SecretRef{
		ResourceMeta: appsv1alpha1.ResourceMeta{
			Name:         name,
			MountPoint:   mountPoint,
			AsVolumeFrom: []string{containerName},
		},
		Secret: corev1.SecretVolumeSource{
			SecretName: resName,
		},
	}
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		userResourcesRefs := comp.UserResourceRefs
		if userResourcesRefs == nil {
			userResourcesRefs = &appsv1alpha1.UserResourceRefs{}
			comp.UserResourceRefs = userResourcesRefs
		}
		userResourcesRefs.SecretRefs = append(userResourcesRefs.SecretRefs, secretResource)
	})
}

func (factory *MockClusterFactory) AddUserConfigmapVolume(name, mountPoint, resName, containerName string) *MockClusterFactory {
	cmResource := appsv1alpha1.ConfigMapRef{
		ResourceMeta: appsv1alpha1.ResourceMeta{
			Name:         name,
			MountPoint:   mountPoint,
			AsVolumeFrom: []string{containerName},
		},
		ConfigMap: corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: resName,
			},
		},
	}
	return factory.lastComponentRef(func(comp *appsv1alpha1.ClusterComponentSpec) {
		userResourcesRefs := comp.UserResourceRefs
		if userResourcesRefs == nil {
			userResourcesRefs = &appsv1alpha1.UserResourceRefs{}
			comp.UserResourceRefs = userResourcesRefs
		}
		userResourcesRefs.ConfigMapRefs = append(userResourcesRefs.ConfigMapRefs, cmResource)
	})
}
