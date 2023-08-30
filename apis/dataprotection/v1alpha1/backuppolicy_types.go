/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicySpec defines the desired state of BackupPolicy
type BackupPolicySpec struct {
	// backupRepoName is the name of BackupRepo and the backup data will be
	// stored in this repository. If not set, will be stored in the default
	// backup repository.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	BackupRepoName *string `json:"backupRepoName,omitempty"`

	// pathPrefix is the directory inside the backup repository to store the backup content.
	// It is a relative to the path of the backup repository.
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// Specifies the number of retries before marking the backup failed.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=0
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// target specifies the target information to back up.
	// +kubebuilder:validation:Required
	Target *BackupTarget `json:"target"`

	// backupMethods defines the backup methods.
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`
}

type RetentionSpec struct {
	// ttl is a time string ending with the 'd'|'D'|'h'|'H' character to describe how long
	// the Backup should be retained. if not set, will be retained forever.
	// +kubebuilder:validation:Pattern:=`^\d+[d|D|h|H]$`
	// +optional
	TTL *string `json:"ttl,omitempty"`
}

type Schedule struct {
	// startingDeadlineMinutes defines the deadline in minutes for starting the backup job
	// if it misses scheduled time for any reason.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1440
	StartingDeadlineMinutes *int64 `json:"startingDeadlineMinutes,omitempty"`

	// schedule policy for snapshot backup.
	// +optional
	Snapshot *SchedulePolicy `json:"snapshot,omitempty"`

	// schedule policy for datafile backup.
	// +optional
	Datafile *SchedulePolicy `json:"datafile,omitempty"`

	// schedule policy for logfile backup.
	// +optional
	Logfile *SchedulePolicy `json:"logfile,omitempty"`
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

	// refer to PersistentVolumeClaim and the backup data will be stored in the corresponding persistent volume.
	// +optional
	PersistentVolumeClaim PersistentVolumeClaim `json:"persistentVolumeClaim,omitempty"`

	// refer to BackupRepo and the backup data will be stored in the corresponding repo.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	BackupRepoName *string `json:"backupRepoName,omitempty"`

	// which backup tool to perform database backup, only support one tool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	BackupToolName string `json:"backupToolName,omitempty"`
}

type PersistentVolumeClaim struct {
	// the name of PersistentVolumeClaim to store backup data.
	// +optional
	Name *string `json:"name,omitempty"`

	// storageClassName is the name of the StorageClass required by the claim.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
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
	CreatePolicy CreatePVCPolicy `json:"createPolicy,omitempty"`

	// persistentVolumeConfigMap references the configmap which contains a persistentVolume template.
	// key must be "persistentVolume" and value is the "PersistentVolume" struct.
	// support the following built-in Objects:
	// - $(GENERATE_NAME): generate a specific format "`PVC NAME`-`PVC NAMESPACE`".
	// if the PersistentVolumeClaim not exists and CreatePolicy is "IfNotPresent", the controller
	// will create it by this template. this is a mutually exclusive setting with "storageClassName".
	// +optional
	PersistentVolumeConfigMap *PersistentVolumeConfigMap `json:"persistentVolumeConfigMap,omitempty"`
}

type PersistentVolumeConfigMap struct {
	// the name of the persistentVolume ConfigMap.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// the namespace of the persistentVolume ConfigMap.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
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

type BackupTarget struct {
	// podSelector is used to find matching pods.
	// +kube:validation:Required
	PodSelector *PodSelector `json:"podSelector,omitempty"`

	// connectionCredential specifies the connection credential to connect to the
	// target database cluster.
	// +optional
	ConnectionCredential *ConnectionCredential `json:"connectionCredential,omitempty"`
}

type PodSelector struct {
	// labelsSelector is used to find matching pods.
	metav1.LabelSelector `json:",inline"`

	// strategy specifies the strategy to select the target pod when multiple pods
	// are selected.
	// Valid values are:
	// - All: select all pods that match the labelsSelector.
	// - Any: select any one pod that match the labelsSelector.
	// +kubebuilder:default=Any
	Strategy PodSelectionStrategy `json:"strategy,omitempty"`
}

// PodSelectionStrategy specifies the strategy to select when multiple pods are
// selected for backup target
// +kubebuilder:validation:Enum=All;Any
type PodSelectionStrategy string

const (
	// PodSelectionStrategyAll selects all pods that match the labelsSelector.
	PodSelectionStrategyAll PodSelectionStrategy = "All"

	// PodSelectionStrategyAny selects any one pod that match the labelsSelector.
	PodSelectionStrategyAny PodSelectionStrategy = "Any"
)

// ConnectionCredential specifies the connection credential to connect to the
// target database cluster.
type ConnectionCredential struct {
	// secretRef refers to the Secret object that contains the connection credential.
	// +kube:validation:Required
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// usernameKey specifies the map key of the user in the connection credential secret.
	// +kubebuilder:default=username
	UsernameKey string `json:"usernameKey,omitempty"`

	// passwordKey specifies the map key of the password in the connection credential secret.
	// +kubebuilder:default=password
	PasswordKey string `json:"passwordKey,omitempty"`

	// endpointKey specifies the map key of the endpoint in the connection credential secret.
	// +kubebuilder:default=endpoint
	EndpointKey string `json:"endpointKey,omitempty"`

	// hostKey specifies the map key of the host in the connection credential secret.
	// +kubebuilder:default=host
	HostKey string `json:"hostKey,omitempty"`

	// portKey specifies the map key of the port in the connection credential secret.
	// +kubebuilder:default=port
	PortKey string `json:"portKey,omitempty"`
}

// SecretReference represents a Secret Reference. It has enough information to
// retrieve secret in any namespace
type SecretReference struct {
	// Name is unique within a namespace to reference a secret resource.
	// +optional
	Name string `json:"name,omitempty"`
	// Namespace defines the space within which the secret name must be unique.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type KubeResources struct {
	// selector is a metav1.LabelSelector to filter the target kubernetes resources
	// that need to be backed up.
	// If not set, will do not back up any kubernetes resources.
	// +kube:validation:Required
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// included is a slice of namespaced-scoped resource type names to include in
	// the kubernetes resources.
	// The default value is "*", which means all resource types will be included.
	// +optional
	// +kubebuilder:default="*"
	Included []string `json:"included,omitempty"`

	// excluded is a slice of namespaced-scoped resource type names to exclude in
	// the kubernetes resources.
	// The default value is empty.
	// +optional
	Excluded []string `json:"excluded,omitempty"`
}

// BackupMethod defines the backup method.
type BackupMethod struct {
	// the name of backup method.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// useVolumeSnapshot defines whether to use volume snapshot to back up volume.
	// if true, the BackupScript is not required, the controller will use the CSI
	// volume snapshotter to create the snapshot.
	// +optional
	// +kubebuilder:default=false
	UseVolumeSnapshot *bool `json:"useVolumeSnapshot,omitempty"`

	// backupScriptRef refers to the BackupScript object that defines the backup actions.
	// For volume snapshot backup, the backupScript is not required, the controller
	// will use the CSI volume snapshotter to create the snapshot.
	// +optional
	BackupScriptRef string `json:"backupScriptRef,omitempty"`

	// targetVolumes specifies which volumes from the target should be mounted in
	// the backup workload.
	// +optional
	TargetVolumes *TargetVolumeInfo `json:"targetVolumes,omitempty"`

	// env specifies the environment variables for the backup workload.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// runtimeSettings specifies runtime settings for the backup workload container.
	// +optional
	RuntimeSettings *RuntimeSettings `json:"runtimeSettings,omitempty"`
}

// TargetVolumeInfo specifies the volumes and their mounts of the targeted application
// that should be mounted in backup workload.
type TargetVolumeInfo struct {
	// Volumes indicates the list of volumes of targeted application that should
	// be mounted on the backup job.
	// +optional
	Volumes []string `json:"volumes,omitempty"`

	// volumeMounts specifies the mount for the volumes specified in `Volumes` section.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type RuntimeSettings struct {
	// resources specifies the resource required by container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
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

	// useTargetPodServiceAccount defines whether this job requires the service account of the backup target pod.
	// if true, will use the service account of the backup target pod. otherwise, will use the system service account.
	// +optional
	UseTargetPodServiceAccount bool `json:"useTargetPodServiceAccount,omitempty"`

	// when to update the backup status, pre: before backup, post: after backup
	// +kubebuilder:validation:Required
	UpdateStage BackupStatusUpdateStage `json:"updateStage"`
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

// +genclient
// +k8s:openapi-gen=true
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

func (r *BackupPolicySpec) GetCommonSchedulePolicy(backupType BackupType) *SchedulePolicy {
	switch backupType {
	case BackupTypeSnapshot:
		return r.Schedule.Snapshot
	case BackupTypeDataFile:
		return r.Schedule.Datafile
	case BackupTypeLogFile:
		return r.Schedule.Logfile
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

// AddTTL adds tll with hours
func AddTTL(ttl *string, hours int) string {
	if ttl == nil {
		return ""
	}
	ttlLower := strings.ToLower(*ttl)
	if strings.HasSuffix(ttlLower, "d") {
		days, _ := strconv.Atoi(strings.ReplaceAll(ttlLower, "d", ""))
		return fmt.Sprintf("%dh", days*24+hours)
	}
	ttlHours, _ := strconv.Atoi(strings.ReplaceAll(ttlLower, "h", ""))
	return fmt.Sprintf("%dh", ttlHours+hours)
}
