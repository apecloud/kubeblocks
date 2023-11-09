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

package dataprotection

import (
	"runtime"
	"time"

	corev1 "k8s.io/api/core/v1"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	trueVal = "true"
)

const (
	// settings keys
	maxConcurDataProtectionReconKey = "MAXCONCURRENTRECONCILES_DATAPROTECTION"

	// label keys
	dataProtectionBackupRepoKey          = "dataprotection.kubeblocks.io/backup-repo-name"
	dataProtectionWaitRepoPreparationKey = "dataprotection.kubeblocks.io/wait-repo-preparation"
	dataProtectionIsToolConfigKey        = "dataprotection.kubeblocks.io/is-tool-config"

	// annotation keys
	dataProtectionBackupRepoDigestAnnotationKey     = "dataprotection.kubeblocks.io/backup-repo-digest"
	dataProtectionNeedUpdateToolConfigAnnotationKey = "dataprotection.kubeblocks.io/need-update-tool-config"
)

// condition constants
const (
	// condition types
	ConditionTypeStorageProviderReady  = "StorageProviderReady"
	ConditionTypeParametersChecked     = "ParametersChecked"
	ConditionTypeStorageClassCreated   = "StorageClassCreated"
	ConditionTypePVCTemplateChecked    = "PVCTemplateChecked"
	ConditionTypeDerivedObjectsDeleted = "DerivedObjectsDeleted"
	ConditionTypePreCheckPassed        = "PreCheckPassed"

	// condition reasons
	ReasonStorageProviderReady      = "StorageProviderReady"
	ReasonStorageProviderNotReady   = "StorageProviderNotReady"
	ReasonStorageProviderNotFound   = "StorageProviderNotFound"
	ReasonInvalidStorageProvider    = "InvalidStorageProvider"
	ReasonParametersChecked         = "ParametersChecked"
	ReasonCredentialSecretNotFound  = "CredentialSecretNotFound"
	ReasonPrepareCSISecretFailed    = "PrepareCSISecretFailed"
	ReasonPrepareStorageClassFailed = "PrepareStorageClassFailed"
	ReasonBadPVCTemplate            = "BadPVCTemplate"
	ReasonStorageClassCreated       = "StorageClassCreated"
	ReasonPVCTemplateChecked        = "PVCTemplateChecked"
	ReasonHaveAssociatedBackups     = "HaveAssociatedBackups"
	ReasonHaveResidualPVCs          = "HaveResidualPVCs"
	ReasonDerivedObjectsDeleted     = "DerivedObjectsDeleted"
	ReasonPreCheckPassed            = "PreCheckPassed"
	ReasonPreCheckFailed            = "PreCheckFailed"
	ReasonDigestChanged             = "DigestChanged"
	ReasonUnknownError              = "UnknownError"
	ReasonSkipped                   = "Skipped"
)

// constant  for volume populator
const (
	populatePodPrefix = "kb-populate"

	// annotation keys
	annSelectedNode = "volume.kubernetes.io/selected-node"
	annPopulateFrom = "dataprotection.kubeblocks.io/populate-from"

	// event reason
	reasonStartToVolumePopulate = "StartToVolumePopulate"
	reasonVolumePopulateSucceed = "VolumePopulateSucceed"
	reasonVolumePopulateFailed  = "VolumePopulateFailed"

	// pvc condition type and reason
	reasonPopulatingFailed     = "Failed"
	reasonPopulatingProcessing = "Processing"
	reasonPopulatingSucceed    = "Succeed"

	PersistentVolumeClaimPopulating corev1.PersistentVolumeClaimConditionType = "Populating"
)

var reconcileInterval = time.Second

func init() {
	viper.SetDefault(maxConcurDataProtectionReconKey, runtime.NumCPU()*2)
}
