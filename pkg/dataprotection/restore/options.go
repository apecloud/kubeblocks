/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package restore

import (
	corev1 "k8s.io/api/core/v1"

	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// BackupDataSourceRef returns the PVC dataSourceRef form that represents the
// cluster-facing DP restore input API.
func BackupDataSourceRef(name string) *corev1.TypedObjectReference {
	apiGroup := dptypes.DataprotectionAPIGroup
	return &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     name,
	}
}

// IsBackupDataSourceRef reports whether ref points to a DP Backup.
func IsBackupDataSourceRef(ref *corev1.TypedObjectReference) bool {
	if ref == nil || ref.APIGroup == nil {
		return false
	}
	return *ref.APIGroup == dptypes.DataprotectionAPIGroup &&
		ref.Kind == dptypes.BackupKind &&
		ref.Name != ""
}
