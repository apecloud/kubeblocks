/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	defaultInstanceTemplateReplicas = 1
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:storageversion
// +kubebuilder:resource:categories={kubeblocks},shortName=its
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.readyReplicas",description="ready replicas."
// +kubebuilder:printcolumn:name="DESIRED",type="string",JSONPath=".spec.replicas",description="desired replicas."
// +kubebuilder:printcolumn:name="UP-TO-DATE",type="string",JSONPath=".status.updatedReplicas",description="updated replicas."
// +kubebuilder:printcolumn:name="AVAILABLE",type="string",JSONPath=".status.availableReplicas",description="available replicas, which are ready for at least minReadySeconds."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// InstanceSet is the Schema for the instancesets API.
type InstanceSet struct {
	// The metadata for the type, like API version and kind.
	metav1.TypeMeta `json:",inline"`

	// Contains the metadata for the particular object, such as name, namespace, labels, and annotations.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of the state machine. It includes the configuration details for the state machine.
	//
	Spec InstanceSetSpec `json:"spec,omitempty"`

	// Represents the current information about the state machine. This data may be out of date.
	//
	Status InstanceSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceSetList contains a list of InstanceSet
type InstanceSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstanceSet{}, &InstanceSetList{})
}

// InstanceSetSpec defines the desired state of InstanceSet
type InstanceSetSpec struct {
	// Specifies the desired number of replicas of the given Template.
	// These replicas are instantiations of the same Template, with each having a consistent identity.
	// Defaults to 1 if unspecified.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the desired Ordinals of the default template.
	// The Ordinals used to specify the ordinal of the instance (pod) names to be generated under the default template.
	//
	// For example, if Ordinals is {ranges: [{start: 0, end: 1}], discrete: [7]},
	// then the instance names generated under the default template would be
	// $(cluster.name)-$(component.name)-0、$(cluster.name)-$(component.name)-1 and $(cluster.name)-$(component.name)-7
	DefaultTemplateOrdinals kbappsv1.Ordinals `json:"defaultTemplateOrdinals,omitempty"`

	// Defines the minimum number of seconds a newly created pod should be ready
	// without any of its container crashing to be considered available.
	// Defaults to 0, meaning the pod will be considered available as soon as it is ready.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// Represents a label query over pods that should match the desired replica count indicated by the `replica` field.
	// It must match the labels defined in the pod template.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`

	Template corev1.PodTemplateSpec `json:"template"`

	// Overrides values in default Template.
	//
	// Instance is the fundamental unit managed by KubeBlocks.
	// It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
	// An InstanceSet manages instances with a total count of Replicas,
	// and by default, all these instances are generated from the same template.
	// The InstanceTemplate provides a way to override values in the default template,
	// allowing the InstanceSet to manage instances from different templates.
	//
	// The naming convention for instances (pods) based on the InstanceSet Name, InstanceTemplate Name, and ordinal.
	// The constructed instance name follows the pattern: $(instance_set.name)-$(template.name)-$(ordinal).
	// By default, the ordinal starts from 0 for each InstanceTemplate.
	// It is important to ensure that the Name of each InstanceTemplate is unique.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the InstanceSet.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
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
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Specifies a list of PersistentVolumeClaim templates that define the storage requirements for each replica.
	// Each template specifies the desired characteristics of a persistent volume, such as storage class,
	// size, and access modes.
	// These templates are used to dynamically provision persistent volumes for replicas upon their creation.
	// The final name of each PVC is generated by appending the pod's identifier to the name specified in volumeClaimTemplates[*].name.
	//
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// persistentVolumeClaimRetentionPolicy describes the lifecycle of persistent
	// volume claims created from volumeClaimTemplates. By default, all persistent
	// volume claims are created as needed and retained until manually deleted. This
	// policy allows the lifecycle to be altered, for example by deleting persistent
	// volume claims when their workload is deleted, or when their pod is scaled
	// down.
	//
	// +optional
	PersistentVolumeClaimRetentionPolicy *PersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`

	// Controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down.
	//
	// The default policy is `OrderedReady`, where pods are created in increasing order and the controller waits until each pod is ready before
	// continuing. When scaling down, the pods are removed in the opposite order.
	// The alternative policy is `Parallel` which will create pods in parallel
	// to match the desired scale without waiting, and on scale down will delete
	// all pods at once.
	//
	// Note: This field will be removed in future version.
	//
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

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
	PodUpdatePolicy PodUpdatePolicyType `json:"podUpdatePolicy,omitempty"`

	// Provides fine-grained control over the spec update process of all instances.
	//
	// +optional
	InstanceUpdateStrategy *InstanceUpdateStrategy `json:"instanceUpdateStrategy,omitempty"`

	// Members(Pods) update strategy.
	//
	// - serial: update Members one by one that guarantee minimum component unavailable time.
	// - parallel: force parallel
	// - bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
	//
	// +kubebuilder:validation:Enum={Serial,Parallel,BestEffortParallel}
	// +optional
	MemberUpdateStrategy *MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`

	// A list of roles defined in the system. Instanceset obtains role through pods' role label `kubeblocks.io/role`.
	//
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// Provides actions to do membership dynamic reconfiguration.
	//
	// +optional
	MembershipReconfiguration *MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// Provides variables which are used to call Actions.
	//
	// +optional
	TemplateVars map[string]string `json:"templateVars,omitempty"`

	// Indicates that the InstanceSet is paused, meaning the reconciliation of this InstanceSet object will be paused.
	//
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Describe the configs to be reconfigured.
	//
	// +optional
	Configs []ConfigTemplate `json:"configs,omitempty"`
}

// InstanceSetStatus defines the observed state of InstanceSet
type InstanceSetStatus struct {
	// observedGeneration is the most recent generation observed for this InstanceSet. It corresponds to the
	// InstanceSet's generation, which is updated on mutation by the API Server.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// replicas is the number of instances created by the InstanceSet controller.
	Replicas int32 `json:"replicas"`

	// readyReplicas is the number of instances created for this InstanceSet with a Ready Condition.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// currentReplicas is the number of instances created by the InstanceSet controller from the InstanceSet version
	// indicated by CurrentRevisions.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// updatedReplicas is the number of instances created by the InstanceSet controller from the InstanceSet version
	// indicated by UpdateRevisions.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// currentRevision, if not empty, indicates the version of the InstanceSet used to generate instances in the
	// sequence [0,currentReplicas).
	CurrentRevision string `json:"currentRevision,omitempty"`

	// updateRevision, if not empty, indicates the version of the InstanceSet used to generate instances in the sequence
	// [replicas-updatedReplicas,replicas)
	UpdateRevision string `json:"updateRevision,omitempty"`

	// Represents the latest available observations of an instanceset's current state.
	// Known .status.conditions.type are: "InstanceFailure", "InstanceReady"
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Total number of available instances (ready for at least minReadySeconds) targeted by this InstanceSet.
	//
	// +optional
	AvailableReplicas int32 `json:"availableReplicas"`

	// Defines the initial number of instances when the cluster is first initialized.
	// This value is set to spec.Replicas at the time of object creation and remains constant thereafter.
	// Used only when spec.roles set.
	//
	// +optional
	InitReplicas int32 `json:"initReplicas"`

	// Represents the number of instances that have already reached the MembersStatus during the cluster initialization stage.
	// This value remains constant once it equals InitReplicas.
	// Used only when spec.roles set.
	//
	// +optional
	ReadyInitReplicas int32 `json:"readyInitReplicas,omitempty"`

	// Provides the status of each member in the cluster.
	//
	// +optional
	MembersStatus []MemberStatus `json:"membersStatus,omitempty"`

	// Provides the status of each instance in the ITS.
	//
	// +optional
	InstanceStatus []InstanceStatus `json:"instanceStatus,omitempty"`

	// currentRevisions, if not empty, indicates the old version of the InstanceSet used to generate the underlying workload.
	// key is the pod name, value is the revision.
	//
	// +optional
	CurrentRevisions map[string]string `json:"currentRevisions,omitempty"`

	// updateRevisions, if not empty, indicates the new version of the InstanceSet used to generate the underlying workload.
	// key is the pod name, value is the revision.
	//
	// +optional
	UpdateRevisions map[string]string `json:"updateRevisions,omitempty"`

	// TemplatesStatus represents status of each instance generated by InstanceTemplates
	// +optional
	TemplatesStatus []InstanceTemplateStatus `json:"templatesStatus,omitempty"`
}

// PersistentVolumeClaimRetentionPolicy describes the policy used for PVCs created from the VolumeClaimTemplates.
//
// +kubebuilder:object:generate=false
type PersistentVolumeClaimRetentionPolicy = kbappsv1.PersistentVolumeClaimRetentionPolicy

// Ordinals represents a combination of continuous segments and individual values.
//
// +kubebuilder:object:generate=false
type Ordinals = kbappsv1.Ordinals

// SchedulingPolicy defines the scheduling policy for instances.
//
// +kubebuilder:object:generate=false
type SchedulingPolicy = kbappsv1.SchedulingPolicy

// InstanceTemplate allows customization of individual replica configurations in a Component.
type InstanceTemplate struct {
	// Name specifies the unique name of the instance Pod created using this InstanceTemplate.
	// This name is constructed by concatenating the Component's name, the template's name, and the instance's ordinal
	// using the pattern: $(cluster.name)-$(component.name)-$(template.name)-$(ordinal). Ordinals start from 0.
	// The specified name overrides any default naming conventions or patterns.
	//
	// +kubebuilder:validation:MaxLength=54
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the number of instances (Pods) to create from this InstanceTemplate.
	// This field allows setting how many replicated instances of the Component,
	// with the specific overrides in the InstanceTemplate, are created.
	// The default value is 1. A value of 0 disables instance creation.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the desired Ordinals of this InstanceTemplate.
	// The Ordinals used to specify the ordinal of the instance (pod) names to be generated under this InstanceTemplate.
	//
	// For example, if Ordinals is {ranges: [{start: 0, end: 1}], discrete: [7]},
	// then the instance names generated under this InstanceTemplate would be
	// $(cluster.name)-$(component.name)-$(template.name)-0、$(cluster.name)-$(component.name)-$(template.name)-1 and
	// $(cluster.name)-$(component.name)-$(template.name)-7
	Ordinals Ordinals `json:"ordinals,omitempty"`

	// Specifies a map of key-value pairs to be merged into the Pod's existing annotations.
	// Existing keys will have their values overwritten, while new keys will be added to the annotations.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specifies a map of key-value pairs that will be merged into the Pod's existing labels.
	// Values for existing keys will be overwritten, and new keys will be added.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies the scheduling policy for the Component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies an override for the resource requirements of the first container in the Pod.
	// This field allows for customizing resource allocation (CPU, memory, etc.) for the container.
	//
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Defines Env to override.
	// Add new or override existing envs.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

func (t *InstanceTemplate) GetName() string {
	return t.Name
}

func (t *InstanceTemplate) GetReplicas() int32 {
	if t.Replicas != nil {
		return *t.Replicas
	}
	return defaultInstanceTemplateReplicas
}

func (t *InstanceTemplate) GetOrdinals() Ordinals {
	return t.Ordinals
}

// PodUpdatePolicyType indicates how pods should be updated
//
// +kubebuilder:object:generate=false
type PodUpdatePolicyType = kbappsv1.PodUpdatePolicyType

// InstanceUpdateStrategy defines fine-grained control over the spec update process of all instances.
//
// +kubebuilder:object:generate=false
type InstanceUpdateStrategy = kbappsv1.InstanceUpdateStrategy

// RollingUpdate specifies how the rolling update should be applied.
//
// +kubebuilder:object:generate=false
type RollingUpdate = kbappsv1.RollingUpdate

// MemberUpdateStrategy defines Cluster Component update strategy.
// +enum
type MemberUpdateStrategy string

const (
	SerialUpdateStrategy             MemberUpdateStrategy = "Serial"
	ParallelUpdateStrategy           MemberUpdateStrategy = "Parallel"
	BestEffortParallelUpdateStrategy MemberUpdateStrategy = "BestEffortParallel"
)

// ReplicaRole represents a role that can be assigned to a component instance, defining its behavior and responsibilities.
// +kubebuilder:object:generate=false
type ReplicaRole = kbappsv1.ReplicaRole

type MembershipReconfiguration struct {
	// Defines the procedure for a controlled transition of a role to a new replica.
	//
	// +optional
	Switchover *kbappsv1.Action `json:"switchover,omitempty"`

	// TODO: member join/leave
}

type ConfigTemplate struct {
	// The name of the config.
	Name string `json:"name"`

	// The generation of the config.
	Generation int64 `json:"generation"`

	// The custom reconfigure action.
	//
	// +optional
	Reconfigure *kbappsv1.Action `json:"reconfigure,omitempty"`

	// The name of the custom reconfigure action.
	//
	// An empty name indicates that the reconfigure action is the default one defined by lifecycle actions.
	//
	// +optional
	ReconfigureActionName string `json:"reconfigureActionName,omitempty"`

	// The parameters to call the reconfigure action.
	//
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

type MemberStatus struct {
	// Represents the name of the pod.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	PodName string `json:"podName"`

	// Defines the role of the replica in the cluster.
	//
	// +optional
	ReplicaRole *ReplicaRole `json:"role,omitempty"`
}

type InstanceStatus struct {
	// Represents the name of the pod.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	PodName string `json:"podName"`

	// The status of configs.
	//
	// +optional
	Configs []InstanceConfigStatus `json:"configs,omitempty"`
}

type InstanceConfigStatus struct {
	// The name of the config.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The generation of the config.
	//
	// +kubebuilder:validation:Required
	Generation int64 `json:"generation"`
}

// InstanceTemplateStatus aggregates the status of replicas for each InstanceTemplate
type InstanceTemplateStatus struct {
	// Name, the name of the InstanceTemplate.
	Name string `json:"name"`

	// Replicas is the number of replicas of the InstanceTemplate.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of Pods that have a Ready Condition.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of Pods that ready for at least minReadySeconds.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// currentReplicas is the number of instances created by the InstanceSet controller from the InstanceSet version
	// indicated by CurrentRevisions.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// UpdatedReplicas is the number of Pods created by the InstanceSet controller from the InstanceSet version
	// indicated by UpdateRevisions.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
}

type ConditionType string

const (
	// InstanceReady is added in an instance set when at least one of its instances(pods) is in a Ready condition.
	// ConditionStatus will be True if all its instances(pods) are in a Ready condition.
	// Or, a NotReady reason with not ready instances encoded in the Message filed will be set.
	InstanceReady ConditionType = "InstanceReady"

	// InstanceAvailable ConditionStatus will be True if all instances(pods) are in the ready condition
	// and continue for "MinReadySeconds" seconds. Otherwise, it will be set to False.
	InstanceAvailable ConditionType = "InstanceAvailable"

	// InstanceFailure is added in an instance set when at least one of its instances(pods) is in a `Failed` phase.
	InstanceFailure ConditionType = "InstanceFailure"

	// InstanceUpdateRestricted represents a ConditionType that indicates updates to an InstanceSet are blocked(when the
	// PodUpdatePolicy is set to StrictInPlace but the pods cannot be updated in-place).
	InstanceUpdateRestricted ConditionType = "InstanceUpdateRestricted"
)

const (
	// ReasonNotReady is a reason for condition InstanceReady.
	ReasonNotReady = "NotReady"

	// ReasonReady is a reason for condition InstanceReady.
	ReasonReady = "Ready"

	// ReasonNotAvailable is a reason for condition InstanceAvailable.
	ReasonNotAvailable = "NotAvailable"

	// ReasonAvailable is a reason for condition InstanceAvailable.
	ReasonAvailable = "Available"

	// ReasonInstanceFailure is a reason for condition InstanceFailure.
	ReasonInstanceFailure = "InstanceFailure"

	// ReasonInstanceUpdateRestricted is a reason for condition InstanceUpdateRestricted.
	ReasonInstanceUpdateRestricted = "InstanceUpdateRestricted"
)

// IsInstancesReady gives Instance level 'ready' state when all instances are available
func (r *InstanceSet) IsInstancesReady() bool {
	if r == nil {
		return false
	}
	// check whether the cluster has been initialized
	if r.Status.ReadyInitReplicas != r.Status.InitReplicas {
		return false
	}
	// check whether latest spec has been sent to the underlying workload
	if r.Status.ObservedGeneration != r.Generation {
		return false
	}
	// check whether the underlying workload is ready
	if r.Spec.Replicas == nil {
		return false
	}
	replicas := *r.Spec.Replicas
	if r.Status.Replicas != replicas ||
		r.Status.ReadyReplicas != replicas ||
		r.Status.UpdatedReplicas != replicas {
		return false
	}
	// check availableReplicas only if minReadySeconds is set
	if r.Spec.MinReadySeconds > 0 && r.Status.AvailableReplicas != replicas {
		return false
	}

	return true
}

// IsInstanceSetReady gives InstanceSet level 'ready' state:
// 1. all instances are available
// 2. and all members have role set (if they are role-ful)
func (r *InstanceSet) IsInstanceSetReady() bool {
	instancesReady := r.IsInstancesReady()
	if !instancesReady {
		return false
	}

	// check whether role probe has done
	if len(r.Spec.Roles) == 0 {
		return true
	}
	membersStatus := r.Status.MembersStatus
	return len(membersStatus) == int(*r.Spec.Replicas)
}
