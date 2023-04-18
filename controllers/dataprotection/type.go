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

package dataprotection

import (
	"embed"
	"runtime"
	"time"

	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	dataProtectionLabelBackupNameKey     = "backups.dataprotection.kubeblocks.io/name"
	dataProtectionLabelRestoreJobNameKey = "restorejobs.dataprotection.kubeblocks.io/name"

	dataProtectionBackupTargetPodKey          = "dataprotection.kubeblocks.io/target-pod-name"
	dataProtectionAnnotationCreateByPolicyKey = "dataprotection.kubeblocks.io/created-by-policy"
	// error status
	errorJobFailed = "JobFailed"
)

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
}
