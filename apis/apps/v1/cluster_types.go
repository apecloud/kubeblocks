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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:categories={kubeblocks,all}
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.clusterVersionRef",description="Cluster Application Version."
// +kubebuilder:printcolumn:name="TERMINATION-POLICY",type="string",JSONPath=".spec.terminationPolicy",description="Cluster termination policy."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Cluster Status."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster offers a unified management interface for a wide variety of database and storage systems:
//
// - Relational databases: MySQL, PostgreSQL, MariaDB
// - NoSQL databases: Redis, MongoDB
// - KV stores: ZooKeeper, etcd
// - Analytics systems: ElasticSearch, OpenSearch, ClickHouse, Doris, StarRocks, Solr
// - Message queues: Kafka, Pulsar
// - Distributed SQL: TiDB, OceanBase
// - Vector databases: Qdrant, Milvus, Weaviate
// - Object storage: Minio
//
// KubeBlocks utilizes an abstraction layer to encapsulate the characteristics of these diverse systems.
// A Cluster is composed of multiple Components, each defined by vendors or KubeBlocks Addon developers via ComponentDefinition,
// arranged in Directed Acyclic Graph (DAG) topologies.
// The topologies, defined in a ClusterDefinition, coordinate reconciliation across Cluster's lifecycle phases:
// Creating, Running, Updating, Stopping, Stopped, Deleting.
// Lifecycle management ensures that each Component operates in harmony, executing appropriate actions at each lifecycle stage.
//
// For sharded-nothing architecture, the Cluster supports managing multiple shards,
// each shard managed by a separate Component, supporting dynamic resharding.
//
// The Cluster object is aimed to maintain the overall integrity and availability of a database cluster,
// serves as the central control point, abstracting the complexity of multiple-component management,
// and providing a unified interface for cluster-wide operations.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Cluster. Edit cluster_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}
