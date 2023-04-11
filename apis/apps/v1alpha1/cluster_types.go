/*
Copyright ApeCloud, Inc.

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
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// Cluster referenced ClusterDefinition name, this is an immutable attribute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// Cluster referenced ClusterVersion name.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// Cluster termination policy. One of DoNotTerminate, Halt, Delete, WipeOut.
	// DoNotTerminate will block delete operation.
	// Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	// Delete is based on Halt and deletes PVCs.
	// WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	// +kubebuilder:validation:Required
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy"`

	// List of componentSpecs you want to replace in ClusterDefinition and ClusterVersion. It will replace the field in ClusterDefinition's and ClusterVersion's component if type is matching.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	ComponentSpecs []ClusterComponentSpec `json:"componentSpecs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// affinity is a group of affinity scheduling rules.
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// tolerations are attached to tolerate any taint that matches the triple <key,value,effect> using the matching operator <operator>.
	// +kubebuilder:pruning:PreserveUnknownFields
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

	// phase describes the phase of the Cluster, the detail information of the phases are as following:
	// Running: cluster is running, all its components are available. [terminal state]
	// Stopped: cluster has stopped, all its components are stopped. [terminal state]
	// Failed: cluster is unavailable. [terminal state]
	// Abnormal: Cluster is still running, but part of its components are Abnormal/Failed. [terminal state]
	// Starting: Cluster has entered starting process.
	// Updating: Cluster has entered updating process, triggered by Spec. updated.
	// Stopping: Cluster has entered a stopping process.
	// if the component workload type is Consensus/Replication, the Leader/Primary pod must be ready in Abnormal phase.
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`

	// message describes cluster details message in current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// components record the current status information of all components of the cluster.
	// +optional
	Components map[string]ClusterComponentStatus `json:"components,omitempty"`

	// clusterDefGeneration represents the generation number of ClusterDefinition referenced.
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// Describe current state of cluster API Resource, like warning.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ClusterComponentSpec struct {
	// name defines cluster's component name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// componentDefRef reference componentDef defined in ClusterDefinition spec.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// monitor which is a switch to enable monitoring, default is false
	// KubeBlocks provides an extension mechanism to support component level monitoring,
	// which will scrape metrics auto or manually from servers in component and export
	// metrics to Time Series Database.
	// +kubebuilder:default=false
	// +optional
	Monitor bool `json:"monitor,omitempty"`

	// enabledLogs indicate which log file takes effect in database cluster
	// element is the log type which defined in cluster definition logConfig.name,
	// and will set relative variables about this log type in database kernel.
	// +listType=set
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// Component replicas, use default value in ClusterDefinition spec. if not specified.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// affinity describes affinities which specific by users.
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Component tolerations will override ClusterSpec.Tolerations if specified.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// resources requests and limits of workload.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// volumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// services expose endpoints can be accessed by clients
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// primaryIndex determines which index is primary when workloadType is Replication, index number starts from zero.
	// +kubebuilder:validation:Minimum=0
	// +optional
	PrimaryIndex *int32 `json:"primaryIndex,omitempty"`

	// switchPolicy defines the strategy for switchover and failover when workloadType is Replication.
	// +optional
	SwitchPolicy *ClusterSwitchPolicy `json:"switchPolicy,omitempty"`

	// tls should be enabled or not
	// +optional
	TLS bool `json:"tls,omitempty"`

	// issuer who provides tls certs
	// required when TLS enabled
	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`
}

type ComponentMessageMap map[string]string

// ClusterComponentStatus record components status information
type ClusterComponentStatus struct {
	// phase describes the phase of the component, the detail information of the phases are as following:
	// Failed: component is unavailable, i.e, all pods are not ready for Stateless/Stateful component;
	// Leader/Primary pod is not ready for Consensus/Replication component.
	// Running: component is running. [terminal state]
	// Stopped: component is stopped, as no running pod. [terminal state]
	// Failed: component has failed to start running. [terminal state]
	// Abnormal: component is running but part of its pods are not ready. [terminal state]
	// Starting: component has entered starting process.
	// Updating: component has entered updating process, triggered by Spec. updated.
	// Stopping: component has entered a stopping process.
	// If the component workload type is Consensus/Replication, the Leader/Primary pod must be ready in Abnormal phase.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// message records the component details message in current phase.
	// keys are podName or deployName or statefulSetName, the format is `<ObjectKind>/<Name>`.
	// +optional
	Message ComponentMessageMap `json:"message,omitempty"`

	// podsReady checks if all pods of the component are ready.
	// +optional
	PodsReady *bool `json:"podsReady,omitempty"`

	// podsReadyTime what time point of all component pods are ready,
	// this time is the ready time of the last component pod.
	// +optional
	PodsReadyTime *metav1.Time `json:"podsReadyTime,omitempty"`

	// consensusSetStatus role and pod name mapping.
	// +optional
	ConsensusSetStatus *ConsensusSetStatus `json:"consensusSetStatus,omitempty"`

	// replicationSetStatus role and pod name mapping.
	// +optional
	ReplicationSetStatus *ReplicationSetStatus `json:"replicationSetStatus,omitempty"`
}

type ConsensusSetStatus struct {
	// leader status.
	// +kubebuilder:validation:Required
	Leader ConsensusMemberStatus `json:"leader"`

	// followers status.
	// +optional
	Followers []ConsensusMemberStatus `json:"followers,omitempty"`

	// learner status.
	// +optional
	Learner *ConsensusMemberStatus `json:"learner,omitempty"`
}

type ConsensusMemberStatus struct {
	// name role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// accessMode, what service this pod provides.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	AccessMode AccessMode `json:"accessMode"`

	// pod name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	Pod string `json:"pod"`
}

type ReplicationSetStatus struct {
	// primary status.
	// +kubebuilder:validation:Required
	Primary ReplicationMemberStatus `json:"primary"`

	// secondaries status.
	// +optional
	Secondaries []ReplicationMemberStatus `json:"secondaries,omitempty"`
}

type ReplicationMemberStatus struct {
	// pod name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	Pod string `json:"pod"`
}

type ClusterSwitchPolicy struct {
	// TODO other attribute extensions

	// clusterSwitchPolicy type defined by Provider in ClusterDefinition, refer components[i].replicationSpec.switchPolicies[x].type
	// +kubebuilder:validation:Required
	// +kubebuilder:default=MaximumAvailability
	// +optional
	Type SwitchPolicyType `json:"type"`
}

type ClusterComponentVolumeClaimTemplate struct {
	// Ref ClusterVersion.spec.components.containers.volumeMounts.name
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// spec defines the desired characteristics of a volume requested by a pod author.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Spec *corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

type Affinity struct {
	// podAntiAffinity describes the anti-affinity level of pods within a component.
	// Preferred means try spread pods by `TopologyKeys`.
	// Required means must spread pods by `TopologyKeys`.
	// +kubebuilder:default=Preferred
	// +optional
	PodAntiAffinity PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// topologyKey is the key of node labels.
	// Nodes that have a label with this key and identical values are considered to be in the same topology.
	// It's used as the topology domain for pod anti-affinity and pod spread constraint.
	// Some well-known label keys, such as "kubernetes.io/hostname" and "topology.kubernetes.io/zone"
	// are often used as TopologyKey, as well as any other custom label key.
	// +listType=set
	// +optional
	TopologyKeys []string `json:"topologyKeys,omitempty"`

	// nodeLabels describes that pods must be scheduled to the nodes with the specified node labels.
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	// tenancy describes how pods are distributed across node.
	// SharedNode means multiple pods may share the same node.
	// DedicatedNode means each pod runs on their own dedicated node.
	// +kubebuilder:default=SharedNode
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`
}

// Issuer defines Tls certs issuer
type Issuer struct {
	// name of issuer
	// options supported:
	// - KubeBlocks - Certificates signed by KubeBlocks Operator.
	// - UserProvided - User provided own CA-signed certificates.
	// +kubebuilder:validation:Enum={KubeBlocks, UserProvided}
	// +kubebuilder:default=KubeBlocks
	// +kubebuilder:validation:Required
	Name IssuerName `json:"name"`

	// secretRef, Tls certs Secret reference
	// required when from is UserProvided
	// +optional
	SecretRef *TLSSecretRef `json:"secretRef,omitempty"`
}

// TLSSecretRef defines Secret contains Tls certs
type TLSSecretRef struct {
	// name of the Secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// ca cert key in Secret
	// +kubebuilder:validation:Required
	CA string `json:"ca"`

	// cert key in Secret
	// +kubebuilder:validation:Required
	Cert string `json:"cert"`

	// key of TLS private key in Secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type ClusterComponentService struct {
	// Service name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

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
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// If ServiceType is LoadBalancer, cloud provider related parameters can be put here
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all}
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.clusterVersionRef",description="Cluster Application Version."
// +kubebuilder:printcolumn:name="TERMINATION-POLICY",type="string",JSONPath=".spec.terminationPolicy",description="Cluster termination policy."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Cluster Status."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

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
func (r *Cluster) ValidateEnabledLogs(cd *ClusterDefinition) error {
	message := make([]string, 0)
	for _, comp := range r.Spec.ComponentSpecs {
		invalidLogNames := cd.ValidateEnabledLogConfigs(comp.ComponentDefRef, comp.EnabledLogs)
		if len(invalidLogNames) == 0 {
			continue
		}
		message = append(message, fmt.Sprintf("EnabledLogs: %s are not defined in Component: %s of the clusterDefinition", invalidLogNames, comp.Name))
	}
	if len(message) > 0 {
		return errors.New(strings.Join(message, ";"))
	}
	return nil
}

// GetDefNameMappingComponents returns ComponentDefRef name mapping ClusterComponentSpec.
func (r *Cluster) GetDefNameMappingComponents() map[string][]ClusterComponentSpec {
	m := map[string][]ClusterComponentSpec{}
	for _, c := range r.Spec.ComponentSpecs {
		v := m[c.ComponentDefRef]
		v = append(v, c)
		m[c.ComponentDefRef] = v
	}
	return m
}

// GetMessage gets message map deep copy object
func (in *ClusterComponentStatus) GetMessage() ComponentMessageMap {
	messageMap := map[string]string{}
	for k, v := range in.Message {
		messageMap[k] = v
	}
	return messageMap
}

// SetMessage override message map object
func (in *ClusterComponentStatus) SetMessage(messageMap ComponentMessageMap) {
	if in == nil {
		return
	}
	in.Message = messageMap
}

// SetObjectMessage sets k8s workload message to component status message map
func (in *ClusterComponentStatus) SetObjectMessage(objectKind, objectName, message string) {
	if in == nil {
		return
	}
	if in.Message == nil {
		in.Message = map[string]string{}
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	in.Message[messageKey] = message
}

// GetObjectMessage gets the k8s workload message in component status message map
func (in *ClusterComponentStatus) GetObjectMessage(objectKind, objectName string) string {
	if in == nil {
		return ""
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	return in.Message[messageKey]
}

// SetObjectMessage sets k8s workload message to component status message map
func (m ComponentMessageMap) SetObjectMessage(objectKind, objectName, message string) {
	if m == nil {
		return
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	m[messageKey] = message
}

// GetComponentByName gets component by name.
func (r *Cluster) GetComponentByName(componentName string) *ClusterComponentSpec {
	for _, v := range r.Spec.ComponentSpecs {
		if v.Name == componentName {
			return &v
		}
	}
	return nil
}

// GetComponentDefRefName gets the name of referenced component definition.
func (r *Cluster) GetComponentDefRefName(componentName string) string {
	for _, component := range r.Spec.ComponentSpecs {
		if componentName == component.Name {
			return component.ComponentDefRef
		}
	}
	return ""
}

// SetComponentStatus does safe operation on ClusterStatus.Components map object update.
func (r *ClusterStatus) SetComponentStatus(name string, status ClusterComponentStatus) {
	if r == nil {
		return
	}
	r.checkedInitComponentsMap()
	r.Components[name] = status
}

func (r *ClusterStatus) checkedInitComponentsMap() {
	if r.Components == nil {
		r.Components = map[string]ClusterComponentStatus{}
	}
}

// ToVolumeClaimTemplates convert r.VolumeClaimTemplates to []corev1.PersistentVolumeClaimTemplate.
func (r *ClusterComponentSpec) ToVolumeClaimTemplates() []corev1.PersistentVolumeClaimTemplate {
	if r == nil {
		return nil
	}
	var ts []corev1.PersistentVolumeClaimTemplate
	for _, template := range r.VolumeClaimTemplates {
		ts = append(ts, toVolumeClaimTemplate(template))
	}
	return ts
}

// GetPrimaryIndex provide safe operation get ClusterComponentSpec.PrimaryIndex, if value is nil, it's treated as 0.
func (r *ClusterComponentSpec) GetPrimaryIndex() int32 {
	if r == nil || r.PrimaryIndex == nil {
		return 0
	}
	return *r.PrimaryIndex
}

// GetClusterTerminalPhases return Cluster terminal phases.
func GetClusterTerminalPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		StoppedClusterPhase,
		FailedClusterPhase,
		AbnormalClusterPhase,
	}
}

// GetClusterUpRunningPhases return Cluster running or partially running phases.
func GetClusterUpRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		AbnormalClusterPhase,
		// FailedClusterPhase, // REVIEW/TODO: single component with single pod component are handled as FailedClusterPhase, ought to remove this.
	}
}

// GetClusterFailedPhases return Cluster failed or partially failed phases.
func GetClusterFailedPhases() []ClusterPhase {
	return []ClusterPhase{
		FailedClusterPhase,
		AbnormalClusterPhase,
	}
}

// GetComponentTerminalPhases return Cluster's component terminal phases.
func GetComponentTerminalPhases() []ClusterComponentPhase {
	return []ClusterComponentPhase{
		RunningClusterCompPhase,
		StoppedClusterCompPhase,
		FailedClusterCompPhase,
		AbnormalClusterCompPhase,
	}
}

func toVolumeClaimTemplate(template ClusterComponentVolumeClaimTemplate) corev1.PersistentVolumeClaimTemplate {
	t := corev1.PersistentVolumeClaimTemplate{}
	t.ObjectMeta.Name = template.Name
	if template.Spec != nil {
		t.Spec = *template.Spec
	}
	return t
}
