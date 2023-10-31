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
)

// RestoreSpec defines the desired state of Restore
type RestoreSpec struct {
	// backup to be restored. The restore behavior based on the backup type:
	// 1. Full: will be restored the full backup directly.
	// 2. Incremental: will be restored sequentially from the most recent full backup of this incremental backup.
	// 3. Differential: will be restored sequentially from the parent backup of the differential backup.
	// 4. Continuous: will find the most recent full backup at this time point and the continuous backups after it to restore.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupName"
	Backup BackupRef `json:"backup"`

	// restoreTime is the point in time for restoring.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restoreTime"
	// +optional
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`
	RestoreTime string `json:"restoreTime,omitempty"`

	// restore the specified resources of kubernetes.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.resources"
	// +optional
	Resources *RestoreKubeResources `json:"resources,omitempty"`

	// configuration for the action of "prepareData" phase, including the persistent volume claims
	// that need to be restored and scheduling strategy of temporary recovery pod.
	// +optional
	PrepareDataConfig *PrepareDataConfig `json:"prepareDataConfig,omitempty"`

	// service account name which needs for recovery pod.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// configuration for the action of "postReady" phase.
	// +kubebuilder:validation:XValidation:rule="has(self.jobAction) || has(self.execAction)", message="at least one exists for jobAction and execAction."
	// +optional
	ReadyConfig *ReadyConfig `json:"readyConfig,omitempty"`

	// list of environment variables to set in the container for restore and will be
	// merged with the env of Backup and ActionSet.
	// The priority of merging is as follows: Restore env > Backup env > ActionSet env.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// specified the required resources of restore job's container.
	// +optional
	ContainerResources corev1.ResourceRequirements `json:"containerResources,omitempty"`
}

// BackupRef describes the backup name and namespace.
type BackupRef struct {
	// backup name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// backup namespace
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

type RestoreKubeResources struct {
	// will restore the specified resources
	IncludeResources []IncludeResource `json:"included,omitempty"`

	// TODO: supports exclude resources for recovery
}

type IncludeResource struct {
	//
	// +kubebuilder:validation:Required
	GroupResource string `json:"groupResource"`

	// select the specified resource for recovery by label.
	// +optional
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type PrepareDataConfig struct {
	// dataSourceRef describes the configuration when using `persistentVolumeClaim.spec.dataSourceRef` method for restoring.
	// it describes the source volume of the backup targetVolumes and how to mount path in the restoring container.
	// +kubebuilder:validation:XValidation:rule="self.volumeSource != '' || self.mountPath !=''",message="at least one exists for volumeSource and mountPath."
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.dataSourceRef"
	// +optional
	DataSourceRef *VolumeConfig `json:"dataSourceRef,omitempty"`

	// volumeClaims defines the persistent Volume claims that need to be restored and mount them together into the restore job.
	// these persistent Volume claims will be created if not exist.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.volumeClaims"
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	RestoreVolumeClaims []RestoreVolumeClaim `json:"volumeClaims,omitempty"`

	// volumeClaimsTemplate defines a template to build persistent Volume claims that need to be restored.
	// these claims will be created in an orderly manner based on the number of replicas or reused if already exist.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.volumeClaimsTemplate"
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	RestoreVolumeClaimsTemplate *RestoreVolumeClaimsTemplate `json:"volumeClaimsTemplate,omitempty"`

	// VolumeClaimRestorePolicy defines restore policy for persistent volume claim.
	// Supported policies are as follows:
	// 1. Parallel: parallel recovery of persistent volume claim.
	// 2. Serial: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.
	// +kubebuilder:default=Parallel
	// +kubebuilder:validation:Required
	VolumeClaimRestorePolicy VolumeClaimRestorePolicy `json:"volumeClaimRestorePolicy"`

	// scheduling spec for restoring pod.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.prepareDataConfig.schedulingSpec"
	// +optional
	SchedulingSpec SchedulingSpec `json:"schedulingSpec,omitempty"`
}

type ReadyConfig struct {
	// configuration for job action.
	// +optional
	JobAction *JobAction `json:"jobAction,omitempty"`

	// configuration for exec action.
	// +optional
	ExecAction *ExecAction `json:"execAction,omitempty"`

	// credential template used for creating a connection credential
	// +optional
	ConnectionCredential *ConnectionCredential `json:"connectionCredential,omitempty"`

	// periodic probe of the service readiness.
	// controller will perform postReadyHooks of BackupScript.spec.restore after the service readiness when readinessProbe is configured.
	// +optional
	ReadinessProbe *ReadinessProbe `json:"readinessProbe,omitempty"`
}

type JobAction struct {
	// jobActionTarget defines the pod that need to be executed for the job action.
	//  will select a pod that meets the conditions to execute.
	// +kubebuilder:validation:Required
	Target JobActionTarget `json:"target"`
}

type ExecAction struct {
	// execActionTarget defines the pods that need to be executed for the exec action.
	// will execute on all pods that meet the conditions.
	// +optional
	Target ExecActionTarget `json:"target"`
}

type ExecActionTarget struct {
	// kubectl exec in all selected pods.
	// +kubebuilder:validation:Required
	PodSelector metav1.LabelSelector `json:"podSelector"`
}

type JobActionTarget struct {
	// select one of the pods which selected by labels to build the job spec, such as mount required volumes and inject built-in env of the selected pod.
	// +kubebuilder:validation:Required
	PodSelector metav1.LabelSelector `json:"podSelector"`

	// volumeMounts defines which volumes of the selected pod need to be mounted on the restoring pod.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type VolumeConfig struct {
	// volumeSource describes the volume will be restored from the specified volume of the backup targetVolumes.
	// required if the backup uses volume snapshot.
	// +optional
	VolumeSource string `json:"volumeSource,omitempty"`

	// mountPath path within the restoring container at which the volume should be mounted.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

type RestoreVolumeClaim struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +kubebuilder:validation:Required
	metav1.ObjectMeta `json:"metadata"`

	// volumeClaimSpec defines the desired characteristics of a persistent volume claim.
	// +kubebuilder:validation:Required
	VolumeClaimSpec corev1.PersistentVolumeClaimSpec `json:"volumeClaimSpec"`

	// describing the source volume of the backup targetVolumes and how to mount path in the restoring container.
	// +kubebuilder:validation:XValidation:rule="self.volumeSource != '' || self.mountPath !=''",message="at least one exists for volumeSource and mountPath."
	VolumeConfig `json:",inline"`
}

type RestoreVolumeClaimsTemplate struct {
	// templates is a list of volume claims.
	// +kubebuilder:validation:Required
	Templates []RestoreVolumeClaim `json:"templates"`

	// the replicas of persistent volume claim which need to be created and restored.
	// the format of created claim name is "<template-name>-<index>".
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`

	// the starting index for the created persistent volume claim by according to template.
	// minimum is 0.
	// +kubebuilder:validation:Minimum=0
	StartingIndex int32 `json:"startingIndex,omitempty"`
}

type SchedulingSpec struct {
	// the restoring pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// nodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// nodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// affinity is a group of affinity scheduling rules.
	// refer to https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// topologySpreadConstraints describes how a group of pods ought to spread across topology
	// domains. Scheduler will schedule pods in a way which abides by the constraints.
	// refer to https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// If specified, the pod will be dispatched by specified scheduler.
	// If not specified, the pod will be dispatched by default scheduler.
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`
}

type ReadinessProbe struct {
	// number of seconds after the container has started before probe is initiated.
	// +optional
	// +kubebuilder:validation:Minimum=0
	InitialDelaySeconds int `json:"initialDelaySeconds,omitempty"`

	// number of seconds after which the probe times out.
	// defaults to 30 second, minimum value is 1.
	// +optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds int `json:"timeoutSeconds"`

	// how often (in seconds) to perform the probe.
	// defaults to 5 second, minimum value is 1.
	// +optional
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int `json:"periodSeconds"`

	// exec specifies the action to take.
	// +kubebuilder:validation:Required
	Exec ReadinessProbeExecAction `json:"exec"`

	// TODO: support readiness probe by checking k8s resource
}

type ReadinessProbeExecAction struct {
	// refer to container image.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// refer to container command.
	// +kubebuilder:validation:Required
	Command []string `json:"command"`
}

type RestoreStatusActions struct {
	// record the actions for prepareData phase.
	// +patchMergeKey=jobName
	// +patchStrategy=merge,retainKeys
	// +optional
	PrepareData []RestoreStatusAction `json:"prepareData,omitempty"`

	// record the actions for postReady phase.
	// +patchMergeKey=jobName
	// +patchStrategy=merge,retainKeys
	// +optional
	PostReady []RestoreStatusAction `json:"postReady,omitempty"`
}

type RestoreStatusAction struct {
	// name describes the name of the recovery action based on the current backup.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// which backup's restore action belongs to.
	// +kubebuilder:validation:Required
	BackupName string `json:"backupName"`

	// the execution object of the restore action.
	// +kubebuilder:validation:Required
	ObjectKey string `json:"objectKey"`

	// message is a human readable message indicating details about the object condition.
	// +optional
	Message string `json:"message,omitempty"`

	// the status of this action.
	// +kubebuilder:validation:Required
	Status RestoreActionStatus `json:"status,omitempty"`

	// startTime is the start time for the restore job.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`

	// endTime is the completion time for the restore job.
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// Date/time when the restore started being processed.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// Date/time when the restore finished being processed.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// The duration time of restore execution.
	// When converted to a string, the form is "1h2m0.5s".
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// recorded all restore actions performed.
	// +optional
	Actions RestoreStatusActions `json:"actions,omitempty"`

	// describe current state of restore API Resource, like warning.
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
