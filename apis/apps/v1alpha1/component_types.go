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
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// compDef is the name of the referenced componentDefinition.
	// +kubebuilder:validation:Required
	CompDef string `json:"compDef"`

	// classDefRef references the class defined in ComponentClassDefinition.
	// +kubebuilder:deprecatedversion:warning="Due to the lack of practical use cases, this field is deprecated from KB 0.9.0."
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:
	//
	// - Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.
	// - Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.
	//
	// Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
	// It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.
	//
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Specifies Annotations to override or add for underlying Pods.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// List of environment variables to add.
	//
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources requests and limits of workload.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// VolumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// List of volumes to override.
	//
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

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
	//
	// - Creating: `Creating` is a special `Updating` with previous phase `empty`(means "") or `Creating`.
	// - Running: component replicas > 0 and all pod specs are latest with a Running state.
	// - Updating: component replicas > 0 and has no failed pods. the component is being updated.
	// - Abnormal: component replicas > 0 but having some failed pods. the component basically works but in a fragile state.
	// - Failed: component replicas > 0 but having some failed pods. the component doesn't work anymore.
	// - Stopping: component replicas = 0 and has terminating pods.
	// - Stopped: component replicas = 0 and all pods have been deleted.
	// - Deleting: the component is being deleted.
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
