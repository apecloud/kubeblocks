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
	// service kind, indicating the type or nature of the service. It should be well-known application cluster type, e.g. {mysql, redis, mongodb}.
	// The serviceKind is case-insensitive and supports abbreviations for some well-known databases.
	// For example, both `zk` and `zookeeper` will be considered as a ZooKeeper cluster, and `pg`, `postgres`, `postgresql` will all be considered as a PostgreSQL cluster.
	//
	// +kubebuilder:validation:Required
	ServiceKind string `json:"serviceKind"`

	// The version of the service reference.
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`

	// endpoint is the endpoint of the service connection credential.
	// +optional
	Endpoint *CredentialVar `json:"endpoint,omitempty"`

	// auth is the auth of the service connection credential.
	// +optional
	Auth *ConnectionCredentialAuth `json:"auth,omitempty"`

	// port is the port of the service connection credential.
	// +optional
	Port *CredentialVar `json:"port,omitempty" protobuf:"bytes,4,opt,name=port"`
}

type ConnectionCredentialAuth struct {
	// service connection based-on username and password credential.
	// +optional
	Username *CredentialVar `json:"username,omitempty"`

	// service connection based-on username and password credential.
	// +optional
	Password *CredentialVar `json:"password,omitempty"`
}

type CredentialVar struct {
	// Optional: no more than one of the following may be specified.

	// Variable references $(VAR_NAME) are expanded
	// using the previously defined environment variables in the container and
	// any service environment variables. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.
	// "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".
	// Escaped references will never be expanded, regardless of whether the variable
	// exists or not.
	// Defaults to "".
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// Source for the environment variable's value. Cannot be used if value is not empty.
	// +optional
	ValueFrom *corev1.EnvVarSource `json:"valueFrom,omitempty" protobuf:"bytes,3,opt,name=valueFrom"`
}

// ServiceDescriptorStatus defines the observed state of ServiceDescriptor
type ServiceDescriptorStatus struct {
	// phase - in list of [Available,Unavailable]
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// A human-readable message indicating details about why the ServiceConnectionCredential is in this phase.
	// +optional
	Message string `json:"message,omitempty"`

	// generation number
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
