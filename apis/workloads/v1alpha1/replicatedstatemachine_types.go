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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReplicatedStateMachineSpec defines the desired state of ReplicatedStateMachine
type ReplicatedStateMachineSpec struct {
	// Specifies the desired number of replicas of the given Template.
	// These replicas are instantiations of the same Template, with each having a consistent identity.
	// Defaults to 1 if unspecified.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Defines the minimum number of seconds a newly created pod should be ready
	// without any of its container crashing to be considered available.
	// Defaults to 0, meaning the pod will be considered available as soon as it is ready.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// Represents a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`

	// Refers to the name of the service that governs this StatefulSet.
	// This service must exist before the StatefulSet and is responsible for
	// the network identity of the set. Pods get DNS/hostnames that follow a specific pattern.
	ServiceName string `json:"serviceName"`

	// Defines the behavior of a service spec.
	// Provides read-write service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Service *corev1.Service `json:"service,omitempty"`

	// Defines Alternative Services selector pattern specifier.
	// Can be used for creating Readonly service.
	// +optional
	AlternativeServices []corev1.Service `json:"alternativeServices,omitempty"`

	Template corev1.PodTemplateSpec `json:"template"`

	// Overrides values in default Template.
	//
	// Instance is the fundamental unit managed by KubeBlocks.
	// It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
	// A RSM manages instances with a total count of Replicas,
	// and by default, all these instances are generated from the same template.
	// The InstanceTemplate provides a way to override values in the default template,
	// allowing the RSM to manage instances from different templates.
	//
	// The naming convention for instances (pods) based on the RSM Name, InstanceTemplate Name, and ordinal.
	// The constructed instance name follows the pattern: $(rsm.name)-$(template.name)-$(ordinal).
	// By default, the ordinal starts from 0 for each InstanceTemplate.
	// It is important to ensure that the Name of each InstanceTemplate is unique.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the RSM.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies instances to be scaled in with dedicated names in the list.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Represents a list of claims that pods are allowed to reference.
	// The ReplicatedStateMachine controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// Controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down.
	//
	// The default policy is `OrderedReady`, where pods are created in increasing order and the controller waits until each pod is ready before
	// continuing. When scaling down, the pods are removed in the opposite order.
	// The alternative policy is `Parallel` which will create pods in parallel
	// to match the desired scale without waiting, and on scale down will delete
	// all pods at once.
	//
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the RSM when a revision is made to
	// Template.
	// UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil
	UpdateStrategy appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// A list of roles defined in the system.
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// Provides method to probe role.
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// Provides actions to do membership dynamic reconfiguration.
	// +optional
	MembershipReconfiguration *MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// Members(Pods) update strategy.
	//
	// - serial: update Members one by one that guarantee minimum component unavailable time.
	// - bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
	// - parallel: force parallel
	//
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	MemberUpdateStrategy *MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`

	// Indicates that the rsm is paused, meaning the reconciliation of this rsm object will be paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Credential used to connect to DB engine
	// +optional
	Credential *Credential `json:"credential,omitempty"`
}

// ReplicatedStateMachineStatus defines the observed state of ReplicatedStateMachine
type ReplicatedStateMachineStatus struct {
	appsv1.StatefulSetStatus `json:",inline"`

	// Defines the initial number of pods (members) when the cluster is first initialized.
	// This value is set to spec.Replicas at the time of object creation and remains constant thereafter.
	InitReplicas int32 `json:"initReplicas"`

	// Represents the number of pods (members) that have already reached the MembersStatus during the cluster initialization stage.
	// This value remains constant once it equals InitReplicas.
	//
	// +optional
	ReadyInitReplicas int32 `json:"readyInitReplicas,omitempty"`

	// When not empty, indicates the version of the Replicated State Machine (RSM) used to generate the underlying workload.
	//
	// +optional
	CurrentGeneration int64 `json:"currentGeneration,omitempty"`

	// Provides the status of each member in the cluster.
	//
	// +optional
	MembersStatus []MemberStatus `json:"membersStatus,omitempty"`

	// currentRevisions, if not empty, indicates the old version of the RSM used to generate Pods.
	// key is the pod name, value is the revision.
	//
	// +optional
	CurrentRevisions map[string]string `json:"currentRevisions,omitempty"`

	// updateRevisions, if not empty, indicates the new version of the RSM used to generate Pods.
	// key is the pod name, value is the revision.
	//
	// +optional
	UpdateRevisions map[string]string `json:"updateRevisions,omitempty"`
}

// +genclient
// +kubebuilder:skip

// ReplicatedStateMachine is the Schema for the replicatedstatemachines API.
type ReplicatedStateMachine struct {
	// The metadata for the type, like API version and kind.
	metav1.TypeMeta `json:",inline"`

	// Contains the metadata for the particular object, such as name, namespace, labels, and annotations.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of the state machine. It includes the configuration details for the state machine.
	//
	Spec ReplicatedStateMachineSpec `json:"spec,omitempty"`

	// Represents the current information about the state machine. This data may be out of date.
	//
	Status ReplicatedStateMachineStatus `json:"status,omitempty"`
}

// ReplicatedStateMachineList contains a list of ReplicatedStateMachine
type ReplicatedStateMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicatedStateMachine `json:"items"`
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicatedStateMachine) DeepCopyInto(out *ReplicatedStateMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicatedStateMachine.
func (in *ReplicatedStateMachine) DeepCopy() *ReplicatedStateMachine {
	if in == nil {
		return nil
	}
	out := new(ReplicatedStateMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicatedStateMachineList) DeepCopyInto(out *ReplicatedStateMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ReplicatedStateMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicatedStateMachineList.
func (in *ReplicatedStateMachineList) DeepCopy() *ReplicatedStateMachineList {
	if in == nil {
		return nil
	}
	out := new(ReplicatedStateMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicatedStateMachineSpec) DeepCopyInto(out *ReplicatedStateMachineSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(corev1.Service)
		(*in).DeepCopyInto(*out)
	}
	if in.AlternativeServices != nil {
		in, out := &in.AlternativeServices, &out.AlternativeServices
		*out = make([]corev1.Service, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Template.DeepCopyInto(&out.Template)
	if in.Instances != nil {
		in, out := &in.Instances, &out.Instances
		*out = make([]InstanceTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.OfflineInstances != nil {
		in, out := &in.OfflineInstances, &out.OfflineInstances
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.VolumeClaimTemplates != nil {
		in, out := &in.VolumeClaimTemplates, &out.VolumeClaimTemplates
		*out = make([]corev1.PersistentVolumeClaim, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.UpdateStrategy.DeepCopyInto(&out.UpdateStrategy)
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]ReplicaRole, len(*in))
		copy(*out, *in)
	}
	if in.RoleProbe != nil {
		in, out := &in.RoleProbe, &out.RoleProbe
		*out = new(RoleProbe)
		(*in).DeepCopyInto(*out)
	}
	if in.MembershipReconfiguration != nil {
		in, out := &in.MembershipReconfiguration, &out.MembershipReconfiguration
		*out = new(MembershipReconfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.MemberUpdateStrategy != nil {
		in, out := &in.MemberUpdateStrategy, &out.MemberUpdateStrategy
		*out = new(MemberUpdateStrategy)
		**out = **in
	}
	if in.Credential != nil {
		in, out := &in.Credential, &out.Credential
		*out = new(Credential)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicatedStateMachineSpec.
func (in *ReplicatedStateMachineSpec) DeepCopy() *ReplicatedStateMachineSpec {
	if in == nil {
		return nil
	}
	out := new(ReplicatedStateMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicatedStateMachineStatus) DeepCopyInto(out *ReplicatedStateMachineStatus) {
	*out = *in
	in.StatefulSetStatus.DeepCopyInto(&out.StatefulSetStatus)
	if in.MembersStatus != nil {
		in, out := &in.MembersStatus, &out.MembersStatus
		*out = make([]MemberStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CurrentRevisions != nil {
		in, out := &in.CurrentRevisions, &out.CurrentRevisions
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.UpdateRevisions != nil {
		in, out := &in.UpdateRevisions, &out.UpdateRevisions
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicatedStateMachineStatus.
func (in *ReplicatedStateMachineStatus) DeepCopy() *ReplicatedStateMachineStatus {
	if in == nil {
		return nil
	}
	out := new(ReplicatedStateMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ReplicatedStateMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ReplicatedStateMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&ReplicatedStateMachine{}, &ReplicatedStateMachineList{})
}
