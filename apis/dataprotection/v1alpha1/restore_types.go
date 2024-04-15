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

// RestoreSpec defines the desired state of Restore
type RestoreSpec struct {
	// Specifies the backup to be restored. The restore behavior is based on the backup type:
	//
	// 1. Full: will be restored the full backup directly.
	// 2. Incremental: will be restored sequentially from the most recent full backup of this incremental backup.
	// 3. Differential: will be restored sequentially from the parent backup of the differential backup.
	// 4. Continuous: will find the most recent full backup at this time point and the continuous backups after it to restore.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupName"
	Backup BackupRef `json:"backup"`

	// Specifies the point in time for restoring.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restoreTime"
	// +optional
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`
	RestoreTime string `json:"restoreTime,omitempty"`

	// Restores the specified resources of Kubernetes.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.resources"
	// +optional
	Resources *RestoreKubeResources `json:"resources,omitempty"`

	// Configuration for the action of "prepareData" phase, including the persistent volume claims
	// that need to be restored and scheduling strategy of temporary recovery pod.
	//
	// +optional
	PrepareDataConfig *PrepareDataConfig `json:"prepareDataConfig,omitempty"`

	// Specifies the service account name needed for recovery pod.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Configuration for the action of "postReady" phase.
	//
	// +kubebuilder:validation:XValidation:rule="has(self.jobAction) || has(self.execAction)", message="at least one exists for jobAction and execAction."
	// +optional
	ReadyConfig *ReadyConfig `json:"readyConfig,omitempty"`

	// List of environment variables to set in the container for restore. These will be
	// merged with the env of Backup and ActionSet.
	//
	// The priority of merging is as follows: `Restore env > Backup env > ActionSet env`.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Specifies the required resources of restore job's container.
	//
	// +optional
	ContainerResources corev1.ResourceRequirements `json:"containerResources,omitempty"`

	// Specifies the number of retries before marking the restore failed.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
}

// BackupRef describes the backup info.
type BackupRef struct {
	// Specifies the backup name.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the backup namespace.
	//
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Specifies the source target for restoration, identified by its name.
	SourceTargetName string `json:"sourceTargetName,omitempty"`
}

type RestoreKubeResources struct {
	// Restores the specified resources.
	//
	// +optional
	IncludeResources []IncludeResource `json:"included,omitempty"`

	// TODO: supports exclude resources for recovery
}

type IncludeResource struct {
	// +kubebuilder:validation:Required
	GroupResource string `json:"groupResource"`

	// Selects the specified resource for recovery by label.
	//
	// +optional
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type PrepareDataConfig struct {

	// Specifies the restore policy, which is required when the pod selection strategy for the source target is 'All'.
	// This field is ignored if the pod selection strategy is 'Any'.
	// optional
	RequiredPolicyForAllPodSelection *RequiredPolicyForAllPodSelection `json:"requiredPolicyForAllPodSelection,omitempty"`

	// Specifies the configuration when using `persistentVolumeClaim.spec.dataSourceRef` method for restoring.
	// Describes the source volume of the backup targetVolumes and the mount path in the restoring container.
	//
	// +kubebuilder:validation:XValidation:rule="self.volumeSource != '' || self.mountPath !=''",message="at least one exists for volumeSource and mountPath."
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.dataSourceRef"
	// +optional
	DataSourceRef *VolumeConfig `json:"dataSourceRef,omitempty"`

	// Defines the persistent Volume claims that need to be restored and mounted together into the restore job.
	// These persistent Volume claims will be created if they do not exist.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.volumeClaims"
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	RestoreVolumeClaims []RestoreVolumeClaim `json:"volumeClaims,omitempty"`

	// Defines a template to build persistent Volume claims that need to be restored.
	// These claims will be created in an orderly manner based on the number of replicas or reused if they already exist.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.volumeClaimsTemplate"
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	RestoreVolumeClaimsTemplate *RestoreVolumeClaimsTemplate `json:"volumeClaimsTemplate,omitempty"`

	// Defines restore policy for persistent volume claim.
	// Supported policies are as follows:
	//
	// - `Parallel`: parallel recovery of persistent volume claim.
	// - `Serial`: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.
	//
	// +kubebuilder:default=Parallel
	// +kubebuilder:validation:Required
	VolumeClaimRestorePolicy VolumeClaimRestorePolicy `json:"volumeClaimRestorePolicy"`

	// Specifies the scheduling spec for the restoring pod.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.schedulingSpec"
	// +optional
	SchedulingSpec SchedulingSpec `json:"schedulingSpec,omitempty"`
}

type ReadyConfig struct {
	// Specifies the configuration for a job action.
	//
	// +optional
	JobAction *JobAction `json:"jobAction,omitempty"`

	// Specifies the configuration for an exec action.
	//
	// +optional
	ExecAction *ExecAction `json:"execAction,omitempty"`

	// Defines the credential template used to create a connection credential.
	//
	// +optional
	ConnectionCredential *ConnectionCredential `json:"connectionCredential,omitempty"`

	// Defines a periodic probe of the service readiness.
	// The controller will perform postReadyHooks of BackupScript.spec.restore
	// after the service readiness when readinessProbe is configured.
	//
	// +optional
	ReadinessProbe *ReadinessProbe `json:"readinessProbe,omitempty"`
}

type JobAction struct {

	// Specifies the restore policy, which is required when the pod selection strategy for the source target is 'All'.
	// This field is ignored if the pod selection strategy is 'Any'.
	// optional
	RequiredPolicyForAllPodSelection *RequiredPolicyForAllPodSelection `json:"requiredPolicyForAllPodSelection,omitempty"`

	// Defines the pods that needs to be executed for the job action.
	//
	// +kubebuilder:validation:Required
	Target JobActionTarget `json:"target"`
}

type RequiredPolicyForAllPodSelection struct {
	// Specifies the data restore policy. Options include:
	// - OneToMany: Enables restoration of all volumes from a single data copy of the original target instance.
	// The 'sourceOfOneToMany' field must be set when using this policy.
	// - OneToOne: Restricts data restoration such that each data piece can only be restored to a single target instance.
	// This is the default policy. When the number of target instances specified for restoration surpasses the count of original backup target instances.
	// +kubebuilder:default=OneToOne
	// +kubebuilder:validation:Required
	DataRestorePolicy DataRestorePolicy `json:"dataRestorePolicy"`

	// Specifies the name of the source target pod. This field is mandatory when the DataRestorePolicy is configured to 'OneToMany'.
	SourceOfOneToMany *SourceOfOneToMany `json:"sourceOfOneToMany,omitempty"`
}

type SourceOfOneToMany struct {
	// Specifies the name of the source target pod.
	// +kubebuilder:validation:Required
	TargetPodName string `json:"targetPodName"`
}

type ExecAction struct {
	// Defines the pods that need to be executed for the exec action.
	// Execution will occur on all pods that meet the conditions.
	//
	// +optional
	Target ExecActionTarget `json:"target"`
}

type ExecActionTarget struct {
	// Executes kubectl in all selected pods.
	//
	// +kubebuilder:validation:Required
	PodSelector metav1.LabelSelector `json:"podSelector"`
}

type JobActionTarget struct {
	// Selects one of the pods, identified by labels, to build the job spec.
	// This includes mounting required volumes and injecting built-in environment variables of the selected pod.
	//
	// +kubebuilder:validation:Required
	PodSelector PodSelector `json:"podSelector"`

	// Defines which volumes of the selected pod need to be mounted on the restoring pod.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type VolumeConfig struct {
	// Describes the volume that will be restored from the specified volume of the backup targetVolumes.
	// This is required if the backup uses a volume snapshot.
	//
	// +optional
	VolumeSource string `json:"volumeSource,omitempty"`

	// Specifies the path within the restoring container at which the volume should be mounted.
	//
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

type RestoreVolumeClaim struct {
	// Specifies the standard metadata for the object.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	//
	// +kubebuilder:validation:Required
	metav1.ObjectMeta `json:"metadata"`

	// Defines the desired characteristics of a persistent volume claim.
	//
	// +kubebuilder:validation:Required
	VolumeClaimSpec corev1.PersistentVolumeClaimSpec `json:"volumeClaimSpec"`

	// Describes the source volume of the backup target volumes and the mount path in the restoring container.
	// At least one must exist for volumeSource and mountPath.
	//
	// +kubebuilder:validation:XValidation:rule="self.volumeSource != '' || self.mountPath !=''",message="at least one exists for volumeSource and mountPath."
	VolumeConfig `json:",inline"`
}

type RestoreVolumeClaimsTemplate struct {
	// Contains a list of volume claims.
	// +kubebuilder:validation:Required
	Templates []RestoreVolumeClaim `json:"templates"`

	// Specifies the replicas of persistent volume claim that need to be created and restored.
	// The format of the created claim name is `$(template-name)-$(index)`.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`

	// Specifies the starting index for the created persistent volume claim according to the template.
	// The minimum value is 0.
	//
	// +kubebuilder:validation:Minimum=0
	StartingIndex int32 `json:"startingIndex,omitempty"`
}

type SchedulingSpec struct {
	// Specifies the tolerations for the restoring pod.
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Defines a selector which must be true for the pod to fit on a node.
	// The selector must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	//
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Specifies a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	//
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Contains a group of affinity scheduling rules.
	// Refer to https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Describes how a group of pods ought to spread across topology
	// domains. The scheduler will schedule pods in a way which abides by the constraints.
	// Refer to https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
	//
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Specifies the scheduler to dispatch the pod.
	// If not specified, the pod will be dispatched by the default scheduler.
	//
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`
}

type ReadinessProbe struct {
	// Specifies the number of seconds after the container has started before the probe is initiated.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	InitialDelaySeconds int `json:"initialDelaySeconds,omitempty"`

	// Specifies the number of seconds after which the probe times out.
	// The default value is 30 seconds, and the minimum value is 1.
	//
	// +optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds int `json:"timeoutSeconds"`

	// Specifies how often (in seconds) to perform the probe.
	// The default value is 5 seconds, and the minimum value is 1.
	//
	// +optional
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int `json:"periodSeconds"`

	// Specifies the action to take.
	//
	// +kubebuilder:validation:Required
	Exec ReadinessProbeExecAction `json:"exec"`

	// TODO: support readiness probe by checking k8s resource
}

type ReadinessProbeExecAction struct {
	// Refers to the container image.
	//
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Refers to the container command.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`
}

type RestoreStatusActions struct {
	// Records the actions for the prepareData phase.
	//
	// +patchMergeKey=jobName
	// +patchStrategy=merge,retainKeys
	// +optional
	PrepareData []RestoreStatusAction `json:"prepareData,omitempty"`

	// Records the actions for the postReady phase.
	//
	// +patchMergeKey=jobName
	// +patchStrategy=merge,retainKeys
	// +optional
	PostReady []RestoreStatusAction `json:"postReady,omitempty"`
}

type RestoreStatusAction struct {
	// Describes the name of the restore action based on the current backup.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Describes which backup's restore action belongs to.
	//
	// +kubebuilder:validation:Required
	BackupName string `json:"backupName"`

	// Describes the execution object of the restore action.
	//
	// +kubebuilder:validation:Required
	ObjectKey string `json:"objectKey"`

	// Provides a human-readable message indicating details about the object condition.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// The status of this action.
	//
	// +kubebuilder:validation:Required
	Status RestoreActionStatus `json:"status,omitempty"`

	// The start time of the restore job.
	//
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`

	// The completion time of the restore job.
	//
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	// Represents the current phase of the restore.
	//
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// Records the date/time when the restore started being processed.
	//
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// Records the date/time when the restore finished being processed.
	//
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// Records the duration of the restore execution.
	// When converted to a string, the form is "1h2m0.5s".
	//
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// Records all restore actions performed.
	//
	// +optional
	Actions RestoreStatusActions `json:"actions,omitempty"`

	// Describes the current state of the restore API Resource, like warning.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all}
// +kubebuilder:printcolumn:name="BACKUP",type="string",JSONPath=".spec.backup.name"
// +kubebuilder:printcolumn:name="RESTORE-TIME",type="string",JSONPath=".spec.restoreTime",description="Point in time for restoring"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Restore Status."
// +kubebuilder:printcolumn:name="DURATION",type=string,JSONPath=".status.duration"
// +kubebuilder:printcolumn:name="CREATION-TIME",type=string,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="COMPLETION-TIME",type=string,JSONPath=".status.completionTimestamp"

// Restore is the Schema for the restores API
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}

func (p *PrepareDataConfig) IsSerialPolicy() bool {
	if p == nil {
		return false
	}
	return p.VolumeClaimRestorePolicy == VolumeClaimRestorePolicySerial
}
