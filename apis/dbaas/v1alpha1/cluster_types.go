/*
Copyright 2022 The Kubeblocks Authors

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	DoNotTerminate TerminationPolicyType = "DoNotTerminate"
	Halt           TerminationPolicyType = "Halt"
	Delete         TerminationPolicyType = "Delete"
	WipeOut        TerminationPolicyType = "WipeOut"
)

type TerminationPolicyType string

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// ref ClusterDefinition, immutable
	// +kubebuilder:validation:Required
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// ref AppVersion
	// +kubebuilder:validation:Required
	AppVersionRef string `json:"appVersionRef"`

	// +optional
	Components []ClusterComponent `json:"components,omitempty"`

	// Affinity describes affinities which specific by users
	Affinity Affinity `json:"affinity,omitempty"`

	// One of DoNotTerminate, Halt, Delete, WipeOut.
	// Defaults to Halt.
	// DoNotTerminate means block delete operation.
	// Halt means delete resources such as sts,deploy,svc,pdb, but keep pvcs.
	// Delete is based on Halt and delete pvcs.
	// WipeOut is based on Delete and wipe out all snapshots and snapshot data from bucket.
	// +kubebuilder:default=Halt
	// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// observedGeneration is the most recent generation observed for this
	// Cluster. It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// phase - in list of [Running, Failed, Creating, Updating, Deleting, Deleted]
	// +kubebuilder:validation:Enum={Running,Failed,Creating,Updating,Deleting,Deleted}
	Phase Phase `json:"phase,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	Components []ClusterStatusComponent `json:"components,omitempty"`

	ClusterDefinitionStatusGeneration `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas,all}
//+kubebuilder:printcolumn:name="APP-VERSION",type="string",JSONPath=".spec.appVersionRef",description="Cluster Application Version."
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="Cluster Status."
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

type ClusterComponent struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	Name string `json:"name"`

	// component name in ClusterDefinition
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	Type string `json:"type"`

	// default value in ClusterDefinition
	Replicas int `json:"replicas,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ref roleGroups in ClusterDefinition
	RoleGroups []ClusterRoleGroup `json:"roleGroups,omitempty"`

	// +optional
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`

	// serviceType determines how the Service is exposed. Valid
	// options are ClusterIP, NodePort, and LoadBalancer.
	// "ClusterIP" allocates a cluster-internal IP address for load-balancing
	// to endpoints. Endpoints are determined by the selector or if that is not
	// specified, by manual construction of an Endpoints object or
	// EndpointSlice objects. If clusterIP is "None", no virtual IP is
	// allocated and the endpoints are published as a set of endpoints rather
	// than a virtual IP.
	// "NodePort" builds on ClusterIP and allocates a port on every node which
	// routes to the same endpoints as the clusterIP.
	// "LoadBalancer" builds on NodePort and creates an external load-balancer
	// (if supported in the current cloud) which routes to the same endpoints
	// as the clusterIP.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum={ClusterIP,NodePort,LoadBalancer}
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

type ClusterRoleGroup struct {
	// +optional
	Name string `json:"name,omitempty"`

	// roleGroup name in ClusterDefinition
	// +optional
	Type string `json:"type,omitempty"`

	// +kubebuilder:default=-1
	// +optional
	Replicas int `json:"replicas,omitempty"`

	// +optional
	Service corev1.ServiceSpec `json:"service,omitempty"`
}

type ClusterStatusComponent struct {
	ID         string                   `json:"id,omitempty"`
	Type       string                   `json:"type,omitempty"`
	Phase      string                   `json:"phase,omitempty"`
	Message    string                   `json:"message,omitempty"`
	RoleGroups []ClusterStatusRoleGroup `json:"roleGroups,omitempty"`
}

type ClusterStatusRoleGroup struct {
	ID          string `json:"id,omitempty"`
	Type        string `json:"type,omitempty"`
	RefWorkload string `json:"refWorkload,omitempty"`
}

type ClusterComponentVolumeClaimTemplate struct {
	Name string                           `json:"name"`
	Spec corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

type Affinity struct {
	// TopologyKeys describe topologyKeys for `topologySpreadConstraint` and `podAntiAffinity` in ClusterDefinition API
	// +kubebuilder:validation:MinItems=1
	TopologyKeys []string `json:"topologyKeys"`
	// NodeLabels describe constrain which nodes pod can be scheduled on based on node labels
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
