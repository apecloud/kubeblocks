/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDescriptorSpec defines the desired state of ServiceDescriptor.
type ServiceDescriptorSpec struct {
	// Describes the type of database service provided by the external service.
	// For example, "mysql", "redis", "mongodb".
	// This field categorizes databases by their functionality, protocol and compatibility, facilitating appropriate
	// service integration based on their unique capabilities.
	//
	// This field is case-insensitive.
	//
	// It also supports abbreviations for some well-known databases:
	// - "pg", "pgsql", "postgres", "postgresql": PostgreSQL service
	// - "zk", "zookeeper": ZooKeeper service
	// - "es", "elasticsearch": Elasticsearch service
	// - "mongo", "mongodb": MongoDB service
	// - "ch", "clickhouse": ClickHouse service
	//
	// +kubebuilder:validation:Required
	ServiceKind string `json:"serviceKind"`

	// Describes the version of the service provided by the external service.
	// This is crucial for ensuring compatibility between different components of the system,
	// as different versions of a service may have varying features.
	//
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`

	// Specifies the endpoint of the external service.
	//
	// If the service is exposed via a cluster, the endpoint will be provided in the format of <host>:<port>.
	//
	// +optional
	Endpoint *CredentialVar `json:"endpoint,omitempty"`

	// Specifies the service or IP address of the external service.
	//
	// +optional
	Host *CredentialVar `json:"host,omitempty"`

	// Specifies the port of the external service.
	//
	// +optional
	Port *CredentialVar `json:"port,omitempty"`

	// Specifies the authentication credentials required for accessing an external service.
	//
	// +optional
	Auth *ConnectionCredentialAuth `json:"auth,omitempty"`
}

// ConnectionCredentialAuth specifies the authentication credentials required for accessing an external service.
type ConnectionCredentialAuth struct {
	// Specifies the username for the external service.
	//
	// +optional
	Username *CredentialVar `json:"username,omitempty"`

	// Specifies the password for the external service.
	//
	// +optional
	Password *CredentialVar `json:"password,omitempty"`
}

// CredentialVar represents a variable that retrieves its value either directly from a specified expression
// or from a source defined in `valueFrom`.
// Only one of these options may be used at a time.
type CredentialVar struct {
	// Holds a direct string or an expression that can be evaluated to a string.
	//
	// It can include variables denoted by $(VAR_NAME).
	// These variables are expanded to the value of the environment variables defined in the container.
	// If a variable cannot be resolved, it remains unchanged in the output.
	//
	// To escape variable expansion and retain the literal value, use double $ characters.
	//
	// For example:
	//
	// - "$(VAR_NAME)" will be expanded to the value of the environment variable VAR_NAME.
	// - "$$(VAR_NAME)" will result in "$(VAR_NAME)" in the output, without any variable expansion.
	//
	// Default value is an empty string.
	//
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`

	// Specifies the source for the variable's value.
	//
	// +optional
	ValueFrom *corev1.EnvVarSource `json:"valueFrom,omitempty" protobuf:"bytes,3,opt,name=valueFrom"`
}

// ServiceDescriptorStatus defines the observed state of ServiceDescriptor
type ServiceDescriptorStatus struct {
	// Indicates the current lifecycle phase of the ServiceDescriptor. This can be either 'Available' or 'Unavailable'.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides a human-readable explanation detailing the reason for the current phase of the ServiceConnectionCredential.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Represents the generation number that has been processed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

func (r ServiceDescriptorStatus) GetTerminalPhases() []Phase {
	return []Phase{AvailablePhase}
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=sd
// +kubebuilder:printcolumn:name="SERVICE_KIND",type="string",JSONPath=".spec.serviceKind",description="service kind"
// +kubebuilder:printcolumn:name="SERVICE_VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ServiceDescriptor describes a service provided by external sources.
// It contains the necessary details such as the service's address and connection credentials.
// To enable a Cluster to access this service, the ServiceDescriptor's name should be specified
// in the Cluster configuration under `clusterComponent.serviceRefs[*].serviceDescriptor`.
type ServiceDescriptor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceDescriptorSpec   `json:"spec,omitempty"`
	Status ServiceDescriptorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceDescriptorList contains a list of ServiceDescriptor.
type ServiceDescriptorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceDescriptor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceDescriptor{}, &ServiceDescriptorList{})
}
