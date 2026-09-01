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
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestValidateAndInitRestoreMGRCrossNamespaceBackup(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	actionSet := &dpv1alpha1.ActionSet{
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
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "full", ActionSetName: actionSet.Name},
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
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet, backup).Build()
		mgr := &RestoreManager{Restore: restoreObj.DeepCopy()}
		assert.NoError(t, ValidateAndInitRestoreMGR(reqCtx, cli, mgr))
		assert.Len(t, mgr.PrepareDataBackupSets, 1)
	})

	t.Run("rejects a cross-namespace VolumeSnapshot restore", func(t *testing.T) {
		snapshotBackup := backup.DeepCopy()
		snapshotBackup.Status.BackupMethod.SnapshotVolumes = new(bool)
		*snapshotBackup.Status.BackupMethod.SnapshotVolumes = true
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet, snapshotBackup).Build()
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
					ActionSetName:   actionSet.Name,
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
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet, continuousActionSet, baseBackup, continuousBackup).Build()

		err := ValidateAndInitRestoreMGR(reqCtx, cli, &RestoreManager{Restore: pitrRestore})
		assert.ErrorContains(t, err, "VolumeSnapshot")
		assert.ErrorContains(t, err, "source/base-snapshot")
		assert.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal))
	})
}

func TestGetSourcePodNameForTargetPod(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					constant.AppInstanceLabelKey:    "source",
					constant.KBAppComponentLabelKey: "redis",
				}},
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
	}
	policy := &dpv1alpha1.RequiredPolicyForAllPodSelection{DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy}

	t.Run("matches instance template identity without list order", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-az-a-4", "source-redis-az-a-3"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-az-a-3", "az-a")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-az-a-3", podName)
	})

	t.Run("disambiguates templates sharing an ordinal", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-az-b-3", "source-redis-az-a-3"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-az-a-3", "az-a")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-az-a-3", podName)
	})

	t.Run("matches the exact template rather than a suffix", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-b-a-3", "source-redis-a-3"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-a-3", "a")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-a-3", podName)
	})

	t.Run("does not treat a flat workload suffix as a template", func(t *testing.T) {
		target.PodSelector.MatchLabels[constant.KBAppComponentLabelKey] = "redis-a"
		target.SelectedTargetPods = []string{"source-redis-a-3"}
		_, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-a-3", "a")
		assert.ErrorContains(t, err, "no selected source target pod matches")
		target.PodSelector.MatchLabels[constant.KBAppComponentLabelKey] = "redis"
	})

	t.Run("matches a flat ordinal with holes", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-7", "source-redis-2"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-7", "")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-7", podName)
	})

	t.Run("rejects an ambiguous generic ordinal", func(t *testing.T) {
		labelSelector := target.PodSelector.LabelSelector
		target.PodSelector.LabelSelector = nil
		defer func() { target.PodSelector.LabelSelector = labelSelector }()
		target.SelectedTargetPods = []string{"source-redis-az-a-3", "source-redis-az-b-3"}
		_, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-3", "")
		assert.ErrorContains(t, err, "multiple selected source target pods")
	})

	t.Run("rejects a missing identity", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-az-a-4"}
		_, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-az-a-3", "az-a")
		assert.ErrorContains(t, err, "no selected source target pod matches")
	})
}
