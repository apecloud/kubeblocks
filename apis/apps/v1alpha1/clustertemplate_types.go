/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cmpd
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version",description="component version"
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".spec.serviceKind",description="service"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterTemplate is the Schema for the clustertemplates API
type ClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentTemplateSpec `json:"spec,omitempty"`
	Status ClusterTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterTemplateList contains a list of ClusterTemplate
type ClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterTemplate{}, &ClusterTemplateList{})
}

// ClusterTemplateSpec defines the desired state of ClusterTemplate
type ClusterTemplateSpec struct {
	// UpdatePolicy defines the update behavior for existing clusters using this template when the cluster template is updated.
	// +kubebuilder:validation:Required
	UpdatePolicy ClusterTemplateUpdatePolicy `json:"updatePolicy"`

	Components []ClusterComponentTemplateSpec `json:"components,omitempty"`

	// Services expose endpoints that can be accessed by clients.
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// TODO: support other resources provisioning.
	// +optional
	DefaultAccounts *SystemAccountSpec `json:"defaultAccounts,omitempty"`

	ConnectionCredentials []ConnectionCredential `json:"connectionCredentials,omitempty"`

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

	// +optional
	Tenancy *TenancyType `json:"tenancy,omitempty"`

	// +optional
	AvailabilityPolicy *AvailabilityPolicyType `json:"availabilityPolicy,omitempty"`

	// +kubebuilder:default=false
	// +optional
	TLS bool `json:"tls,omitempty"`

	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`
}

// ClusterTemplateStatus defines the observed state of ClusterTemplate
type ClusterTemplateStatus struct {
	// ObservedGeneration is the most recent generation observed for this ClusterTemplate.
	// It corresponds to the ComponentDefinition's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase valid values are ``, `Available`, 'Unavailable`.
	// Available is ComponentDefinition become available, and can be referenced for co-related objects.
	Phase Phase `json:"phase,omitempty"`
}

type ClusterTemplateUpdatePolicy struct {
}

type ClusterComponentTemplateSpec struct {
	// The name of the component.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	ComponentDef string `json:"componentDef"`

	//// +optional
	// ClassRef string `json:"classRef,omitempty"`

	// Resources requests and limits of workload.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// volumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.
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

	//// switchPolicy defines the strategy for switchover and failover when workloadType is Replication.
	//// +optional
	// SwitchPolicy *ClusterSwitchPolicy `json:"switchPolicy,omitempty"`

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
