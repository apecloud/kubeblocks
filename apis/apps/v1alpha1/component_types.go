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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// +kubebuilder:validation:Required
	CompDef string `json:"compDef"`

	//// classDefRef references the class defined in ComponentClassDefinition.
	//// +optional
	// ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	//// serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:
	//// Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.
	//// Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.
	//// Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
	//// It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.
	//// +optional
	// ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Resources requests and limits of workload.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// VolumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Component replicas. The default value is used in ClusterDefinition spec if not specified.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// +optional
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	//// Services expose endpoints that can be accessed by clients.
	//// +optional
	// Services []ClusterComponentService `json:"services,omitempty"`

	// +optional
	Monitor *intstr.IntOrString `json:"monitor,omitempty"`

	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// serviceAccountName is the name of the ServiceAccount that running component depends on.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Component termination policy. Valid values are DoNotTerminate, Halt, Delete, WipeOut.
	// DoNotTerminate will block delete operation.
	// Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	// Delete is based on Halt and deletes PVCs.
	// WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	// +kubebuilder:validation:Required
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy"`

	// +kubebuilder:default=false
	// +optional
	TLS bool `json:"tls,omitempty"`

	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`

	//// noCreatePDB defines the PodDisruptionBudget creation behavior and is set to true if creation of PodDisruptionBudget
	//// for this component is not needed. It defaults to false.
	//// +kubebuilder:default=false
	//// +optional
	// NoCreatePDB bool `json:"noCreatePDB,omitempty"`
}

// ComponentStatus defines the observed state of Component
type ComponentStatus struct {
	// observedGeneration is the most recent generation observed for this Component.
	// It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Describe current state of component API Resource, like warning.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// phase describes the phase of the component and the detail information of the phases are as following:
	// Creating: `Creating` is a special `Updating` with previous phase `empty`(means "") or `Creating`.
	// Running: component replicas > 0 and all pod specs are latest with a Running state.
	// Updating: component replicas > 0 and has no failed pods. the component is being updated.
	// Abnormal: component replicas > 0 but having some failed pods. the component basically works but in a fragile state.
	// Failed: component replicas > 0 but having some failed pods. the component doesn't work anymore.
	// Stopping: component replicas = 0 and has terminating pods.
	// Stopped: component replicas = 0 and all pods have been deleted.
	// Deleting: the component is being deleted.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// message records the component details message in current phase.
	// Keys are podName or deployName or statefulSetName. The format is `ObjectKind/Name`.
	// +optional
	Message ComponentMessageMap `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
