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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicySpec defines the desired state of BackupPolicy
type BackupPolicySpec struct {
	// policy can inherit from backup config and override some fields.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	BackupPolicyTemplateName string `json:"backupPolicyTemplateName,omitempty"`

	// The schedule in Cron format, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Backup ComponentDefRef. full or incremental or snapshot. if unset, default is snapshot.
	// +kubebuilder:validation:Enum={full,incremental,snapshot}
	// +kubebuilder:default=snapshot
	// +optional
	BackupType string `json:"backupType,omitempty"`

	// The number of automatic backups to retain. Value must be non-negative integer.
	// 0 means NO limit on the number of backups.
	// +kubebuilder:default=7
	// +optional
	BackupsHistoryLimit int32 `json:"backupsHistoryLimit,omitempty"`

	// which backup tool to perform database backup, only support one tool.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	BackupToolName string `json:"backupToolName,omitempty"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// database cluster service
	// +kubebuilder:validation:Required
	Target TargetCluster `json:"target"`

	// execute hook commands for backup.
	// +optional
	Hooks *BackupPolicyHook `json:"hooks,omitempty"`

	// array of remote volumes from CSI driver definition.
	// +kubebuilder:validation:Required
	RemoteVolume corev1.Volume `json:"remoteVolume"`

	// count of backup stop retries on fail.
	// +optional
	OnFailAttempted int32 `json:"onFailAttempted,omitempty"`

	// define how to update metadata for backup status.
	// +optional
	BackupStatusUpdates []BackupStatusUpdate `json:"backupStatusUpdates,omitempty"`
}

// TargetCluster TODO (dsj): target cluster need redefined from Cluster API
type TargetCluster struct {
	// LabelSelector is used to find matching pods.
	// Pods that match this label selector are counted to determine the number of pods
	// in their corresponding topology domain.
	// +kubebuilder:validation:Required
	// +kubebuilder:pruning:PreserveUnknownFields
	LabelsSelector *metav1.LabelSelector `json:"labelsSelector"`

	// Secret is used to connect to the target database cluster.
	// If not set, secret will be inherited from backup policy template.
	// if still not set, the controller will check if any system account for dataprotection has been created.
	// +optional
	Secret *BackupPolicySecret `json:"secret,omitempty"`
}

// BackupPolicySecret defined for the target database secret that backup tool can connect.
type BackupPolicySecret struct {
	// the secret name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// UserKeyword the map keyword of the user in the connection credential secret
	// +optional
	UserKeyword string `json:"userKeyword,omitempty"`

	// PasswordKeyword the map keyword of the password in the connection credential secret
	// +optional
	PasswordKeyword string `json:"passwordKeyword,omitempty"`
}

// BackupPolicyHook defined for the database execute commands before and after backup.
type BackupPolicyHook struct {
	// pre backup to perform commands
	// +optional
	PreCommands []string `json:"preCommands,omitempty"`

	// post backup to perform commands
	// +optional
	PostCommands []string `json:"postCommands,omitempty"`

	// exec command with image
	// +optional
	Image string `json:"image,omitempty"`

	// which container can exec command
	// +optional
	ContainerName string `json:"containerName,omitempty"`
}

// BackupStatusUpdateStage defines the stage of backup status update.
// +enum
// +kubebuilder:validation:Enum={pre,post}
type BackupStatusUpdateStage string

const (
	PRE  BackupStatusUpdateStage = "pre"
	POST BackupStatusUpdateStage = "post"
)

type BackupStatusUpdate struct {
	// specify the json path of backup object for patch.
	// example: manifests.backupLog -- means patch the backup jon path of status.manifests.backupLog.
	// +optional
	Path string `json:"path,omitempty"`

	// which container name that kubectl can execute.
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// the shell Script commands to collect backup status metadata.
	// The script must exist in the container of ContainerName and the output format must be set to JSON.
	// Note that outputting to stderr may cause the result format to not be in JSON.
	// +optional
	Script string `json:"script,omitempty"`

	// when to update the backup status, pre: before backup, post: after backup
	// +optional
	UpdateStage BackupStatusUpdateStage `json:"updateStage,omitempty"`
}

// BackupPolicyStatus defines the observed state of BackupPolicy
type BackupPolicyStatus struct {
	// backup policy phase valid value: available, failed, new.
	// +optional
	Phase BackupPolicyTemplatePhase `json:"phase,omitempty"`

	// the reason if backup policy check failed.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// Information when was the last time the job was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// Information when was the last time the job successfully completed.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Namespaced
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="SCHEDULE",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="LAST SCHEDULE",type=string,JSONPath=`.status.lastScheduleTime`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`

// BackupPolicy is the Schema for the backuppolicies API (defined by User)
type BackupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupPolicySpec   `json:"spec,omitempty"`
	Status BackupPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupPolicyList contains a list of BackupPolicy
type BackupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicy{}, &BackupPolicyList{})
}
