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

	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

const (
	trueVal = "true"
)

const (
	// settings keys
	maxConcurDataProtectionReconKey = "MAXCONCURRENTRECONCILES_DATAPROTECTION"

	// label keys
	dataProtectionLabelBackupScheduleKey = "dataprotection.kubeblocks.io/backup-schedule"
	dataProtectionLabelBackupPolicyKey   = "dataprotection.kubeblocks.io/backup-policy"
	dataProtectionLabelBackupMethodKey   = "dataprotection.kubeblocks.io/backup-method"
	dataProtectionLabelBackupTypeKey     = "dataprotection.kubeblocks.io/backup-type"
	dataProtectionLabelAutoBackupKey     = "dataprotection.kubeblocks.io/autobackup"

	dataProtectionBackupTargetPodKey          = "dataprotection.kubeblocks.io/target-pod-name"
	dataProtectionAnnotationCreateByPolicyKey = "dataprotection.kubeblocks.io/created-by-policy"

	dataProtectionBackupRepoKey  = "dataprotection.kubeblocks.io/backup-repo-name"
	dataProtectionNeedRepoPVCKey = "dataprotection.kubeblocks.io/need-repo-pvc"

	// annotation keys
	dataProtectionSecretTemplateMD5AnnotationKey = "dataprotection.kubeblocks.io/secret-template-md5"
	dataProtectionTemplateValuesMD5AnnotationKey = "dataprotection.kubeblocks.io/template-values-md5"
	dataProtectionPVCTemplateMD5MD5AnnotationKey = "dataprotection.kubeblocks.io/pvc-template-md5"
)

// condition constants
const (
	// condition types
	ConditionTypeStorageProviderReady  = "StorageProviderReady"
	ConditionTypeParametersChecked     = "ParametersChecked"
	ConditionTypeStorageClassCreated   = "StorageClassCreated"
	ConditionTypePVCTemplateChecked    = "PVCTemplateChecked"
	ConditionTypeDerivedObjectsDeleted = "DerivedObjectsDeleted"

	// condition reasons
	ReasonStorageProviderReady     = "StorageProviderReady"
	ReasonStorageProviderNotReady  = "StorageProviderNotReady"
	ReasonStorageProviderNotFound  = "StorageProviderNotFound"
	ReasonInvalidStorageProvider   = "InvalidStorageProvider"
	ReasonParametersChecked        = "ParametersChecked"
	ReasonCredentialSecretNotFound = "CredentialSecretNotFound"
	ReasonBadSecretTemplate        = "BadSecretTemplate"
	ReasonBadStorageClassTemplate  = "BadStorageClassTemplate"
	ReasonBadPVCTemplate           = "BadPVCTemplate"
	ReasonStorageClassCreated      = "StorageClassCreated"
	ReasonPVCTemplateChecked       = "PVCTemplateChecked"
	ReasonHaveAssociatedBackups    = "HaveAssociatedBackups"
	ReasonHaveResidualPVCs         = "HaveResidualPVCs"
	ReasonDerivedObjectsDeleted    = "DerivedObjectsDeleted"
	ReasonUnknownError             = "UnknownError"
)

const manifestsUpdaterContainerName = "manifests-updater"

var reconcileInterval = time.Second

func init() {
	viper.SetDefault(maxConcurDataProtectionReconKey, runtime.NumCPU()*2)
}
