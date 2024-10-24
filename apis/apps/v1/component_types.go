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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:categories={kubeblocks},shortName=cmp
// +kubebuilder:printcolumn:name="DEFINITION",type="string",JSONPath=".spec.compDef",description="component definition"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Component is a fundamental building block of a Cluster object.
// For example, a Redis Cluster can include Components like 'redis', 'sentinel', and potentially a proxy like 'twemproxy'.
//
// The Component object is responsible for managing the lifecycle of all replicas within a Cluster component,
// It supports a wide range of operations including provisioning, stopping, restarting, termination, upgrading,
// configuration changes, vertical and horizontal scaling, failover, switchover, cross-node migration,
// scheduling configuration, exposing Services, managing system accounts, enabling/disabling exporter,
// and configuring log collection.
//
// Component is an internal sub-object derived from the user-submitted Cluster object.
// It is designed primarily to be used by the KubeBlocks controllers,
// users are discouraged from modifying Component objects directly and should use them only for monitoring Component statuses.
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

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// Specifies the name of the referenced ComponentDefinition.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	CompDef string `json:"compDef"`

	// ServiceVersion specifies the version of the Service expected to be provisioned by this Component.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Defines a list of ServiceRef for a Component, enabling access to both external services and
	// Services provided by other Clusters.
	//
	// Types of services:
	//
	// - External services: Not managed by KubeBlocks or managed by a different KubeBlocks operator;
	//   Require a ServiceDescriptor for connection details.
	// - Services provided by a Cluster: Managed by the same KubeBlocks operator;
	//   identified using Cluster, Component and Service names.
	//
	// ServiceRefs with identical `serviceRef.name` in the same Cluster are considered the same.
	//
	// Example:
	// ```yaml
	// serviceRefs:
	//   - name: "redis-sentinel"
	//     serviceDescriptor:
	//       name: "external-redis-sentinel"
	//   - name: "postgres-cluster"
	//     clusterServiceSelector:
	//       cluster: "my-postgres-cluster"
	//       service:
	//         component: "postgresql"
	// ```
	// The example above includes ServiceRefs to an external Redis Sentinel service and a PostgreSQL Cluster.
	//
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Specifies Labels to override or add for underlying Pods, PVCs, Account & TLS Secrets, Services Owned by Component.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies Annotations to override or add for underlying Pods, PVCs, Account & TLS Secrets, Services Owned by Component.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// List of environment variables to add.
	//
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the resources required by the Component.
	// It allows defining the CPU, memory requirements and limits for the Component's containers.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component.
	// Each template specifies the desired characteristics of a persistent volume, such as storage class,
	// size, and access modes.
	// These templates are used to dynamically provision persistent volumes for the Component.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// List of volumes to override.
	//
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Overrides Services defined in referenced ComponentDefinition and exposes endpoints that can be accessed by clients.
	//
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// Overrides system accounts defined in referenced ComponentDefinition.
	//
	// +optional
	SystemAccounts []ComponentSystemAccount `json:"systemAccounts,omitempty"`

	// Specifies the desired number of replicas in the Component for enhancing availability and durability, or load balancing.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Specifies the configuration content of a config template.
	//
	// +optional
	Configs []ClusterComponentConfig `json:"configs,omitempty"`

	// Specifies the name of the ServiceAccount required by the running Component.
	// This ServiceAccount is used to grant necessary permissions for the Component's Pods to interact
	// with other Kubernetes resources, such as modifying Pod labels or sending events.
	//
	// If not specified, KubeBlocks automatically creates a default ServiceAccount named
	// "kb-{cluster.name}-{component.name}", bound to a role with rules defined in ComponentDefinition's
	// `policyRules` field. If the field is empty, ServiceAccount will not be created.
	//
	// If the field is not empty, the specified ServiceAccount will be used. And KubeBlocks will not
	// create a ServiceAccount.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Controls the concurrency of pods during initial scale up, when replacing pods on nodes,
	// or when scaling down. It only used when `PodManagementPolicy` is set to `Parallel`.
	// The default Concurrency is 100%.
	//
	// +optional
	ParallelPodManagementConcurrency *intstr.IntOrString `json:"parallelPodManagementConcurrency,omitempty"`

	// PodUpdatePolicy indicates how pods should be updated
	//
	// - `StrictInPlace` indicates that only allows in-place upgrades.
	// Any attempt to modify other fields will be rejected.
	// - `PreferInPlace` indicates that we will first attempt an in-place upgrade of the Pod.
	// If that fails, it will fall back to the ReCreate, where pod will be recreated.
	// Default value is "PreferInPlace"
	//
	// +optional
	PodUpdatePolicy *PodUpdatePolicyType `json:"podUpdatePolicy,omitempty"`

	// Specifies the scheduling policy for the Component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies the TLS configuration for the Component, including:
	//
	// - A boolean flag that indicates whether the Component should use Transport Layer Security (TLS) for secure communication.
	// - An optional field that specifies the configuration for the TLS certificates issuer when TLS is enabled.
	//   It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
	//	 The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
	//
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// Allows for the customization of configuration values for each instance within a Component.
	// An Instance represent a single replica (Pod and associated K8s resources like PVCs, Services, and ConfigMaps).
	// While instances typically share a common configuration as defined in the ClusterComponentSpec,
	// they can require unique settings in various scenarios:
	//
	// For example:
	// - A database Component might require different resource allocations for primary and secondary instances,
	//   with primaries needing more resources.
	// - During a rolling upgrade, a Component may first update the image for one or a few instances,
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
	// 1. The associated Pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
	//    future reuse or data recovery, but it is no longer actively used.
	// 2. The ordinal number assigned to this instance is preserved, ensuring it remains unique
	//    and avoiding conflicts with new instances.
	//
	// Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
	// ordinal consistency within the Cluster.
	// Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
	// The administrator must manually manage the cleanup and removal of these resources when they are no longer needed.
	//
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Defines runtimeClassName for all Pods managed by this Component.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Determines whether metrics exporter information is annotated on the Component's headless Service.
	//
	// If set to true, the following annotations will not be patched into the Service:
	//
	// - "monitor.kubeblocks.io/path"
	// - "monitor.kubeblocks.io/port"
	// - "monitor.kubeblocks.io/scheme"
	//
	// These annotations allow the Prometheus installed by KubeBlocks to discover and scrape metrics from the exporter.
	//
	// +optional
	DisableExporter *bool `json:"disableExporter,omitempty"`

	// Stop the Component.
	// If set, all the computing resources will be released.
	//
	// +optional
	Stop *bool `json:"stop,omitempty"`
}

// ComponentStatus represents the observed state of a Component within the Cluster.
type ComponentStatus struct {
	// Specifies the most recent generation observed for this Component object.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents a list of detailed status of the Component object.
	// Each condition in the list provides real-time information about certain aspect of the Component object.
	//
	// This field is crucial for administrators and developers to monitor and respond to changes within the Component.
	// It provides a history of state transitions and a snapshot of the current state that can be used for
	// automated logic or direct inspection.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Indicates the current phase of the Component, with each phase indicating specific conditions:
	//
	// - Creating: The initial phase for new Components, transitioning from 'empty'("").
	// - Running: All Pods in a Running state.
	// - Updating: The Component is currently being updated, with no failed Pods present.
	// - Abnormal: Some Pods have failed, indicating a potentially unstable state.
	//   However, the cluster remains available as long as a quorum of members is functioning.
	// - Failed: A significant number of Pods or critical Pods have failed
	//   The cluster may be non-functional or may offer only limited services (e.g, read-only).
	// - Stopping: All Pods are being terminated, with current replica count at zero.
	// - Stopped: All associated Pods have been successfully deleted.
	// - Deleting: The Component is being deleted.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// A map that stores detailed message about the Component.
	// Each entry in the map provides insights into specific elements of the Component, such as Pods or workloads.
	//
	// Keys in this map are formatted as `ObjectKind/Name`, where `ObjectKind` could be a type like Pod,
	// and `Name` is the specific name of the object.
	//
	// +optional
	Message map[string]string `json:"message,omitempty"`
}
