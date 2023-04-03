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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigrationTemplateSpec defines the desired state of MigrationTemplate
type MigrationTemplateSpec struct {
	TaskType       []TaskTypeEnum      `json:"taskType,omitempty"`
	Source         DbTypeSupport       `json:"source"`
	Sink           DbTypeSupport       `json:"target"`
	Initialization InitializationModel `json:"initialization,omitempty"`
	Cdc            CdcModel            `json:"cdc,omitempty"`
	// +optional
	Description string `json:"description,omitempty"`
	// +optional
	Decorator string `json:"decorator,omitempty"`
}

type DbTypeSupport struct {
	DbType    DbTypeEnum `json:"dbType"`
	DbVersion string     `json:"dbVersion"`
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
