/*
Copyright 2023.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sync2FoxLakeTaskSpec defines the desired state of Sync2FoxLakeTask
type Sync2FoxLakeTaskSpec struct {
	SourceEndpoint   Endpoint         `json:"sourceEndpoint"`
	SinkEndpoint     Endpoint         `json:"sinkEndpoint"`
	SyncDatabaseSpec SyncDatabaseSpec `json:"syncDatabaseSpec"`
}

// Sync2FoxLakeTaskStatus defines the observed state of Sync2FoxLakeTask
type Sync2FoxLakeTaskStatus struct {
	TaskStatus        Status `json:"taskStatus"`
	Database          string `json:"database"`
	AppliedSequenceID string `json:"appliedSequenceId"`
	TargetSequenceID  string `json:"targetSequenceId"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="DATABASE",type="string",JSONPath=".status.database",description="synchronized database name"
// +kubebuilder:printcolumn:name="APPLIED-SEQUENCE-ID",type="string",JSONPath=".status.appliedSequenceId",description="synchronized database status: applied-sequence-id"
// +kubebuilder:printcolumn:name="TARGET-SEQUENCE-ID",type="string",JSONPath=".status.targetSequenceId",description="synchronized database status: target-sequence-id"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.taskStatus",description="sync2foxlake task status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Sync2FoxLakeTask is the Schema for the sync2foxlaketasks API
type Sync2FoxLakeTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Sync2FoxLakeTaskSpec   `json:"spec,omitempty"`
	Status Sync2FoxLakeTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Sync2FoxLakeTaskList contains a list of Sync2FoxLakeTask
type Sync2FoxLakeTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sync2FoxLakeTask `json:"items"`
}
