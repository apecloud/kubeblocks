/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package restore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestValidateAndInitRestoreMGRCrossNamespaceBackup(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	fullActionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-action"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Restore:    &dpv1alpha1.RestoreActionSpec{PrepareData: &dpv1alpha1.JobActionSpec{}},
		},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "source"},
		Status: dpv1alpha1.BackupStatus{
			Phase:        dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "full", ActionSetName: fullActionSet.Name},
		},
	}
	restoreObj := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "target"},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace},
		},
	}
	reqCtx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restoreObj)},
	}

	t.Run("allows the Backup reference by default", func(t *testing.T) {
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fullActionSet, backup).Build()
		mgr := &RestoreManager{Restore: restoreObj.DeepCopy()}
		assert.NoError(t, ValidateAndInitRestoreMGR(reqCtx, cli, mgr))
		assert.Len(t, mgr.PrepareDataBackupSets, 1)
	})

	t.Run("rejects a cross-namespace VolumeSnapshot restore", func(t *testing.T) {
		snapshotBackup := backup.DeepCopy()
		snapshotVolumes := true
		snapshotBackup.Status.BackupMethod.SnapshotVolumes = &snapshotVolumes
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fullActionSet, snapshotBackup).Build()
		err := ValidateAndInitRestoreMGR(reqCtx, cli, &RestoreManager{Restore: restoreObj.DeepCopy()})
		assert.ErrorContains(t, err, "VolumeSnapshot")
		assert.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal))
	})

	t.Run("rejects a cross-namespace VolumeSnapshot base Backup selected for PITR", func(t *testing.T) {
		const backupPolicyName = "policy"
		now := time.Now().UTC().Truncate(time.Second)
		continuousStart := metav1.NewTime(now.Add(-2 * time.Hour))
		baseBackupEnd := metav1.NewTime(now.Add(-time.Hour))
		continuousEnd := metav1.NewTime(now)
		snapshotVolumes := true

		continuousActionSet := &dpv1alpha1.ActionSet{
			ObjectMeta: metav1.ObjectMeta{Name: "continuous-action"},
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeContinuous,
				Restore:    &dpv1alpha1.RestoreActionSpec{},
			},
		}
		baseBackup := &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "base-snapshot",
				Namespace: backup.Namespace,
				Labels: map[string]string{
					dptypes.BackupTypeLabelKey:   string(dpv1alpha1.BackupTypeFull),
					dptypes.BackupPolicyLabelKey: backupPolicyName,
				},
			},
			Spec: dpv1alpha1.BackupSpec{BackupPolicyName: backupPolicyName},
			Status: dpv1alpha1.BackupStatus{
				Phase: dpv1alpha1.BackupPhaseCompleted,
				TimeRange: &dpv1alpha1.BackupTimeRange{
					Start: &continuousStart,
					End:   &baseBackupEnd,
				},
				BackupMethod: &dpv1alpha1.BackupMethod{
					Name:            "snapshot",
					ActionSetName:   fullActionSet.Name,
					SnapshotVolumes: &snapshotVolumes,
				},
			},
		}
		continuousBackup := &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "continuous", Namespace: backup.Namespace},
			Spec:       dpv1alpha1.BackupSpec{BackupPolicyName: backupPolicyName},
			Status: dpv1alpha1.BackupStatus{
				Phase: dpv1alpha1.BackupPhaseCompleted,
				TimeRange: &dpv1alpha1.BackupTimeRange{
					Start: &continuousStart,
					End:   &continuousEnd,
				},
				BackupMethod: &dpv1alpha1.BackupMethod{
					Name:          "continuous",
					ActionSetName: continuousActionSet.Name,
				},
			},
		}
		pitrRestore := restoreObj.DeepCopy()
		pitrRestore.Spec.Backup = dpv1alpha1.BackupRef{Name: continuousBackup.Name, Namespace: continuousBackup.Namespace}
		pitrRestore.Spec.RestoreTime = now.Add(-30 * time.Minute).Format(time.RFC3339)
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fullActionSet, continuousActionSet, baseBackup, continuousBackup).Build()

		err := ValidateAndInitRestoreMGR(reqCtx, cli, &RestoreManager{Restore: pitrRestore})
		assert.ErrorContains(t, err, "VolumeSnapshot")
		assert.ErrorContains(t, err, "source/base-snapshot")
		assert.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal))
	})
}
