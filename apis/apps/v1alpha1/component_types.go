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

// ComponentSpec defines the desired state of Component.
type ComponentSpec struct {
	// Specifies the name of the referenced ComponentDefinition.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	CompDef string `json:"compDef"`

	// ServiceVersion specifies the version of the service expected to be provisioned by this component.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Defines a list of ServiceRef for a Component, allowing it to connect and interact with other services.
	// These services can be external or managed by the same KubeBlocks operator, categorized as follows:
	//
	// 1. External Services:
	//
	//    - Not managed by KubeBlocks. These could be services outside KubeBlocks or non-Kubernetes services.
	//    - Connection requires a ServiceDescriptor providing details for service binding.
	//
	// 2. KubeBlocks Services:
	//
	//    - Managed within the same KubeBlocks environment.
	//    - Service binding is achieved by specifying cluster names in the service references,
	//      with configurations handled by the KubeBlocks operator.
	//
	// ServiceRef maintains cluster-level semantic consistency; references with the same `serviceRef.name`
	// within the same cluster are treated as identical.
	// Only bindings to the same cluster or ServiceDescriptor are allowed within a cluster.
	//
	// Example:
	// ```yaml
	// serviceRefs:
	//   - name: "redis-sentinel"
	//     serviceDescriptor:
	//       name: "external-redis-sentinel"
	//   - name: "postgres-cluster"
	//     cluster:
	//       name: "my-postgres-cluster"
	// ```
	// The example above includes references to an external Redis Sentinel service and a PostgreSQL cluster managed by KubeBlocks.
	//
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Specifies the resources required by the Component.
	// It allows defining the CPU, memory requirements and limits for the Component's containers.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component.
	// Each template specifies the desired characteristics of a persistent volume, such as storage class,
	// size, and access modes.
	// These templates are used to dynamically provision persistent volumes for the Component when it is deployed.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Overrides services defined in referenced ComponentDefinition and exposes endpoints that can be accessed
	// by clients.
	//
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// Each component supports running multiple replicas to provide high availability and persistence.
	// This field can be used to specify the desired number of replicas.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Reserved field for future use.
	//
	// +optional
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// Specifies which types of logs should be collected for the Cluster.
	// The log types are defined in the `componentDefinition.spec.logConfigs` field with the LogConfig entries.
	//
	// The elements in the `enabledLogs` array correspond to the names of the LogConfig entries.
	// For example, if the `componentDefinition.spec.logConfigs` defines LogConfig entries with
	// names "slow_query_log" and "error_log",
	// you can enable the collection of these logs by including their names in the `enabledLogs` array:
	// enabledLogs: ["slow_query_log", "error_log"]
	//
	// +listType=set
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// Specifies the name of the ServiceAccount required by the running Component.
	// This ServiceAccount is used to grant necessary permissions for the Component's Pods to interact
	// with other Kubernetes resources, such as modifying pod labels or sending events.
	//
	// Defaults:
	// If not specified, KubeBlocks automatically assigns a default ServiceAccount named "kb-{cluster.name}",
	// bound to a default role defined during KubeBlocks installation.
	//
	// Future Changes:
	// Future versions might change the default ServiceAccount creation strategy to one per Component,
	// potentially revising the naming to "kb-{cluster.name}-{component.name}".
	//
	// Users can override the automatic ServiceAccount assignment by explicitly setting the name of
	// an existed ServiceAccount in this field.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Specifies a group of affinity scheduling rules for the Component.
	// It allows users to control how the Component's Pods are scheduled onto nodes in the cluster.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Allows the Component to be scheduled onto nodes with matching taints.
	// It is an array of tolerations that are attached to the Component's Pods.
	//
	// Each toleration consists of a `key`, `value`, `effect`, and `operator`.
	// The `key`, `value`, and `effect` define the taint that the toleration matches.
	// The `operator` specifies how the toleration matches the taint.
	//
	// If a node has a taint that matches a toleration, the Component's pods can be scheduled onto that node.
	// This allows the Component's Pods to run on nodes that have been tainted to prevent regular Pods from being scheduled.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies the scheduling policy for the component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies the TLS configuration for the component, including:
	//
	// - A boolean flag that indicates whether the component should use Transport Layer Security (TLS) for secure communication.
	// - An optional field that specifies the configuration for the TLS certificates issuer when TLS is enabled.
	//   It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
	//	 The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
	//
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// Allows for the customization of configuration values for each instance within a component.
	// An Instance represent a single replica (Pod and associated K8s resources like PVCs, Services, and ConfigMaps).
	// While instances typically share a common configuration as defined in the ClusterComponentSpec,
	// they can require unique settings in various scenarios:
	//
	// For example:
	// - A database component might require different resource allocations for primary and secondary instances,
	//   with primaries needing more resources.
	// - During a rolling upgrade, a component may first update the image for one or a few instances,
	//   and then update the remaining instances after verifying that the updated instances are functioning correctly.
	//
	// InstanceTemplate allows for specifying these unique configurations per instance.
	// Each instance's name is constructed using the pattern: $(component.name)-$(template.name)-$(ordinal),
	// starting with an ordinal of 0.
	// It is crucial to maintain unique names for each InstanceTemplate to avoid conflicts.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the Component.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies the names of instances to be transitioned to offline status.
	//
	// Marking an instance as offline results in the following:
	//
	// 1. The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
	//    future reuse or data recovery, but it is no longer actively used.
	// 2. The ordinal number assigned to this instance is preserved, ensuring it remains unique
	//    and avoiding conflicts with new instances.
	//
	// Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
	// ordinal consistency within the cluster.
	// Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
	// The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.
	//
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Defines RuntimeClassName for all Pods managed by this component.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Defines the sidecar containers that will be attached to the component's main container.
	//
	// +optional
	Sidecars []string `json:"sidecars,omitempty"`
}

// ComponentStatus represents the observed state of a Component within the cluster.
type ComponentStatus struct {
	// Specifies the most recent generation observed for this Component.
	// This corresponds to the Cluster's generation, which is updated by the API Server upon mutation.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Defines the current state of the component API Resource, such as warnings.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Indicates the phase of the component. Detailed information for each phase is as follows:
	//
	// - Creating: A special `Updating` phase with previous phase `empty`(means "") or `Creating`.
	// - Running: Component replicas > 0 and all pod specs are latest with a Running state.
	// - Updating: Component replicas > 0 and no failed pods. The component is being updated.
	// - Abnormal: Component replicas > 0 but some pods have failed. The component is functional but in a fragile state.
	// - Failed: Component replicas > 0 but some pods have failed. The component is no longer functional.
	// - Stopping: Component replicas = 0 and pods are terminating.
	// - Stopped: Component replicas = 0 and all pods have been deleted.
	// - Deleting: The component is being deleted.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// Records the detailed message of the component in its current phase.
	// Keys can be podName, deployName, or statefulSetName. The format is `ObjectKind/Name`.
	//
	// +optional
	Message ComponentMessageMap `json:"message,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=cmp
// +kubebuilder:printcolumn:name="DEFINITION",type="string",JSONPath=".spec.compDef",description="component definition"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Component is a derived sub-object of a user-submitted Cluster object.
// It is an internal object, and users should not modify Component objects;
// they are intended only for observing the status of Components within the system.
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component.
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
