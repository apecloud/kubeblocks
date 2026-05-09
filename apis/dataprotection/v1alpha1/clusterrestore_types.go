/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ClusterRestoreSpec defines the desired state of ClusterRestore.
type ClusterRestoreSpec struct {
	// Specifies the target Cluster name.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.targetClusterName"
	TargetClusterName string `json:"targetClusterName"`

	// Specifies the backup used as the restore source.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupRef"
	BackupRef ClusterRestoreBackupRef `json:"backupRef"`

	// Specifies the template used to create the target Cluster.
	// If omitted, the target Cluster is created from the Cluster snapshot stored in the Backup.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.targetClusterTemplate"
	// +optional
	TargetClusterTemplate *ClusterRestoreTargetClusterTemplate `json:"targetClusterTemplate,omitempty"`

	// Specifies the point in time for restoring.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restoreTime"
	// +optional
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`
	RestoreTime string `json:"restoreTime,omitempty"`

	// Specifies the policy for restoring volume claims of a Cluster's Pods.
	//
	// +kubebuilder:validation:Enum=Parallel;Serial
	// +kubebuilder:default=Parallel
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.volumeRestorePolicy"
	// +optional
	VolumeRestorePolicy VolumeClaimRestorePolicy `json:"volumeRestorePolicy,omitempty"`

	// Controls whether post-ready restore actions are delayed until the whole Cluster is running.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.deferPostReadyUntilClusterRunning"
	// +optional
	DeferPostReadyUntilClusterRunning bool `json:"deferPostReadyUntilClusterRunning,omitempty"`

	// Specifies a list of environment variables to be set in the restore container.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.env"
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Specifies a list of name-value pairs representing restore parameters.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.parameters"
	// +optional
	Parameters []ParameterPair `json:"parameters,omitempty"`
}

// ClusterRestoreBackupRef describes the backup source of a cluster restore.
type ClusterRestoreBackupRef struct {
	// Specifies the backup name.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the backup namespace. If empty, the ClusterRestore namespace is used.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ClusterRestoreTargetClusterTemplate describes the target Cluster to create.
type ClusterRestoreTargetClusterTemplate struct {
	// Specifies labels to set on the target Cluster.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies annotations to set on the target Cluster.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specifies the desired target Cluster spec.
	//
	// +kubebuilder:validation:Required
	Spec appsv1.ClusterSpec `json:"spec"`
}

// ClusterRestoreStatus defines the observed state of ClusterRestore.
type ClusterRestoreStatus struct {
	// The current phase.
	//
	// +optional
	Phase ClusterRestorePhase `json:"phase,omitempty"`

	// The most recent generation observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// References the target Cluster created for this restore.
	//
	// +optional
	TargetClusterRef *ClusterRestoreTargetClusterRef `json:"targetClusterRef,omitempty"`

	// Describes the current state of this restore.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ClusterRestoreTargetClusterRef references a restored Cluster.
type ClusterRestoreTargetClusterRef struct {
	// Specifies the Cluster name.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// Specifies the Cluster namespace.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Specifies the Cluster UID.
	//
	// +optional
	UID types.UID `json:"uid,omitempty"`
}

// ClusterRestorePhase is the high-level state of a cluster restore.
// +enum
// +kubebuilder:validation:Enum={Pending,Preparing,CreatingCluster,Restoring,Completed,Failed}
type ClusterRestorePhase string

const (
	ClusterRestorePhasePending         ClusterRestorePhase = "Pending"
	ClusterRestorePhasePreparing       ClusterRestorePhase = "Preparing"
	ClusterRestorePhaseCreatingCluster ClusterRestorePhase = "CreatingCluster"
	ClusterRestorePhaseRestoring       ClusterRestorePhase = "Restoring"
	ClusterRestorePhaseCompleted       ClusterRestorePhase = "Completed"
	ClusterRestorePhaseFailed          ClusterRestorePhase = "Failed"
)

const (
	// ClusterRestoreReadyCondition is true when the cluster restore has completed.
	ClusterRestoreReadyCondition = "Ready"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=crstr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="BACKUP",type="string",JSONPath=".spec.backupRef.name"
// +kubebuilder:printcolumn:name="TARGET-CLUSTER",type="string",JSONPath=".spec.targetClusterName"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="CREATION-TIME",type=string,JSONPath=".metadata.creationTimestamp"

// ClusterRestore is the Schema for cluster restore requests.
type ClusterRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRestoreSpec   `json:"spec,omitempty"`
	Status ClusterRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterRestoreList contains a list of ClusterRestore.
type ClusterRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterRestore{}, &ClusterRestoreList{})
}
