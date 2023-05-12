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
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicySpec defines the desired state of BackupPolicy
type BackupPolicySpec struct {
	// retention describe how long the Backup should be retained. if not set, will be retained forever.
	// +optional
	Retention *RetentionSpec `json:"retention,omitempty"`

	// schedule policy for backup.
	// +optional
	Schedule Schedule `json:"schedule,omitempty"`

	// the policy for snapshot backup.
	// +optional
	Snapshot *SnapshotPolicy `json:"snapshot,omitempty"`

	// the policy for datafile backup.
	// +optional
	Datafile *CommonBackupPolicy `json:"datafile,omitempty"`

	// the policy for logfile backup.
	// +optional
	Logfile *CommonBackupPolicy `json:"logfile,omitempty"`
}

type RetentionSpec struct {
	// ttl is a time string ending with the 'd'|'D'|'h'|'H' character to describe how long
	// the Backup should be retained. if not set, will be retained forever.
	// +kubebuilder:validation:Pattern:=`^\d+[d|D|h|H]$`
	// +optional
	TTL *string `json:"ttl,omitempty"`
}

type Schedule struct {
	// schedule policy for snapshot backup.
	// +optional
	Snapshot *SchedulePolicy `json:"snapshot,omitempty"`

	// schedule policy for datafile backup.
	// +optional
	Datafile *SchedulePolicy `json:"datafile,omitempty"`

	// schedule policy for logfile backup.
	// +optional
	Logfile *LogSchedulePolicy `json:"logfile,omitempty"`
}

type SchedulePolicy struct {
	// the cron expression for schedule, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// enable or disable the schedule.
	// +kubebuilder:validation:Required
	Enable bool `json:"enable"`
}

type LogSchedulePolicy struct {
	// the interval expression for schedule
	// +kubebuilder:validation:Pattern:=`^\d+[d|D|h|H|m|M]$`
	// +kubebuilder:default="5m"
	Interval string `json:"interval"`

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

	// refer to PersistentVolumeClaim and the backup data will be stored in the corresponding persistent volume.
	// +kubebuilder:validation:Required
	PersistentVolumeClaim PersistentVolumeClaim `json:"persistentVolumeClaim"`

	// which backup tool to perform database backup, only support one tool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	BackupToolName string `json:"backupToolName,omitempty"`
}

type PersistentVolumeClaim struct {
	// the name of the PersistentVolumeClaim.
	Name string `json:"name"`

	// storageClassName is the name of the StorageClass required by the claim.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// initCapacity represents the init storage size of the PersistentVolumeClaim which should be created if not exist.
	// and the default value is 100Gi if it is empty.
	// +optional
	InitCapacity resource.Quantity `json:"initCapacity,omitempty"`

	// createPolicy defines the policy for creating the PersistentVolumeClaim, enum values:
	// - Never: do nothing if the PersistentVolumeClaim not exists.
	// - IfNotPresent: create the PersistentVolumeClaim if not present and the accessModes only contains 'ReadWriteMany'.
	// +kubebuilder:default=IfNotPresent
	// +optional
	CreatePolicy CreatePVCPolicy `json:"createPolicy"`

	// persistentVolumeConfigMap references the configmap which contains a persistentVolume template.
	// key must be "persistentVolume" and value is the "PersistentVolume" struct.
	// support the following built-in Objects:
	// - $(GENERATE_NAME): generate a specific format "pvcName-pvcNamespace".
	// if the PersistentVolumeClaim not exists and CreatePolicy is "IfNotPresent", the controller
	// will create it by this template. this is a mutually exclusive setting with "storageClassName".
	// +optional
	PersistentVolumeConfigMap *PersistentVolumeConfigMap `json:"persistentVolumeConfigMap,omitempty"`
}

type PersistentVolumeConfigMap struct {
	// the name of the persistentVolume ConfigMap.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// the namespace of the persistentVolume ConfigMap.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
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

	// usernameKey the map key of the user in the connection credential secret
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

	// backup policy phase valid value: Available, Failed.
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
	case BackupTypeDataFile:
		return r.Datafile
	case BackupTypeLogFile:
		return r.Logfile
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
