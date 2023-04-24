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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigrationTaskSpec defines the desired state of MigrationTask
type MigrationTaskSpec struct {
	TaskType       TaskTypeEnum `json:"taskType,omitempty"`
	Template       string       `json:"template"`
	SourceEndpoint Endpoint     `json:"sourceEndpoint,omitempty"`
	SinkEndpoint   Endpoint     `json:"sinkEndpoint,omitempty"`
	// +optional
	Cdc CdcConfig `json:"cdc,omitempty"`
	// +optional
	Initialization InitializationConfig   `json:"initialization,omitempty"`
	MigrationObj   MigrationObjectExpress `json:"migrationObj,omitempty"`
	// +optional
	IsForceDelete bool `json:"isForceDelete,omitempty"`
	// +optional
	GlobalTolerations []v1.Toleration `json:"globalTolerations,omitempty"`
	// +optional
	GlobalResources v1.ResourceRequirements `json:"globalResources,omitempty"`
}

type Endpoint struct {
	// +optional
	EndpointType EndpointTypeEnum `json:"endpointType,omitempty"`
	Address      string           `json:"address"`
	// +optional
	DatabaseName string `json:"databaseName,omitempty"`
	// +optional
	UserName string `json:"userName"`
	// +optional
	Password string `json:"password"`
	// +optional
	Secret UserPswSecret `json:"secret"`
}

type UserPswSecret struct {
	Name string `json:"name"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +optional
	UserKeyword string `json:"userKeyword,omitempty"`
	// +optional
	PasswordKeyword string `json:"passwordKeyword,omitempty"`
}

type CdcConfig struct {
	// +optional
	Config BaseConfig `json:"config"`
}

type InitializationConfig struct {
	// +optional
	Steps []StepEnum `json:"steps,omitempty"`
	// +optional
	Config map[StepEnum]BaseConfig `json:"config,omitempty"`
}

type BaseConfig struct {
	// +optional
	Resource v1.ResourceRequirements `json:"resource,omitempty"`
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Param IntOrStringMap `json:"param"`
	// +optional
	PersistentVolumeClaimName string `json:"persistentVolumeClaimName"`
	// +optional
	Metrics Metrics `json:"metrics,omitempty"`
}

// MigrationTaskStatus defines the observed state of MigrationTask
type MigrationTaskStatus struct {
	// +optional
	TaskStatus TaskStatus `json:"taskStatus"`
	// +optional
	StartTime *metav1.Time `json:"startTime"`
	// +optional
	FinishTime *metav1.Time `json:"finishTime"`
	// +optional
	Cdc RunTimeStatus `json:"cdc"`
	// +optional
	Initialization RunTimeStatus `json:"initialization"`
}

type RunTimeStatus struct {
	// +optional
	StartTime *metav1.Time `json:"startTime"`
	// +optional
	FinishTime *metav1.Time `json:"finishTime"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	RunTimeParam IntOrStringMap `json:"runTimeParam,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Metrics IntOrStringMap `json:"metrics,omitempty"`
	// +optional
	FailedReason string `json:"failedReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={dtplatform},scope=Cluster,shortName=mt
// +kubebuilder:printcolumn:name="TEMPLATE",type="string",JSONPath=".spec.template",description="spec migration template"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.taskStatus",description="status taskStatus"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// MigrationTask is the Schema for the migrationTasks API
type MigrationTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationTaskSpec   `json:"spec,omitempty"`
	Status MigrationTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MigrationTaskList contains a list of MigrationTask
type MigrationTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationTask `json:"items"`
}

type Metrics struct {
	IsDisable     bool  `json:"isDisable,omitempty"`
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
}
