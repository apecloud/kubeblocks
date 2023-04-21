/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

// MigrationTemplateSpec defines the desired state of MigrationTemplate
type MigrationTemplateSpec struct {
	TaskType       []TaskTypeEnum      `json:"taskType,omitempty"`
	Source         DBTypeSupport       `json:"source"`
	Sink           DBTypeSupport       `json:"target"`
	Initialization InitializationModel `json:"initialization,omitempty"`
	Cdc            CdcModel            `json:"cdc,omitempty"`
	// +optional
	Description string `json:"description,omitempty"`
	// +optional
	Decorator string `json:"decorator,omitempty"`
}

type DBTypeSupport struct {
	DBType    DBTypeEnum `json:"dbType"`
	DBVersion string     `json:"dbVersion"`
}

type InitializationModel struct {
	// +optional
	IsPositionPreparation bool        `json:"isPositionPreparation,omitempty"`
	Steps                 []StepModel `json:"steps,omitempty"`
}

type StepModel struct {
	Step      StepEnum               `json:"step"`
	Container BasicContainerTemplate `json:"container"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Param IntOrStringMap `json:"param"`
}

type CdcModel struct {
	Container BasicContainerTemplate `json:"container"`
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Param IntOrStringMap `json:"param"`
}

type BasicContainerTemplate struct {
	Image string `json:"image"`
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
}

// MigrationTemplateStatus defines the observed state of MigrationTemplate
type MigrationTemplateStatus struct {
	Phase Phase `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={dtplatform},scope=Cluster,shortName=mtp
// +kubebuilder:printcolumn:name="DATABASE-MAPPING",type="string",JSONPath=".spec.description",description="the database mapping that supported"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="the template status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// MigrationTemplate is the Schema for the migrationtemplates API
type MigrationTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationTemplateSpec   `json:"spec,omitempty"`
	Status MigrationTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MigrationTemplateList contains a list of MigrationTemplate
type MigrationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationTemplate `json:"items"`
}
