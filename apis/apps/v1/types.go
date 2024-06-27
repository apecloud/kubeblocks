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
	corev1 "k8s.io/api/core/v1"
)

// Phase represents the status of a CR.
//
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	// AvailablePhase indicates that a CR is in an available state.
	AvailablePhase Phase = "Available"

	// UnavailablePhase indicates that a CR is in an unavailable state.
	UnavailablePhase Phase = "Unavailable"
)

// LetterCase defines the available cases to be used in password.
//
// +enum
// +kubebuilder:validation:Enum={LowerCases,UpperCases,MixedCases}
type LetterCase string

const (
	// LowerCases represents the use of lower case letters only.
	LowerCases LetterCase = "LowerCases"

	// UpperCases represents the use of upper case letters only.
	UpperCases LetterCase = "UpperCases"

	// MixedCases represents the use of a mix of both lower and upper case letters.
	MixedCases LetterCase = "MixedCases"
)

type Service struct {
	// Name defines the name of the service.
	// otherwise, it indicates the name of the service.
	// Others can refer to this service by its name. (e.g., connection credential)
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=25
	Name string `json:"name"`

	// ServiceName defines the name of the underlying service object.
	// If not specified, the default service name with different patterns will be used:
	//
	// - CLUSTER_NAME: for cluster-level services
	// - CLUSTER_NAME-COMPONENT_NAME: for component-level services
	//
	// Only one default service name is allowed.
	// Cannot be updated.
	//
	// +kubebuilder:validation:MaxLength=25
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	//
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// If ServiceType is LoadBalancer, cloud provider related parameters can be put here
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Spec defines the behavior of a service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	//
	// +optional
	Spec corev1.ServiceSpec `json:"spec,omitempty"`

	// Extends the above `serviceSpec.selector` by allowing you to specify defined role as selector for the service.
	// When `roleSelector` is set, it adds a label selector "kubeblocks.io/role: {roleSelector}"
	// to the `serviceSpec.selector`.
	// Example usage:
	//
	//	  roleSelector: "leader"
	//
	// In this example, setting `roleSelector` to "leader" will add a label selector
	// "kubeblocks.io/role: leader" to the `serviceSpec.selector`.
	// This means that the service will select and route traffic to Pods with the label
	// "kubeblocks.io/role" set to "leader".
	//
	// Note that if `podService` sets to true, RoleSelector will be ignored.
	// The `podService` flag takes precedence over `roleSelector` and generates a service for each Pod.
	//
	// +optional
	RoleSelector string `json:"roleSelector,omitempty"`
}

type SchedulingPolicy struct {
	// If specified, the Pod will be dispatched by specified scheduler.
	// If not specified, the Pod will be dispatched by default scheduler.
	//
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// NodeSelector is a selector which must be true for the Pod to fit on a node.
	// Selector which must match a node's labels for the Pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	//
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// NodeName is a request to schedule this Pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this Pod onto that node, assuming that it fits resource
	// requirements.
	//
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Specifies a group of affinity scheduling rules of the Cluster, including NodeAffinity, PodAffinity, and PodAntiAffinity.
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Allows Pods to be scheduled onto nodes with matching taints.
	// Each toleration in the array allows the Pod to tolerate node taints based on
	// specified `key`, `value`, `effect`, and `operator`.
	//
	// - The `key`, `value`, and `effect` identify the taint that the toleration matches.
	// - The `operator` determines how the toleration matches the taint.
	//
	// Pods with matching tolerations are allowed to be scheduled on tainted nodes, typically reserved for specific purposes.
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints describes how a group of Pods ought to spread across topology
	// domains. Scheduler will schedule Pods in a way which abides by the constraints.
	// All topologySpreadConstraints are ANDed.
	//
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

type TLSConfig struct {
	// A boolean flag that indicates whether the Component should use Transport Layer Security (TLS)
	// for secure communication.
	// When set to true, the Component will be configured to use TLS encryption for its network connections.
	// This ensures that the data transmitted between the Component and its clients or other Components is encrypted
	// and protected from unauthorized access.
	// If TLS is enabled, the Component may require additional configuration,
	// such as specifying TLS certificates and keys, to properly set up the secure communication channel.
	//
	// +kubebuilder:default=false
	// +optional
	Enable bool `json:"enable,omitempty"`

	// Specifies the configuration for the TLS certificates issuer.
	// It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
	// The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
	// Required when TLS is enabled.
	//
	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`
}

// Issuer defines the TLS certificates issuer for the Cluster.
type Issuer struct {
	// The issuer for TLS certificates.
	// It only allows two enum values: `KubeBlocks` and `UserProvided`.
	//
	// - `KubeBlocks` indicates that the self-signed TLS certificates generated by the KubeBlocks Operator will be used.
	// - `UserProvided` means that the user is responsible for providing their own CA, Cert, and Key.
	//   In this case, the user-provided CA certificate, server certificate, and private key will be used
	//   for TLS communication.
	//
	// +kubebuilder:validation:Enum={KubeBlocks, UserProvided}
	// +kubebuilder:default=KubeBlocks
	// +kubebuilder:validation:Required
	Name IssuerName `json:"name"`

	// SecretRef is the reference to the secret that contains user-provided certificates.
	// It is required when the issuer is set to `UserProvided`.
	//
	// +optional
	SecretRef *TLSSecretRef `json:"secretRef,omitempty"`
}

// IssuerName defines the name of the TLS certificates issuer.
// +enum
// +kubebuilder:validation:Enum={KubeBlocks,UserProvided}
type IssuerName string

const (
	// IssuerKubeBlocks represents certificates that are signed by the KubeBlocks Operator.
	IssuerKubeBlocks IssuerName = "KubeBlocks"

	// IssuerUserProvided indicates that the user has provided their own CA-signed certificates.
	IssuerUserProvided IssuerName = "UserProvided"
)

// TLSSecretRef defines Secret contains Tls certs
type TLSSecretRef struct {
	// Name of the Secret that contains user-provided certificates.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key of CA cert in Secret
	// +kubebuilder:validation:Required
	CA string `json:"ca"`

	// Key of Cert in Secret
	// +kubebuilder:validation:Required
	Cert string `json:"cert"`

	// Key of TLS private key in Secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// InstanceTemplate allows customization of individual replica configurations in a Component.
type InstanceTemplate struct {
	// Name specifies the unique name of the instance Pod created using this InstanceTemplate.
	// This name is constructed by concatenating the Component's name, the template's name, and the instance's ordinal
	// using the pattern: $(cluster.name)-$(component.name)-$(template.name)-$(ordinal). Ordinals start from 0.
	// The specified name overrides any default naming conventions or patterns.
	//
	// +kubebuilder:validation:MaxLength=54
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the number of instances (Pods) to create from this InstanceTemplate.
	// This field allows setting how many replicated instances of the Component,
	// with the specific overrides in the InstanceTemplate, are created.
	// The default value is 1. A value of 0 disables instance creation.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies a map of key-value pairs to be merged into the Pod's existing annotations.
	// Existing keys will have their values overwritten, while new keys will be added to the annotations.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specifies a map of key-value pairs that will be merged into the Pod's existing labels.
	// Values for existing keys will be overwritten, and new keys will be added.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies an override for the first container's image in the Pod.
	//
	// +optional
	Image *string `json:"image,omitempty"`

	// Specifies the scheduling policy for the Component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies an override for the resource requirements of the first container in the Pod.
	// This field allows for customizing resource allocation (CPU, memory, etc.) for the container.
	//
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Defines Env to override.
	// Add new or override existing envs.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Defines Volumes to override.
	// Add new or override existing volumes.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Defines VolumeMounts to override.
	// Add new or override existing volume mounts of the first container in the Pod.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Defines VolumeClaimTemplates to override.
	// Add new or override existing volume claim templates.
	// +optional
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
}
