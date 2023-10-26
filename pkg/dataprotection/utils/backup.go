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

package utils

import (
	"fmt"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

// GetBackupMethodsFromBackupPolicy get backup methods from backup policy
// if backup policy is specified, search the backup policy with the name
// if backup policy is not specified, search the default backup policy
// if method's snapshotVolumes is true, use the method as the default backup method
func GetBackupMethodsFromBackupPolicy(backupPolicyList *dpv1alpha1.BackupPolicyList, backupPolicyName string) (string, map[string]struct{}, error) {
	var defaultBackupMethod string
	var backupMethodsMap = make(map[string]struct{})
	for _, policy := range backupPolicyList.Items {
		// if backupPolicyName is not empty, only use the backup policy with the name
		if backupPolicyName != "" && policy.Name != backupPolicyName {
			continue
		}
		// if backupPolicyName is empty, only use the default backup policy
		if backupPolicyName == "" && policy.Annotations[dptypes.DefaultBackupPolicyAnnotationKey] != "true" {
			continue
		}
		if policy.Status.Phase != dpv1alpha1.AvailablePhase {
			continue
		}
		for _, method := range policy.Spec.BackupMethods {
			if boolptr.IsSetToTrue(method.SnapshotVolumes) {
				defaultBackupMethod = method.Name
			}
			backupMethodsMap[method.Name] = struct{}{}
		}
	}
	if defaultBackupMethod == "" {
		return "", nil, fmt.Errorf("failed to find default backup method which snapshotVolumes is true, please check cluster's backup policy")
	}
	return defaultBackupMethod, backupMethodsMap, nil
}
