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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// compDef is the name of the referenced componentDefinition.
	// +kubebuilder:validation:Required
	CompDef string `json:"compDef"`

	// classDefRef references the class defined in ComponentClassDefinition.
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:
	// Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.
	// Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.
	// Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
	// It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

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

	// monitor is a switch to enable monitoring and is set as false by default.
	// KubeBlocks provides an extension mechanism to support component level monitoring,
	// which will scrape metrics auto or manually from servers in component and export
	// metrics to Time Series Database.
	// +kubebuilder:default=false
	// +optional
	Monitor bool `json:"monitor,omitempty"`

	// enabledLogs indicates which log file takes effect in the database cluster.
	// element is the log type which is defined in ComponentDefinition logConfig.name,
	// and will set relative variables about this log type in database kernel.
	// +listType=set
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

	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// RsmTransformPolicy defines the policy generate sts using rsm.
	// ToSts: rsm transform to statefulSet
	// ToPod: rsm transform to pods
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ToSts
	// +optional
	RsmTransformPolicy workloads.RsmTransformPolicy `json:"rsmTransformPolicy,omitempty"`

	// Nodes defines the list of nodes that pods can schedule
	// If the RsmTransformPolicy is specified as OneToMul,the list of nodes will be used. If the list of nodes is empty,
	// no specific node will be assigned. However, if the list of node is filled, all pods will be evenly scheduled
	// across the nodes in the list.
	// +optional
	Nodes []types.NodeName `json:"nodes,omitempty"`

	// Instances defines the list of instance to be deleted priorly
	// +optional
	Instances []string `json:"instances,omitempty"`
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

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=cmp
// +kubebuilder:printcolumn:name="COMPONENT-DEFINITION",type="string",JSONPath=".spec.compDef",description="component definition"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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
