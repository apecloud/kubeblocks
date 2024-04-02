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

// ServiceDescriptorSpec defines the desired state of ServiceDescriptor
type ServiceDescriptorSpec struct {
	// Specifies the type or nature of the service. Should represent a well-known application cluster type, such as {mysql, redis, mongodb}.
	// This field is case-insensitive and supports abbreviations for some well-known databases.
	// For instance, both `zk` and `zookeeper` will be recognized as a ZooKeeper cluster, and `pg`, `postgres`, `postgresql` will all be recognized as a PostgreSQL cluster.
	//
	// +kubebuilder:validation:Required
	ServiceKind string `json:"serviceKind"`

	// Represents the version of the service reference.
	//
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`

	// Represents the endpoint of the service connection credential.
	//
	// +optional
	Endpoint *CredentialVar `json:"endpoint,omitempty"`

	// Represents the authentication details of the service connection credential.
	//
	// +optional
	Auth *ConnectionCredentialAuth `json:"auth,omitempty"`

	// Represents the port of the service connection credential.
	//
	// +optional
	Port *CredentialVar `json:"port,omitempty" protobuf:"bytes,4,opt,name=port"`
}

// ConnectionCredentialAuth represents the authentication details of the service connection credential.
type ConnectionCredentialAuth struct {
	// Represents the username credential for the service connection.
	//
	// +optional
	Username *CredentialVar `json:"username,omitempty"`

	// Represents the password credential for the service connection.
	//
	// +optional
	Password *CredentialVar `json:"password,omitempty"`
}

// CredentialVar defines the value of credential variable.
type CredentialVar struct {
	// Specifies an optional variable. Only one of the following may be specified.
	// Variable references, denoted by $(VAR_NAME), are expanded using previously defined
	// environment variables in the container and any service environment variables.
	// If a variable cannot be resolved, the reference in the input string remains unchanged.
	//
	// Double $$ are reduced to a single $, enabling the escaping of the $(VAR_NAME) syntax.
	// For instance, "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".
	// Escaped references will never be expanded, irrespective of the variable's existence.
	// The default value is "".
	//
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`

	// Defines the source for the environment variable's value. This cannot be used if the value is not empty.
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

// ServiceDescriptor is the Schema for the servicedescriptors API
type ServiceDescriptor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceDescriptorSpec   `json:"spec,omitempty"`
	Status ServiceDescriptorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceDescriptorList contains a list of ServiceDescriptor
type ServiceDescriptorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceDescriptor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceDescriptor{}, &ServiceDescriptorList{})
}
