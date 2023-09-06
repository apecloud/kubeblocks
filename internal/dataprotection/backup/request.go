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

package backup

import (
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
)

// Request is a request for a backup, with all references to other objects.
type Request struct {
	*dpv1alpha1.Backup
	intctrlutil.RequestCtx

	BackupPolicy  *dpv1alpha1.BackupPolicy
	BackupMethod  *dpv1alpha1.BackupMethod
	ActionSet     *dpv1alpha1.ActionSet
	TargetPods    []*corev1.Pod
	BackupRepoPVC *corev1.PersistentVolumeClaim
	BackupRepo    *dpv1alpha1.BackupRepo
}

func (r *Request) GetBackupType() string {
	if r.ActionSet != nil {
		return string(r.ActionSet.Spec.BackupType)
	}
	if r.BackupMethod != nil && boolptr.IsSetToTrue(r.BackupMethod.SnapshotVolumes) {
		return string(dpv1alpha1.BackupTypeFull)
	}
	return ""
}
