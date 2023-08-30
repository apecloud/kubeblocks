/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceConnectionCredentialSpec defines the desired state of ServiceConnectionCredential
type ServiceConnectionCredentialSpec struct {
	// endpoint is the endpoint of the service connection credential.
	// +optional
	Endpoint *CredentialVar `json:"endpoint,omitempty"`

	// auth is the auth of the service connection credential.
	// +optional
	Auth *ConnectionCredentialAuth `json:"auth,omitempty"`

	// port is the port of the service connection credential.
	// +optional
	Port *CredentialVar `json:"port,omitempty" protobuf:"bytes,4,opt,name=port"`

	// extra is the extra information of the service connection credential, and it is a key-value pair.
	// +optional
	Extra map[string]string `json:"extra,omitempty"`
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

// ServiceConnectionCredentialStatus defines the observed state of ServiceConnectionCredential
type ServiceConnectionCredentialStatus struct {
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

func (r ServiceConnectionCredentialStatus) GetTerminalPhases() []Phase {
	return []Phase{AvailablePhase}
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=scc
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ServiceConnectionCredential is the Schema for the serviceconnectioncredentials API
type ServiceConnectionCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceConnectionCredentialSpec   `json:"spec,omitempty"`
	Status ServiceConnectionCredentialStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceConnectionCredentialList contains a list of ServiceConnectionCredential
type ServiceConnectionCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceConnectionCredential `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceConnectionCredential{}, &ServiceConnectionCredentialList{})
}
