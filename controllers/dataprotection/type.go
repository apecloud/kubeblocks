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
	"embed"
	"runtime"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

const (
	trueVal = "true"
)

const (
	// name of our custom finalizer
	dataProtectionFinalizerName = "dataprotection.kubeblocks.io/finalizer"
	// settings keys
	maxConcurDataProtectionReconKey = "MAXCONCURRENTRECONCILES_DATAPROTECTION"

	// label keys
	dataProtectionLabelBackupPolicyKey   = "dataprotection.kubeblocks.io/backup-policy"
	dataProtectionLabelBackupTypeKey     = "dataprotection.kubeblocks.io/backup-type"
	dataProtectionLabelAutoBackupKey     = "dataprotection.kubeblocks.io/autobackup"
	dataProtectionLabelRestoreJobNameKey = "restorejobs.dataprotection.kubeblocks.io/name"

	dataProtectionBackupTargetPodKey          = "dataprotection.kubeblocks.io/target-pod-name"
	dataProtectionAnnotationCreateByPolicyKey = "dataprotection.kubeblocks.io/created-by-policy"

	dataProtectionBackupRepoKey          = "dataprotection.kubeblocks.io/backup-repo-name"
	dataProtectionWaitRepoPreparationKey = "dataprotection.kubeblocks.io/wait-repo-preparation"
	dataProtectionIsToolConfigKey        = "dataprotection.kubeblocks.io/is-tool-config"

	// annotation keys
	dataProtectionSecretTemplateMD5AnnotationKey        = "dataprotection.kubeblocks.io/secret-template-md5"
	dataProtectionTemplateValuesMD5AnnotationKey        = "dataprotection.kubeblocks.io/template-values-md5"
	dataProtectionPVCTemplateMD5MD5AnnotationKey        = "dataprotection.kubeblocks.io/pvc-template-md5"
	dataProtectionToolConfigTemplateMD5MD5AnnotationKey = "dataprotection.kubeblocks.io/tool-config-template-md5"

	// the key of persistentVolumeTemplate in the configmap.
	persistentVolumeTemplateKey = "persistentVolume"

	hostNameLabelKey = "kubernetes.io/hostname"
)

// condition constants
const (
	// condition types
	ConditionTypeStorageProviderReady  = "StorageProviderReady"
	ConditionTypeParametersChecked     = "ParametersChecked"
	ConditionTypeStorageClassCreated   = "StorageClassCreated"
	ConditionTypePVCTemplateChecked    = "PVCTemplateChecked"
	ConditionTypeToolConfigChecked     = "ToolConfigSecretChecked"
	ConditionTypeDerivedObjectsDeleted = "DerivedObjectsDeleted"

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
	ReasonBadToolConfigTemplate     = "BadToolConfigTemplate"
	ReasonStorageClassCreated       = "StorageClassCreated"
	ReasonPVCTemplateChecked        = "PVCTemplateChecked"
	ReasonToolConfigChecked         = "ToolConfigChecked"
	ReasonHaveAssociatedBackups     = "HaveAssociatedBackups"
	ReasonHaveResidualPVCs          = "HaveResidualPVCs"
	ReasonDerivedObjectsDeleted     = "DerivedObjectsDeleted"
	ReasonUnknownError              = "UnknownError"
)

const manifestsUpdaterContainerName = "manifests-updater"

var reconcileInterval = time.Second

func init() {
	viper.SetDefault(maxConcurDataProtectionReconKey, runtime.NumCPU()*2)
}

var (
	//go:embed cue/*
	cueTemplates embed.FS
)

type backupPolicyOptions struct {
	Name             string          `json:"name"`
	BackupPolicyName string          `json:"backupPolicyName"`
	Namespace        string          `json:"namespace"`
	MgrNamespace     string          `json:"mgrNamespace"`
	Cluster          string          `json:"cluster"`
	Schedule         string          `json:"schedule"`
	BackupType       string          `json:"backupType"`
	TTL              metav1.Duration `json:"ttl,omitempty"`
	ServiceAccount   string          `json:"serviceAccount"`
	Image            string          `json:"image"`
	Tolerations      *corev1.PodSpec `json:"tolerations"`
}
