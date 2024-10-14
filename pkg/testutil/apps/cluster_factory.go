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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type MockClusterFactory struct {
	BaseFactory[appsv1.Cluster, *appsv1.Cluster, MockClusterFactory]
}

func NewClusterFactory(namespace, name, clusterDef string) *MockClusterFactory {
	f := &MockClusterFactory{}
	f.Init(namespace, name,
		&appsv1.Cluster{
			Spec: appsv1.ClusterSpec{
				ClusterDef:        clusterDef,
				ComponentSpecs:    []appsv1.ClusterComponentSpec{},
				TerminationPolicy: appsv1.WipeOut,
			},
		}, f)
	return f
}

func (factory *MockClusterFactory) SetTopology(topology string) *MockClusterFactory {
	factory.Get().Spec.Topology = topology
	return factory
}

func (factory *MockClusterFactory) SetTerminationPolicy(policyType appsv1.TerminationPolicyType) *MockClusterFactory {
	factory.Get().Spec.TerminationPolicy = policyType
	return factory
}

func (factory *MockClusterFactory) SetSchedulingPolicy(schedulingPolicy *appsv1.SchedulingPolicy) *MockClusterFactory {
	factory.Get().Spec.SchedulingPolicy = schedulingPolicy
	return factory
}

func (factory *MockClusterFactory) AddSharding(shardingName string, compDefName string) *MockClusterFactory {
	sharding := appsv1.ClusterSharding{
		Template: appsv1.ClusterComponentSpec{
			Name:         "fake",
			ComponentDef: compDefName,
			Replicas:     1,
		},
		Name:   shardingName,
		Shards: 1,
	}
	factory.Get().Spec.Shardings = append(factory.Get().Spec.Shardings, sharding)
	return factory
}

func (factory *MockClusterFactory) AddComponent(compName string, compDefName string) *MockClusterFactory {
	comp := appsv1.ClusterComponentSpec{
		Name:         compName,
		ComponentDef: compDefName,
	}
	factory.Get().Spec.ComponentSpecs = append(factory.Get().Spec.ComponentSpecs, comp)
	return factory
}

func (factory *MockClusterFactory) AddMultipleTemplateComponent(compName string, compDefName string) *MockClusterFactory {
	comp := appsv1.ClusterComponentSpec{
		Name:         compName,
		ComponentDef: compDefName,
		Instances: []appsv1.InstanceTemplate{{
			Name:     "foo",
			Replicas: func() *int32 { replicas := int32(1); return &replicas }(),
		}},
	}
	factory.Get().Spec.ComponentSpecs = append(factory.Get().Spec.ComponentSpecs, comp)
	return factory
}

func (factory *MockClusterFactory) AddService(service appsv1.ClusterService) *MockClusterFactory {
	services := factory.Get().Spec.Services
	if len(services) == 0 {
		services = []appsv1.ClusterService{}
	}
	services = append(services, service)
	factory.Get().Spec.Services = services
	return factory
}

type updateFn func(comp *appsv1.ClusterComponentSpec)

type shardingUpdateFn func(*appsv1.ClusterSharding)

func (factory *MockClusterFactory) lastComponentRef(update updateFn) *MockClusterFactory {
	comps := factory.Get().Spec.ComponentSpecs
	if len(comps) > 0 {
		update(&comps[len(comps)-1])
	}
	factory.Get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) lastSharding(update shardingUpdateFn) *MockClusterFactory {
	shardings := factory.Get().Spec.Shardings
	if len(shardings) > 0 {
		update(&shardings[len(shardings)-1])
	}
	factory.Get().Spec.Shardings = shardings
	return factory
}

func (factory *MockClusterFactory) SetShards(shards int32) *MockClusterFactory {
	return factory.lastSharding(func(sharding *appsv1.ClusterSharding) {
		sharding.Shards = shards
	})
}

func (factory *MockClusterFactory) SetCompDef(compDef string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.ComponentDef = compDef
	})
}

func (factory *MockClusterFactory) SetServiceVersion(serviceVersion string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.ServiceVersion = serviceVersion
	})
}

func (factory *MockClusterFactory) SetReplicas(replicas int32) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.Replicas = replicas
	})
}

func (factory *MockClusterFactory) SetServiceAccountName(serviceAccountName string) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.ServiceAccountName = serviceAccountName
	})
}

func (factory *MockClusterFactory) SetResources(resources corev1.ResourceRequirements) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.Resources = resources
	})
}

func (factory *MockClusterFactory) AddVolumeClaimTemplate(volumeName string,
	pvcSpec appsv1.PersistentVolumeClaimSpec) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.VolumeClaimTemplates = append(comp.VolumeClaimTemplates,
			appsv1.ClusterComponentVolumeClaimTemplate{
				Name: volumeName,
				Spec: pvcSpec,
			})
	})
}

func (factory *MockClusterFactory) SetTLS(tls bool) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.TLS = tls
	})
}

func (factory *MockClusterFactory) SetIssuer(issuer *appsv1.Issuer) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.Issuer = issuer
	})
}

func (factory *MockClusterFactory) AddComponentService(serviceName string, serviceType corev1.ServiceType) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.Services = append(comp.Services,
			appsv1.ClusterComponentService{
				Name:        serviceName,
				ServiceType: serviceType,
			})
	})
}

func (factory *MockClusterFactory) AddSystemAccount(name string, passwordConfig *appsv1.PasswordConfig, secretRef *appsv1.ProvisionSecretRef) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.SystemAccounts = append(comp.SystemAccounts,
			appsv1.ComponentSystemAccount{
				Name:           name,
				PasswordConfig: passwordConfig,
				SecretRef:      secretRef,
			})
	})
}

func (factory *MockClusterFactory) SetBackup(backup *appsv1.ClusterBackup) *MockClusterFactory {
	factory.Get().Spec.Backup = backup
	return factory
}

func (factory *MockClusterFactory) SetServiceRefs(serviceRefs []appsv1.ServiceRef) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.ServiceRefs = serviceRefs
	})
}

func (factory *MockClusterFactory) SetStop(stop *bool) *MockClusterFactory {
	return factory.lastComponentRef(func(comp *appsv1.ClusterComponentSpec) {
		comp.Stop = stop
	})
}
