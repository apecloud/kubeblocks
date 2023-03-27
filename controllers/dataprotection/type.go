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
	"runtime"
	"time"

	"github.com/spf13/viper"
)

const (
	// name of our custom finalizer
	dataProtectionFinalizerName = "dataprotection.kubeblocks.io/finalizer"
	// settings keys
	maxConcurDataProtectionReconKey = "MAXCONCURRENTRECONCILES_DATAPROTECTION"

	// label keys
	dataProtectionLabelBackupTypeKey     = "dataprotection.kubeblocks.io/backup-type"
	dataProtectionLabelAutoBackupKey     = "dataprotection.kubeblocks.io/autobackup"
	dataProtectionLabelBackupNameKey     = "backups.dataprotection.kubeblocks.io/name"
	dataProtectionLabelRestoreJobNameKey = "restorejobs.dataprotection.kubeblocks.io/name"

	dataProtectionBackupTargetPodKey = "dataprotection.kubeblocks.io/target-pod-name"
	// error status
	errorJobFailed = "JobFailed"
)

var reconcileInterval = time.Second

func init() {
	viper.SetDefault(maxConcurDataProtectionReconKey, runtime.NumCPU()*2)
}
