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

// BackupPolicySpec defines the desired state of BackupPolicy
// +kubebuilder:validation:XValidation:rule="(has(self.target) && !has(self.targets)) || (has(self.targets) && !has(self.target))",message="either spec.target or spec.targets"
type BackupPolicySpec struct {
	// Specifies the name of BackupRepo where the backup data will be stored.
	// If not set, data will be stored in the default backup repository.
	//
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	BackupRepoName *string `json:"backupRepoName,omitempty"`

	// Specifies the directory inside the backup repository to store the backup.
	// This path is relative to the path of the backup repository.
	//
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// Specifies the number of retries before marking the backup as failed.
	//
	// +optional
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// Specifies the target information to back up, such as the target pod, the
	// cluster connection credential.
	//
	Target *BackupTarget `json:"target,omitempty"`

	// Specifies multiple target information for backup operations. This includes details
	// such as the target pod and cluster connection credentials. All specified targets
	// will be backed up collectively.
	// optional
	Targets []BackupTarget `json:"targets,omitempty"`

	// Defines the backup methods.
	//
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`

	// Specifies whether backup data should be stored in a Kopia repository.
	//
	// Data within the Kopia repository is both compressed and encrypted. Furthermore,
	// data deduplication is implemented across various backups of the same cluster.
	// This approach significantly reduces the actual storage usage, particularly
	// for clusters with a low update frequency.
	//
	// NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition, otherwise the backup will not be processed.
	//
	// +optional
	// +kubebuilder:default=false
	UseKopia bool `json:"useKopia"`

	// Specifies the parameters for encrypting backup data.
	// Encryption will be disabled if the field is not set.
	//
	// +optional
	EncryptionConfig *EncryptionConfig `json:"encryptionConfig,omitempty"`
}

type BackupTarget struct {
	// Specifies a mandatory and unique identifier for each target when using the "targets" field.
	// The backup data for the current target is stored in a uniquely named subdirectory.
	Name string `json:"name,omitempty"`

	// Used to find the target pod. The volumes of the target pod will be backed up.
	//
	// +kube:validation:Required
	PodSelector *PodSelector `json:"podSelector,omitempty"`

	// Specifies the connection credential to connect to the target database cluster.
	//
	// +optional
	ConnectionCredential *ConnectionCredential `json:"connectionCredential,omitempty"`

	// Specifies the kubernetes resources to back up.
	//
	// +optional
	Resources *KubeResources `json:"resources,omitempty"`

	// Specifies the service account to run the backup workload.
	//
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

type PodSelector struct {
	// labelsSelector is the label selector to filter the target pods.
	*metav1.LabelSelector `json:",inline"`

	// Specifies the strategy to select the target pod when multiple pods are selected.
	// Valid values are:
	//
	// - `Any`: select any one pod that match the labelsSelector.
	// - `All`: select all pods that match the labelsSelector. The backup data for the current pod
	// will be stored in a subdirectory named after the pod.
	//
	// +kubebuilder:default=Any
	Strategy PodSelectionStrategy `json:"strategy,omitempty"`
}

// PodSelectionStrategy specifies the strategy to select when multiple pods are
// selected for backup target
// +kubebuilder:validation:Enum={Any,All}
type PodSelectionStrategy string

const (
	// PodSelectionStrategyAll selects all pods that match the labelsSelector.
	// TODO: support PodSelectionStrategyAll
	PodSelectionStrategyAll PodSelectionStrategy = "All"

	// PodSelectionStrategyAny selects any one pod that match the labelsSelector.
	PodSelectionStrategyAny PodSelectionStrategy = "Any"
)

// ConnectionCredential specifies the connection credential to connect to the
// target database cluster.
type ConnectionCredential struct {
	// Refers to the Secret object that contains the connection credential.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	SecretName string `json:"secretName"`

	// Specifies the map key of the user in the connection credential secret.
	//
	// +kubebuilder:default=username
	UsernameKey string `json:"usernameKey,omitempty"`

	// Specifies the map key of the password in the connection credential secret.
	// This password will be saved in the backup annotation for full backup.
	// You can use the environment variable DP_ENCRYPTION_KEY to specify encryption key.
	//
	// +kubebuilder:default=password
	PasswordKey string `json:"passwordKey,omitempty"`

	// Specifies the map key of the host in the connection credential secret.
	//
	// +optional
	HostKey string `json:"hostKey,omitempty"`

	// Specifies the map key of the port in the connection credential secret.
	//
	// +optional
	PortKey string `json:"portKey,omitempty"`
}

// KubeResources defines the kubernetes resources to back up.
type KubeResources struct {
	// A metav1.LabelSelector to filter the target kubernetes resources that need
	// to be backed up. If not set, will do not back up any kubernetes resources.
	//
	// +kube:validation:Required
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// included is a slice of namespaced-scoped resource type names to include in
	// the kubernetes resources.
	// The default value is empty.
	//
	// +optional
	Included []string `json:"included,omitempty"`

	// excluded is a slice of namespaced-scoped resource type names to exclude in
	// the kubernetes resources.
	// The default value is empty.
	//
	// +optional
	Excluded []string `json:"excluded,omitempty"`
}

// BackupMethod defines the backup method.
type BackupMethod struct {
	// The name of backup method.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies whether to take snapshots of persistent volumes. If true,
	// the ActionSetName is not required, the controller will use the CSI volume
	// snapshotter to create the snapshot.
	//
	// +optional
	// +kubebuilder:default=false
	SnapshotVolumes *bool `json:"snapshotVolumes,omitempty"`

	// Refers to the ActionSet object that defines the backup actions.
	// For volume snapshot backup, the actionSet is not required, the controller
	// will use the CSI volume snapshotter to create the snapshot.
	// +optional
	ActionSetName string `json:"actionSetName,omitempty"`

	// Specifies which volumes from the target should be mounted in the backup workload.
	//
	// +optional
	TargetVolumes *TargetVolumeInfo `json:"targetVolumes,omitempty"`

	// Specifies the environment variables for the backup workload.
	//
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies runtime settings for the backup workload container.
	//
	// +optional
	RuntimeSettings *RuntimeSettings `json:"runtimeSettings,omitempty"`

	// Specifies the target information to back up, it will override the target in backup policy.
	//
	// +optional
	Target *BackupTarget `json:"target,omitempty"`

	// Specifies multiple target information for backup operations. This includes details
	// such as the target pod and cluster connection credentials. All specified targets
	// will be backed up collectively.
	Targets []BackupTarget `json:"targets,omitempty"`
}

// TargetVolumeInfo specifies the volumes and their mounts of the targeted application
// that should be mounted in backup workload.
type TargetVolumeInfo struct {
	// Specifies the list of volumes of targeted application that should be mounted
	// on the backup workload.
	//
	// +optional
	Volumes []string `json:"volumes,omitempty"`

	// Specifies the mount for the volumes specified in `volumes` section.
	//
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type RuntimeSettings struct {
	// Specifies the resource required by container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	//
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// BackupPolicyStatus defines the observed state of BackupPolicy
type BackupPolicyStatus struct {
	// Phase - in list of [Available,Unavailable]
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// A human-readable message indicating details about why the BackupPolicy
	// is in this phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the most recent generation observed for this BackupPolicy.
	// It refers to the BackupPolicy's generation, which is updated on mutation by the API Server.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// BackupPolicyPhase defines phases for BackupPolicy.
// +enum
// +kubebuilder:validation:Enum={Available,Failed}
type BackupPolicyPhase string

const (
	BackupPolicyAvailable BackupPolicyPhase = "Available"
	BackupPolicyFailed    BackupPolicyPhase = "Failed"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Namespaced,shortName=bp
// +kubebuilder:printcolumn:name="BACKUP-REPO", type=string, JSONPath=`.spec.backupRepoName`
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`

// BackupPolicy is the Schema for the backuppolicies API.
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
