/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// Phase defines the BackupPolicy and ActionSet CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	AvailablePhase   Phase = "Available"
	UnavailablePhase Phase = "Unavailable"
)

func (p Phase) IsAvailable() bool {
	return p == AvailablePhase
}

// BackupRepoPhase denotes different stages for the `BackupRepo`.
//
// +enum
// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
type BackupRepoPhase string

const (
	// BackupRepoPreChecking indicates the backup repository is being pre-checked.
	BackupRepoPreChecking BackupRepoPhase = "PreChecking"
	// BackupRepoFailed indicates the pre-check has been failed.
	BackupRepoFailed BackupRepoPhase = "Failed"
	// BackupRepoReady indicates the backup repository is ready for use.
	BackupRepoReady BackupRepoPhase = "Ready"
	// BackupRepoDeleting indicates the backup repository is being deleted.
	BackupRepoDeleting BackupRepoPhase = "Deleting"
)

// RetentionPeriod represents a duration in the format "1y2mo3w4d5h6m", where
// y=year, mo=month, w=week, d=day, h=hour, m=minute.
type RetentionPeriod = appsv1.RetentionPeriod

// RestorePhase The current phase. Valid values are Running, Completed, Failed, AsDataSource.
// +enum
// +kubebuilder:validation:Enum={Running,Completed,Failed,AsDataSource}
type RestorePhase string

const (
	RestorePhaseRunning      RestorePhase = "Running"
	RestorePhaseCompleted    RestorePhase = "Completed"
	RestorePhaseFailed       RestorePhase = "Failed"
	RestorePhaseAsDataSource RestorePhase = "AsDataSource"
)

// RestoreActionStatus the status of restore action.
// +enum
// +kubebuilder:validation:Enum={Processing,Completed,Failed}
type RestoreActionStatus string

const (
	RestoreActionProcessing RestoreActionStatus = "Processing"
	RestoreActionCompleted  RestoreActionStatus = "Completed"
	RestoreActionFailed     RestoreActionStatus = "Failed"
)

type RestoreStage string

const (
	PrepareData RestoreStage = "prepareData"
	PostReady   RestoreStage = "postReady"
)

// VolumeClaimRestorePolicy defines restore policy for persistent volume claim.
// Supported policies are as follows:
//
// 1. Parallel: parallel recovery of persistent volume claim.
// 2. Serial: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.
//
// +enum
// +kubebuilder:validation:Enum={Parallel,Serial}
type VolumeClaimRestorePolicy string

const (
	VolumeClaimRestorePolicyParallel VolumeClaimRestorePolicy = "Parallel"
	VolumeClaimRestorePolicySerial   VolumeClaimRestorePolicy = "Serial"
)

type DataRestorePolicy string

const (
	OneToOneRestorePolicy  DataRestorePolicy = "OneToOne"
	OneToManyRestorePolicy DataRestorePolicy = "OneToMany"
)

const (
	DefaultEncryptionAlgorithm = "AES-256-CFB"
)

// EncryptionConfig defines the parameters for encrypting backup data.
type EncryptionConfig struct {
	// Specifies the encryption algorithm. Currently supported algorithms are:
	//
	// - AES-128-CFB
	// - AES-192-CFB
	// - AES-256-CFB
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=AES-256-CFB
	// +kubebuilder:validation:Enum={AES-128-CFB,AES-192-CFB,AES-256-CFB}
	Algorithm string `json:"algorithm"`

	// Selects the key of a secret in the current namespace, the value of the secret
	// is used as the encryption key.
	//
	// +kubebuilder:validation:Required
	PassPhraseSecretKeyRef *corev1.SecretKeySelector `json:"passPhraseSecretKeyRef"`
}

type ActionSetParametersSchema struct {
	// Defines the schema for parameters using the OpenAPI v3.
	// The supported property types include:
	// - string
	// - number
	// - integer
	// - array: Note that only items of string type are supported.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +k8s:conversion-gen=false
	// +optional
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

type ParameterPair struct {
	// Represents the name of the parameter.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Represents the parameter values.
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}
