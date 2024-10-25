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

package v1

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	APIVersion            = "apps.kubeblocks.io/v1"
	ClusterDefinitionKind = "ClusterDefinition"
	ClusterKind           = "Cluster"
	ComponentKind         = "Component"
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

// ClusterComponentPhase defines the phase of a cluster component as represented in cluster.status.components.phase field.
//
// +enum
// +kubebuilder:validation:Enum={Creating,Running,Updating,Stopping,Stopped,Deleting,Failed,Abnormal}
type ClusterComponentPhase string

const (
	// CreatingClusterCompPhase indicates the component is being created.
	CreatingClusterCompPhase ClusterComponentPhase = "Creating"

	// RunningClusterCompPhase indicates the component has more than zero replicas, and all pods are up-to-date and
	// in a 'Running' state.
	RunningClusterCompPhase ClusterComponentPhase = "Running"

	// UpdatingClusterCompPhase indicates the component has more than zero replicas, and there are no failed pods,
	// it is currently being updated.
	UpdatingClusterCompPhase ClusterComponentPhase = "Updating"

	// StoppingClusterCompPhase indicates the component has zero replicas, and there are pods that are terminating.
	StoppingClusterCompPhase ClusterComponentPhase = "Stopping"

	// StoppedClusterCompPhase indicates the component has zero replicas, and all pods have been deleted.
	StoppedClusterCompPhase ClusterComponentPhase = "Stopped"

	// DeletingClusterCompPhase indicates the component is currently being deleted.
	DeletingClusterCompPhase ClusterComponentPhase = "Deleting"

	// FailedClusterCompPhase indicates the component has more than zero replicas, but there are some failed pods.
	// The component is not functioning.
	FailedClusterCompPhase ClusterComponentPhase = "Failed"

	// AbnormalClusterCompPhase indicates the component has more than zero replicas, but there are some failed pods.
	// The component is functioning, but it is in a fragile state.
	AbnormalClusterCompPhase ClusterComponentPhase = "Abnormal"
)

const (
	ConditionTypeProvisioningStarted = "ProvisioningStarted" // ConditionTypeProvisioningStarted the operator starts resource provisioning to create or change the cluster
	ConditionTypeApplyResources      = "ApplyResources"      // ConditionTypeApplyResources the operator start to apply resources to create or change the cluster
	ConditionTypeReady               = "Ready"               // ConditionTypeReady all components and shardings are running
	ConditionTypeAvailable           = "Available"           // ConditionTypeAvailable indicates whether the target object is available for serving.
)

type ServiceRef struct {
	// Specifies the identifier of the service reference declaration.
	// It corresponds to the serviceRefDeclaration name defined in either:
	//
	// - `componentDefinition.spec.serviceRefDeclarations[*].name`
	// - `clusterDefinition.spec.componentDefs[*].serviceRefDeclarations[*].name` (deprecated)
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the namespace of the referenced Cluster or the namespace of the referenced ServiceDescriptor object.
	// If not provided, the referenced Cluster and ServiceDescriptor will be searched in the namespace of the current
	// Cluster by default.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Specifies the name of the KubeBlocks Cluster being referenced.
	// This is used when services from another KubeBlocks Cluster are consumed.
	//
	// By default, the referenced KubeBlocks Cluster's `clusterDefinition.spec.connectionCredential`
	// will be utilized to bind to the current Component. This credential should include:
	// `endpoint`, `port`, `username`, and `password`.
	//
	// Note:
	//
	// - The `ServiceKind` and `ServiceVersion` specified in the service reference within the
	//   ClusterDefinition are not validated when using this approach.
	// - If both `cluster` and `serviceDescriptor` are present, `cluster` will take precedence.
	//
	// Deprecated since v0.9 since `clusterDefinition.spec.connectionCredential` is deprecated,
	// use `clusterServiceSelector` instead.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// References a service provided by another KubeBlocks Cluster.
	// It specifies the ClusterService and the account credentials needed for access.
	//
	// +optional
	ClusterServiceSelector *ServiceRefClusterSelector `json:"clusterServiceSelector,omitempty"`

	// Specifies the name of the ServiceDescriptor object that describes a service provided by external sources.
	//
	// When referencing a service provided by external sources, a ServiceDescriptor object is required to establish
	// the service binding.
	// The `serviceDescriptor.spec.serviceKind` and `serviceDescriptor.spec.serviceVersion` should match the serviceKind
	// and serviceVersion declared in the definition.
	//
	// If both `cluster` and `serviceDescriptor` are specified, the `cluster` takes precedence.
	//
	// +optional
	ServiceDescriptor string `json:"serviceDescriptor,omitempty"`
}

type ServiceRefClusterSelector struct {
	// The name of the Cluster being referenced.
	//
	// +kubebuilder:validation:Required
	Cluster string `json:"cluster"`

	// Identifies a ClusterService from the list of Services defined in `cluster.spec.services` of the referenced Cluster.
	//
	// +optional
	Service *ServiceRefServiceSelector `json:"service,omitempty"`

	// +optional
	PodFQDNs *ServiceRefPodFQDNsSelector `json:"podFQDNs,omitempty"`

	// Specifies the SystemAccount to authenticate and establish a connection with the referenced Cluster.
	// The SystemAccount should be defined in `componentDefinition.spec.systemAccounts`
	// of the Component providing the service in the referenced Cluster.
	//
	// +optional
	Credential *ServiceRefCredentialSelector `json:"credential,omitempty"`
}

type ServiceRefServiceSelector struct {
	// The name of the Component where the Service resides in.
	//
	// It is required when referencing a Component's Service.
	//
	// +optional
	Component string `json:"component,omitempty"`

	// The name of the Service to be referenced.
	//
	// Leave it empty to reference the default Service. Set it to "headless" to reference the default headless Service.
	//
	// If the referenced Service is of pod-service type (a Service per Pod), there will be multiple Service objects matched,
	// and the resolved value will be presented in the following format: service1.name,service2.name...
	//
	// +kubebuilder:validation:Required
	Service string `json:"service"`

	// The port name of the Service to be referenced.
	//
	// If there is a non-zero node-port exist for the matched Service port, the node-port will be selected first.
	//
	// If the referenced Service is of pod-service type (a Service per Pod), there will be multiple Service objects matched,
	// and the resolved value will be presented in the following format: service1.name:port1,service2.name:port2...
	//
	// +optional
	Port string `json:"port,omitempty"`
}

type ServiceRefPodFQDNsSelector struct {
	// The name of the Component where the pods reside in.
	//
	// +kubebuilder:validation:Required
	Component string `json:"component"`

	// The role of the pods to reference.
	//
	// +optional
	Role *string `json:"role,omitempty"`
}

type ServiceRefCredentialSelector struct {
	// The name of the Component where the credential resides in.
	//
	// +kubebuilder:validation:Required
	Component string `json:"component"`

	// The name of the credential (SystemAccount) to reference.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type ClusterComponentVolumeClaimTemplate struct {
	// Refers to the name of a volumeMount defined in either:
	//
	// - `componentDefinition.spec.runtime.containers[*].volumeMounts`
	// - `clusterDefinition.spec.componentDefs[*].podSpec.containers[*].volumeMounts` (deprecated)
	//
	// The value of `name` must match the `name` field of a volumeMount specified in the corresponding `volumeMounts` array.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the desired characteristics of a PersistentVolumeClaim that will be created for the volume
	// with the mount name specified in the `name` field.
	//
	// When a Pod is created for this ClusterComponent, a new PVC will be created based on the specification
	// defined in the `spec` field. The PVC will be associated with the volume mount specified by the `name` field.
	//
	// +optional
	Spec PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

type PersistentVolumeClaimSpec struct {
	// Contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`

	// Represents the minimum resources the volume should have.
	// If the RecoverVolumeExpansionFailure feature is enabled, users are allowed to specify resource requirements that
	// are lower than the previous value but must still be higher than the capacity recorded in the status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.VolumeResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`

	// The name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1.
	//
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,5,opt,name=storageClassName"`

	// Defines what type of volume is required by the claim, either Block or Filesystem.
	//
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty" protobuf:"bytes,6,opt,name=volumeMode,casttype=PersistentVolumeMode"`
}

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

// ComponentService defines a service that would be exposed as an inter-component service within a Cluster.
// A Service defined in the ComponentService is expected to be accessed by other Components within the same Cluster.
//
// When a Component needs to use a ComponentService provided by another Component within the same Cluster,
// it can declare a variable in the `componentDefinition.spec.vars` section and bind it to the specific exposed address
// of the ComponentService using the `serviceVarRef` field.
type ComponentService struct {
	Service `json:",inline"`

	// Indicates whether to create a corresponding Service for each Pod of the selected Component.
	// When set to true, a set of Services will be automatically generated for each Pod,
	// and the `roleSelector` field will be ignored.
	//
	// The names of the generated Services will follow the same suffix naming pattern: `$(serviceName)-$(podOrdinal)`.
	// The total number of generated Services will be equal to the number of replicas specified for the Component.
	//
	// Example usage:
	//
	// ```yaml
	// name: my-service
	// serviceName: my-service
	// podService: true
	// disableAutoProvision: true
	// spec:
	//   type: NodePort
	//   ports:
	//   - name: http
	//     port: 80
	//     targetPort: 8080
	// ```
	//
	// In this example, if the Component has 3 replicas, three Services will be generated:
	// - my-service-0: Points to the first Pod (podOrdinal: 0)
	// - my-service-1: Points to the second Pod (podOrdinal: 1)
	// - my-service-2: Points to the third Pod (podOrdinal: 2)
	//
	// Each generated Service will have the specified spec configuration and will target its respective Pod.
	//
	// This feature is useful when you need to expose each Pod of a Component individually, allowing external access
	// to specific instances of the Component.
	//
	// +kubebuilder:default=false
	// +optional
	PodService *bool `json:"podService,omitempty"`

	// Indicates whether the automatic provisioning of the service should be disabled.
	//
	// If set to true, the service will not be automatically created at the component provisioning.
	// Instead, you can enable the creation of this service by specifying it explicitly in the cluster API.
	//
	// +optional
	DisableAutoProvision *bool `json:"disableAutoProvision,omitempty"`
}

type ComponentSystemAccount struct {
	// The name of the system account.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the policy for generating the account's password.
	//
	// This field is immutable once set.
	//
	// +optional
	PasswordConfig *PasswordConfig `json:"passwordConfig,omitempty"`

	// Refers to the secret from which data will be copied to create the new account.
	//
	// This field is immutable once set.
	//
	// +optional
	SecretRef *ProvisionSecretRef `json:"secretRef,omitempty"`
}

// PasswordConfig helps provide to customize complexity of password generation pattern.
type PasswordConfig struct {
	// The length of the password.
	//
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=8
	// +kubebuilder:default=16
	// +optional
	Length int32 `json:"length,omitempty"`

	// The number of digits in the password.
	//
	// +kubebuilder:validation:Maximum=8
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=4
	// +optional
	NumDigits int32 `json:"numDigits,omitempty"`

	// The number of symbols in the password.
	//
	// +kubebuilder:validation:Maximum=8
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	NumSymbols int32 `json:"numSymbols,omitempty"`

	// The case of the letters in the password.
	//
	// +kubebuilder:default=MixedCases
	// +optional
	LetterCase LetterCase `json:"letterCase,omitempty"`

	// Seed to generate the account's password.
	// Cannot be updated.
	//
	// +optional
	Seed string `json:"seed,omitempty"`
}

// LetterCase defines the available cases to be used in password generation.
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

// ProvisionSecretRef represents the reference to a secret.
type ProvisionSecretRef struct {
	// The unique identifier of the secret.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The namespace where the secret is located.
	//
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// ClusterComponentConfig represents a config with its source bound.
type ClusterComponentConfig struct {
	// The name of the config.
	//
	// +optional
	Name *string `json:"name,omitempty"`

	// The source of the config.
	ClusterComponentConfigSource `json:",inline"`
}

// ClusterComponentConfigSource represents the source of a config.
type ClusterComponentConfigSource struct {
	// ConfigMap source for the config.
	//
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`

	// TODO: support more diverse sources:
	// - Config template of other components within the same cluster
	// - Config template of components from other clusters
	// - Secret
	// - Local file
}

type PodUpdatePolicyType string

const (
	// StrictInPlacePodUpdatePolicyType indicates that only allows in-place upgrades.
	// Any attempt to modify other fields will be rejected.
	StrictInPlacePodUpdatePolicyType PodUpdatePolicyType = "StrictInPlace"

	// PreferInPlacePodUpdatePolicyType indicates that we will first attempt an in-place upgrade of the Pod.
	// If that fails, it will fall back to the ReCreate, where pod will be recreated.
	PreferInPlacePodUpdatePolicyType PodUpdatePolicyType = "PreferInPlace"
)

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
