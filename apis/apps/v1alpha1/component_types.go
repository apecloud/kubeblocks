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

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// Specifies the name of the referenced ComponentDefinition.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	CompDef string `json:"compDef"`

	// ServiceVersion specifies the version of the service provisioned by the component.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Define service references for the current component. Based on the referenced services, they can be categorized into two types:
	// - Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.
	// - Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.
	//
	// Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
	// It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Requests and limits of workload resources.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Information for statefulset.spec.volumeClaimTemplates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// To override services defined in referenced ComponentDefinition.
	//
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// Specifies the desired number of replicas for the component's workload.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Defines the configuration for the component.
	//
	// +optional
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// A switch to enable monitoring and is set as false by default.
	// KubeBlocks provides an extension mechanism to support component level monitoring,
	// which will scrape metrics auto or manually from servers in component and export
	// metrics to Time Series Database.
	//
	// +kubebuilder:default=false
	// +optional
	Monitor bool `json:"monitor,omitempty"`

	// Indicates which log file takes effect in the database cluster,
	// element is the log type which is defined in ComponentDefinition logConfig.name.
	//
	// +listType=set
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// The name of the ServiceAccount that running component depends on.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Specifies the scheduling constraints for the component.
	// If specified, it will override the cluster-wide affinity.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Specify the tolerations for the component's workload.
	// If specified, they will override the cluster-wide toleration settings.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies the scheduling policy for the component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies the TLS configuration for the component.
	//
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// Overrides values in default Template.
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies instances to be scaled in with dedicated names in the list.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Defines RuntimeClassName for all Pods managed by this component.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
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

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
