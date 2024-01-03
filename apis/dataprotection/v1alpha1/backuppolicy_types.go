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
	corev1 "k8s.io/api/core/v1"
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
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// target specifies the target information to back up.
	// +kubebuilder:validation:Required
	Target *BackupTarget `json:"target"`

	// backupMethods defines the backup methods.
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`
}

type BackupTarget struct {
	// podSelector is used to find the target pod. The volumes of the target pod
	// will be backed up.
	// +kube:validation:Required
	PodSelector *PodSelector `json:"podSelector,omitempty"`

	// connectionCredential specifies the connection credential to connect to the
	// target database cluster.
	// +optional
	ConnectionCredential *ConnectionCredential `json:"connectionCredential,omitempty"`

	// resources specifies the kubernetes resources to back up.
	// +optional
	Resources *KubeResources `json:"resources,omitempty"`

	// serviceAccountName specifies the service account to run the backup workload.
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

type PodSelector struct {
	// labelsSelector is the label selector to filter the target pods.
	*metav1.LabelSelector `json:",inline"`

	// strategy specifies the strategy to select the target pod when multiple pods
	// are selected.
	// Valid values are:
	// - Any: select any one pod that match the labelsSelector.
	// - All: select all pods that match the labelsSelector.
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
	// secretName refers to the Secret object that contains the connection credential.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	SecretName string `json:"secretName"`

	// usernameKey specifies the map key of the user in the connection credential secret.
	// +kubebuilder:default=username
	UsernameKey string `json:"usernameKey,omitempty"`

	// passwordKey specifies the map key of the password in the connection credential secret.
	// This password will be saved in the backup annotation for full backup.
	// You can use the environment variable DP_ENCRYPTION_KEY to specify encryption key.
	// +kubebuilder:default=password
	PasswordKey string `json:"passwordKey,omitempty"`

	// hostKey specifies the map key of the host in the connection credential secret.
	HostKey string `json:"hostKey,omitempty"`

	// portKey specifies the map key of the port in the connection credential secret.
	PortKey string `json:"portKey,omitempty"`
}

// KubeResources defines the kubernetes resources to back up.
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
	// +kubebuilder:default={"*"}
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

	// snapshotVolumes specifies whether to take snapshots of persistent volumes.
	// if true, the BackupScript is not required, the controller will use the CSI
	// volume snapshotter to create the snapshot.
	// +optional
	// +kubebuilder:default=false
	SnapshotVolumes *bool `json:"snapshotVolumes,omitempty"`

	// actionSetName refers to the ActionSet object that defines the backup actions.
	// For volume snapshot backup, the actionSet is not required, the controller
	// will use the CSI volume snapshotter to create the snapshot.
	// +optional
	ActionSetName string `json:"actionSetName,omitempty"`

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

	// target specifies the target information to back up, it will override the global target policy.
	// +optional
	Target *BackupTarget `json:"target,omitempty"`
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

// BackupPolicyStatus defines the observed state of BackupPolicy
type BackupPolicyStatus struct {
	// phase - in list of [Available,Unavailable]
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// A human-readable message indicating details about why the BackupPolicy is
	// in this phase.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the most recent generation observed for this
	// BackupPolicy. It refers to the BackupPolicy's generation, which is
	// updated on mutation by the API Server.
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
