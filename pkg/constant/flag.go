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

package constant

const (
	EnableRBACManager = "EnableRBACManager"

	ManagedNamespacesFlag = "managed-namespaces"
)

const (
	// EnabledNodePortSvcAnnotationKey defines the feature gate of NodePort Service defined in ComponentDefinition.Spec.Services.
	// Components defined in the annotation value, their all services of type NodePort defined in ComponentDefinition will be created; otherwise, they will be ignored.
	// Multiple components are separated by ','. for example: "kubeblocks.io/enabled-node-port-svc: comp1,comp2"
	EnabledNodePortSvcAnnotationKey = "kubeblocks.io/enabled-node-port-svc"

	// EnabledPodOrdinalSvcAnnotationKey defines the feature gate of PodOrdinal Service defined in ComponentDefinition.Spec.Services.
	// Components defined in the annotation value, their all Services defined in the ComponentDefinition with the GeneratePodOrdinalService attribute set to true will be created; otherwise, they will be ignored.
	// This can generate a corresponding Service for each Pod, which can be used in certain specific scenarios: for example, creating a dedicated access service for each read-only Pod.
	// Multiple components are separated by ','. for example: "kubeblocks.io/enabled-pod-ordinal-svc: comp1,comp2"
	EnabledPodOrdinalSvcAnnotationKey = "kubeblocks.io/enabled-pod-ordinal-svc"

	// DisabledClusterIPSvcAnnotationKey defines whether the feature gate of ClusterIp Service defined in ComponentDefinition.Spec.Services will be effected.
	// Components defined in the annotation value, their all services of type ClusterIp defined in ComponentDefinition will be ignored; otherwise, they will be created.
	// Multiple components are separated by ','. for example: "kubeblocks.io/disabled-cluster-ip-svc: comp1,comp2"
	DisabledClusterIPSvcAnnotationKey = "kubeblocks.io/disabled-cluster-ip-svc"

	// ShardSvcAnnotationKey defines the feature gate of creating service for each shard.
	// Sharding name defined in the annotation value, a set of Service defined in Cluster.Spec.Services with the ShardingSelector will be automatically generated for each shard when Cluster.Spec.ShardingSpecs[x].shards is not nil.
	// Multiple sharding names are separated by ','. for example: "kubeblocks.io/enabled-shard-svc: proxy-shard,db-shard"
	ShardSvcAnnotationKey = "kubeblocks.io/enabled-shard-svc"

	// HostNetworkAnnotationKey defines the feature gate to enable the host-network for specified components or shardings.
	HostNetworkAnnotationKey = "kubeblocks.io/host-network"
)
