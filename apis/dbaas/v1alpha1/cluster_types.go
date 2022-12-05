/*
Copyright ApeCloud Inc.

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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// Cluster referenced ClusterDefinition name, this is an immutable attribute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// Cluster referenced AppVersion name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	AppVersionRef string `json:"appVersionRef"`

	// Cluster termination policy. One of DoNotTerminate, Halt, Delete, WipeOut.
	// DoNotTerminate will block delete operation.
	// Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	// Delete is based on Halt and deletes PVCs.
	// WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy"`

	// List of components you want to replace in ClusterDefinition and AppVersion. It will replace the field in ClusterDefinition's and AppVersion's component if type is matching.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Components []ClusterComponent `json:"components,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Affinity describes affinities which specific by users.
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Cluster Tolerations are attached to tolerate any taint that matches the triple <key,value,effect> using the matching operator <operator>.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// observedGeneration is the most recent generation observed for this
	// Cluster. It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase describe the phase of the cluster. the detail information of phase is as follows:
	// Creating: creating cluster.
	// Running: cluster is running, all components is available.
	// Updating: cluster changes, such as horizontal-scaling/vertical-scaling/restart.
	// VolumeExpanding: volume expansion operation is running.
	// Deleting/Deleted: deleting cluster/cluster is deleted.
	// Failed: cluster not available.
	// Abnormal: cluster available but some component is not Abnormal.
	// if the component type is Consensus/Replication, the Leader/Primary pod is must ready in Abnormal phase.
	// +kubebuilder:validation:Enum={Running,Failed,Abnormal,Creating,Updating,Deleting,Deleted,VolumeExpanding}
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Message cluster details message in current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// Components record the current status information of all components of the cluster.
	// +optional
	Components map[string]ClusterStatusComponent `json:"components,omitempty"`

	// Operations declares which operations the cluster supports.
	// +optional
	Operations *Operations `json:"operations,omitempty"`

	ClusterDefinitionStatusGeneration `json:",inline"`

	// Describe current state of cluster API Resource, like warning.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ClusterComponent struct {
	// name defines cluster's component name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Component type name defined in ClusterDefinition spec.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Type string `json:"type"`

	// Monitor which is a switch to enable monitoring, default is false
	// DBaas provides an extension mechanism to support component level monitoring,
	// which will scrape metrics auto or manually from servers in component and export
	// metrics to Time Series Database.
	// +kubebuilder:default=false
	// +optional
	Monitor bool `json:"monitor,omitempty"`

	// EnabledLogs indicate which log file takes effect in database cluster
	// element is the log type which defined in cluster definition logConfig.name,
	// and will set relative variables about this log type in database kernel.
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// Component replicas, use default value in ClusterDefinition spec. if not specified.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Affinity describes affinities which specific by users.
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Component tolerations will override ClusterSpec.Tolerations if specified.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Resources requests and limits of workload.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// VolumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

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

// ClusterStatusComponent record components status information
type ClusterStatusComponent struct {
	// Type of component.
	// +optional
	Type string `json:"type,omitempty"`

	// Phase describe the phase of the cluster. the detail information of phase is as follows:
	// Failed: component not available, i.e, all pod is not ready for Stateless/Stateful component;
	// Leader/Primary pod is not ready for Consensus/Replication component.
	// Abnormal: component available but some pod is not ready.
	// If the component type is Consensus/Replication, the Leader/Primary pod is must ready in Abnormal phase.
	// Other phases behave the same as the cluster phase.
	// +kubebuilder:validation:Enum={Running,Failed,Abnormal,Creating,Updating,Deleting,Deleted,VolumeExpanding}
	Phase Phase `json:"phase,omitempty"`

	// Message record the component details message in current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// ConsensusSetStatus role and pod name mapping.
	// +optional
	ConsensusSetStatus *ConsensusSetStatus `json:"consensusSetStatus,omitempty"`
}

type ConsensusSetStatus struct {
	// Leader status.
	// +kubebuilder:validation:Required
	Leader ConsensusMemberStatus `json:"leader"`

	// Followers status.
	// +optional
	Followers []ConsensusMemberStatus `json:"followers,omitempty"`

	// Learner status.
	// +optional
	Learner *ConsensusMemberStatus `json:"learner,omitempty"`
}

type ConsensusMemberStatus struct {
	// Name role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// AccessMode, what service this pod provides.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	// +kubebuilder:default=ReadWrite
	AccessMode AccessMode `json:"accessMode"`

	// Pod name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	Pod string `json:"pod"`
}

type ClusterComponentVolumeClaimTemplate struct {
	// Ref AppVersion.spec.components.containers.volumeMounts.name
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Spec defines the desired characteristics of a volume requested by a pod author.
	// +optional
	Spec *corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

type Affinity struct {
	// PodAntiAffinity defines pods of component anti-affnity.
	// Defaults to Preferred
	// Preferred means try spread pods by topologyKey
	// Required means must spread pods by topologyKey
	// +kubebuilder:validation:Enum={Preferred,Required}
	// +optional
	PodAntiAffinity PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// TopologyKeys describe topologyKeys for `topologySpreadConstraint` and `podAntiAffinity` in ClusterDefinition API.
	// +optional
	TopologyKeys []string `json:"topologyKeys,omitempty"`

	// NodeLabels describe constrain which nodes pod can be scheduled on based on node labels.
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
}

type Operations struct {
	// Upgradable whether the cluster supports upgrade. if multiple appVersions existed, it is true.
	// +optional
	Upgradable bool `json:"upgradable,omitempty"`

	// VerticalScalable which components of the cluster support verticalScaling.
	// +optional
	VerticalScalable []string `json:"verticalScalable,omitempty"`

	// Restartable which components of the cluster support restart.
	// +optional
	Restartable []string `json:"restartable,omitempty"`

	// VolumeExpandable which components of the cluster and its volumeClaimTemplates support volumeExpansion.
	// +optional
	VolumeExpandable []OperationComponent `json:"volumeExpandable,omitempty"`

	// HorizontalScalable which components of the cluster support horizontalScaling, and the replicas range limit.
	// +optional
	HorizontalScalable []OperationComponent `json:"horizontalScalable,omitempty"`
}

type OperationComponent struct {
	// Name reference component name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Min minimum of replicas when operation is horizontalScaling.
	// +optional
	Min int32 `json:"min,omitempty"`

	// Max maximum of replicas when operation is horizontalScaling.
	// +optional
	Max int32 `json:"max,omitempty"`

	// VolumeClaimTemplateNames which VolumeClaimTemplate of the component support volumeExpansion.
	// +optional
	VolumeClaimTemplateNames []string `json:"volumeClaimTemplateNames,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas,all}
//+kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
//+kubebuilder:printcolumn:name="APP-VERSION",type="string",JSONPath=".spec.appVersionRef",description="Cluster Application Version."
//+kubebuilder:printcolumn:name="TERMINATION-POLICY",type="string",JSONPath=".spec.terminationPolicy",description="Cluster termination policy."
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

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}

// ValidateEnabledLogs validates enabledLogs config in cluster.yaml, and returns metav1.Condition when detect invalid values.
func (r *Cluster) ValidateEnabledLogs(cd *ClusterDefinition) []*metav1.Condition {
	conditionList := make([]*metav1.Condition, 0, len(r.Spec.Components))
	for _, comp := range r.Spec.Components {
		invalidLogNames := cd.ValidateEnabledLogConfigs(comp.Type, comp.EnabledLogs)
		if len(invalidLogNames) == 0 {
			continue
		}
		message := fmt.Sprintf("EnabledLogs config of cluster component %s has invalid value %s which isn't definded in clusterDefinition", comp.Name, invalidLogNames)
		conditionList = append(conditionList, &metav1.Condition{
			Type:               "ValidateEnabledLogs",
			Status:             metav1.ConditionFalse,
			Reason:             "ValidateEnabledLogsFail",
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            message,
		})
	}
	return conditionList
}

// GetTypeMappingComponents return Type name mapping ClusterComponents.
func (r *Cluster) GetTypeMappingComponents() map[string][]ClusterComponent {
	m := map[string][]ClusterComponent{}
	for _, c := range r.Spec.Components {
		v := m[c.Type]
		v = append(v, c)
		m[c.Type] = v
	}
	return m
}
