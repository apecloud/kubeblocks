/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/util/intstr"
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

type PersistentVolumeClaimTemplate struct {
	// Refers to the name of a volumeMount defined in either:
	//
	// - `componentDefinition.spec.runtime.containers[*].volumeMounts`
	//
	// The value of `name` must match the `name` field of a volumeMount specified in the corresponding `volumeMounts` array.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the prefix of the PVC name for the volume.
	//
	// For each replica, the final name of the PVC will be in format: <persistentVolumeClaimName>-<ordinal>
	//
	// +optional
	PersistentVolumeClaimName *string `json:"persistentVolumeClaimName,omitempty"`

	// Specifies the labels for the PVC of the volume.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies the annotations for the PVC of the volume.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Defines the desired characteristics of a PersistentVolumeClaim that will be created for the volume
	// with the mount name specified in the `name` field.
	//
	// +optional
	Spec corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

// PersistentVolumeClaimRetentionPolicy describes the policy used for PVCs created from the VolumeClaimTemplates.
type PersistentVolumeClaimRetentionPolicy struct {
	// WhenDeleted specifies what happens to PVCs created from VolumeClaimTemplates when the workload is deleted.
	// The `Retain` policy causes PVCs to not be affected by workload deletion.
	// The default policy of `Delete` causes those PVCs to be deleted.
	//
	// +optional
	WhenDeleted PersistentVolumeClaimRetentionPolicyType `json:"whenDeleted,omitempty"`

	// WhenScaled specifies what happens to PVCs created from VolumeClaimTemplates when the workload is scaled down.
	// The `Retain` policy causes PVCs to not be affected by a scale down.
	// The default policy of `Delete` causes the associated PVCs for pods scaled down to be deleted.
	//
	// +optional
	WhenScaled PersistentVolumeClaimRetentionPolicyType `json:"whenScaled,omitempty"`
}

// PersistentVolumeClaimRetentionPolicyType is a string enumeration of the policies that will determine
// when volumes from the VolumeClaimTemplates will be deleted when the controlling StatefulSet is
// deleted or scaled down.
//
// +enum
// +kubebuilder:validation:Enum={Retain,Delete}
type PersistentVolumeClaimRetentionPolicyType string

const (
	// RetainPersistentVolumeClaimRetentionPolicyType is the default PersistentVolumeClaimRetentionPolicy
	// and specifies that PersistentVolumeClaims associated with VolumeClaimTemplates will not be deleted.
	RetainPersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Retain"

	// DeletePersistentVolumeClaimRetentionPolicyType specifies that PersistentVolumeClaims associated with
	// VolumeClaimTemplates will be deleted in the scenario specified in PersistentVolumeClaimRetentionPolicy.
	DeletePersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Delete"
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

	// Specifies whether the system account is disabled.
	//
	// +kubebuilder:default=false
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// Specifies the policy for generating the account's password.
	//
	// This field is immutable once set.
	//
	// +optional
	PasswordConfig *PasswordConfig `json:"passwordConfig,omitempty"`

	// Refers to the secret from which data will be copied to create the new account.
	//
	// For user-specified passwords, the maximum length is limited to 64 bytes.
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

	// The key in the secret data that contains the password.
	//
	// +kubebuilder:default="password"
	// +optional
	Password string `json:"password,omitempty"`
}

// ClusterComponentConfig represents a configuration for a component.
type ClusterComponentConfig struct {
	// The name of the config.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	Name *string `json:"name,omitempty"`

	// Variables are key-value pairs for dynamic configuration values that can be provided by the user.
	//
	// +optional
	Variables map[string]string `json:"variables,omitempty"`

	// The external source for the configuration.
	ClusterComponentConfigSource `json:",inline"`

	// The custom reconfigure action to reload the service configuration whenever changes to this config are detected.
	//
	// The container executing this action has access to following variables:
	//
	// - KB_CONFIG_FILES_CREATED: file1,file2...
	// - KB_CONFIG_FILES_REMOVED: file1,file2...
	// - KB_CONFIG_FILES_UPDATED: file1:checksum1,file2:checksum2...
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Reconfigure *Action `json:"reconfigure,omitempty"`

	// ExternalManaged indicates whether the configuration is managed by an external system.
	// When set to true, the controller will use the user-provided template and reconfigure action,
	// ignoring the default template and update behavior.
	//
	// +optional
	ExternalManaged *bool `json:"externalManaged,omitempty"`
}

// ClusterComponentConfigSource represents the source of a configuration for a component.
type ClusterComponentConfigSource struct {
	// ConfigMap source for the config.
	//
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`

	// TODO: additional fields can be added to support other types of sources in the future, such as:
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

// InstanceUpdateStrategy defines fine-grained control over the spec update process of all instances.
type InstanceUpdateStrategy struct {
	// Indicates the type of the update strategy.
	// Default is RollingUpdate.
	//
	// +optional
	Type InstanceUpdateStrategyType `json:"type,omitempty"`

	// Specifies how the rolling update should be applied.
	//
	// +optional
	RollingUpdate *RollingUpdate `json:"rollingUpdate,omitempty"`
}

// InstanceUpdateStrategyType is a string enumeration type that enumerates
// all possible update strategies for the KubeBlocks controllers.
//
// +enum
// +kubebuilder:validation:Enum={RollingUpdate,OnDelete}
type InstanceUpdateStrategyType string

const (
	// RollingUpdateStrategyType indicates that update will be
	// applied to all Instances with respect to the workload
	// ordering constraints.
	RollingUpdateStrategyType InstanceUpdateStrategyType = "RollingUpdate"
	// OnDeleteStrategyType indicates that ordered rolling restarts are disabled. Instances are recreated
	// when they are manually deleted.
	OnDeleteStrategyType InstanceUpdateStrategyType = "OnDelete"
)

// RollingUpdate specifies how the rolling update should be applied.
type RollingUpdate struct {
	// Indicates the number of instances that should be updated during a rolling update.
	// The remaining instances will remain untouched. This is helpful in defining how many instances
	// should participate in the update process.
	// Value can be an absolute number (ex: 5) or a percentage of desired instances (ex: 10%).
	// Absolute number is calculated from percentage by rounding up.
	// The default value is ComponentSpec.Replicas (i.e., update all instances).
	//
	// +optional
	Replicas *intstr.IntOrString `json:"replicas,omitempty"`

	// The maximum number of instances that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired instances (ex: 10%).
	// Absolute number is calculated from percentage by rounding up. This can not be 0.
	// Defaults to 1. The field applies to all instances. That means if there is any unavailable pod,
	// it will be counted towards MaxUnavailable.
	//
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
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

// TLSSecretRef defines the Secret that contains TLS certs.
type TLSSecretRef struct {
	// The namespace where the secret is located.
	// If not provided, the secret is assumed to be in the same namespace as the Cluster object.
	//
	// +optional
	Namespace string `json:"namespace"`

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

	// Specifies the desired Ordinals of this InstanceTemplate.
	// The Ordinals used to specify the ordinal of the instance (pod) names to be generated under this InstanceTemplate.
	//
	// For example, if Ordinals is {ranges: [{start: 0, end: 1}], discrete: [7]},
	// then the instance names generated under this InstanceTemplate would be
	// $(cluster.name)-$(component.name)-$(template.name)-0、$(cluster.name)-$(component.name)-$(template.name)-1 and
	// $(cluster.name)-$(component.name)-$(template.name)-7
	Ordinals Ordinals `json:"ordinals,omitempty"`

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

	// Specifies the scheduling policy for the instance.
	// If defined, it will overwrite the scheduling policy defined in ClusterSpec and/or ClusterComponentSpec.
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
}

// Range represents a range with a start and an end value.
// It is used to define a continuous segment.
type Range struct {
	Start int32 `json:"start"`
	End   int32 `json:"end"`
}

// Ordinals represents a combination of continuous segments and individual values.
type Ordinals struct {
	Ranges   []Range `json:"ranges,omitempty"`
	Discrete []int32 `json:"discrete,omitempty"`
}
