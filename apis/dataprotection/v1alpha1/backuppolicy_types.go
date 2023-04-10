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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicySpec defines the desired state of BackupPolicy
type BackupPolicySpec struct {
	// ttl is a time string in days and ending with the 'd' or 'D' character to describe how long
	// the Backup should be retained. if not set,
	// +kubebuilder:validation:Pattern:=`^\d+[d|D|h|H]$`
	// +optional
	TTL *string `json:"ttl,omitempty"`

	// schedule policy for backup.
	// +optional
	Schedule Schedule `json:"schedule,omitempty"`

	// the policy for snapshot backup.
	// +optional
	Snapshot *SnapshotPolicy `json:"snapshot,omitempty"`

	// the policy for full backup.
	// +optional
	Full *CommonBackupPolicy `json:"full,omitempty"`

	// the policy for incremental backup.
	// +optional
	Incremental *CommonBackupPolicy `json:"incremental,omitempty"`
}

type Schedule struct {
	// schedule policy for base backup.
	// +optional
	BaseBackup *BaseBackupSchedulePolicy `json:"baseBackup,omitempty"`

	// schedule policy for incremental backup.
	// +optional
	Incremental *SchedulePolicy `json:"incremental,omitempty"`
}

type BaseBackupSchedulePolicy struct {
	SchedulePolicy `json:",inline"`
	// the type of base backup, only support full and incremental.
	// +kubebuilder:validation:Required
	Type BaseBackupType `json:"type"`
}

type SchedulePolicy struct {
	// the cron expression for schedule, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// enable or disable the schedule.
	// +kubebuilder:validation:Required
	Enable bool `json:"enable"`
}

type SnapshotPolicy struct {
	BasePolicy `json:",inline"`

	// execute hook commands for backup.
	// +optional
	Hooks *BackupPolicyHook `json:"hooks,omitempty"`
}

type CommonBackupPolicy struct {
	BasePolicy `json:",inline"`

	// array of remote volumes from CSI driver definition.
	// +kubebuilder:validation:Required
	RemoteVolume corev1.Volume `json:"remoteVolume"`

	// which backup tool to perform database backup, only support one tool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	BackupToolName string `json:"backupToolName,omitempty"`
}

type BasePolicy struct {
	// target database cluster for backup.
	// +kubebuilder:validation:Required
	Target TargetCluster `json:"target"`

	// the number of automatic backups to retain. Value must be non-negative integer.
	// 0 means NO limit on the number of backups.
	// +kubebuilder:default=7
	// +optional
	BackupsHistoryLimit int32 `json:"backupsHistoryLimit,omitempty"`

	// count of backup stop retries on fail.
	// +optional
	OnFailAttempted int32 `json:"onFailAttempted,omitempty"`

	// define how to update metadata for backup status.
	// +optional
	BackupStatusUpdates []BackupStatusUpdate `json:"backupStatusUpdates,omitempty"`
}

// TargetCluster TODO (dsj): target cluster need redefined from Cluster API
type TargetCluster struct {
	// labelsSelector is used to find matching pods.
	// Pods that match this label selector are counted to determine the number of pods
	// in their corresponding topology domain.
	// +kubebuilder:validation:Required
	// +kubebuilder:pruning:PreserveUnknownFields
	LabelsSelector *metav1.LabelSelector `json:"labelsSelector"`

	// secret is used to connect to the target database cluster.
	// If not set, secret will be inherited from backup policy template.
	// if still not set, the controller will check if any system account for dataprotection has been created.
	// +optional
	Secret *BackupPolicySecret `json:"secret,omitempty"`
}

// BackupPolicySecret defines for the target database secret that backup tool can connect.
type BackupPolicySecret struct {
	// the secret name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// usernameKey the map keyword of the user in the connection credential secret
	// +kubebuilder:validation:Required
	// +kubebuilder:default=username
	UsernameKey string `json:"usernameKey,omitempty"`

	// passwordKey the map key of the password in the connection credential secret
	// +kubebuilder:validation:Required
	// +kubebuilder:default=password
	PasswordKey string `json:"passwordKey,omitempty"`
}

// BackupPolicyHook defines for the database execute commands before and after backup.
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
	// example: manifests.backupLog -- means patch the backup json path of status.manifests.backupLog.
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

	// observedGeneration is the most recent generation observed for this
	// BackupPolicy. It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// backup policy phase valid value: available, failed, new.
	// +optional
	Phase BackupPolicyPhase `json:"phase,omitempty"`

	// the reason if backup policy check failed.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// information when was the last time the job was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// information when was the last time the job successfully completed.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Namespaced,shortName=bp
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
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

func (r *BackupPolicySpec) GetCommonPolicy(backupType BackupType) *CommonBackupPolicy {
	switch backupType {
	case BackupTypeFull:
		return r.Full
	case BackupTypeIncremental:
		return r.Incremental
	}
	return nil
}

// ToDuration converts the ttl string to time.Duration.
func ToDuration(ttl *string) time.Duration {
	if ttl == nil {
		return time.Duration(0)
	}
	ttlLower := strings.ToLower(*ttl)
	if strings.HasSuffix(ttlLower, "d") {
		days, _ := strconv.Atoi(strings.ReplaceAll(ttlLower, "d", ""))
		return time.Hour * 24 * time.Duration(days)
	}
	hours, _ := strconv.Atoi(strings.ReplaceAll(ttlLower, "h", ""))
	return time.Hour * time.Duration(hours)
}
