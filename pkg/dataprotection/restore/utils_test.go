/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestBuildRestoreLabelsKeepsRestoreNameForControllerLookup(t *testing.T) {
	restoreName := "restore-name-that-controller-uses-for-label-filtering"

	labels := BuildRestoreLabels(restoreName)
	if labels[DataProtectionRestoreLabelKey] != restoreName {
		t.Fatalf("restore label = %q, want %q", labels[DataProtectionRestoreLabelKey], restoreName)
	}
	if labels[constant.AppManagedByLabelKey] != dptypes.AppName {
		t.Fatalf("managed-by label = %q, want %q", labels[constant.AppManagedByLabelKey], dptypes.AppName)
	}
}

func TestBuildPVCVolumeAndMountShortensDerivedVolumeName(t *testing.T) {
	builder := &restoreJobBuilder{
		restore: &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore"}},
		backupSet: BackupActionSet{
			Backup: &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup"}},
		},
	}
	claim := dpv1alpha1.VolumeConfig{
		MountPath: "/data",
	}
	claimName := "very-long-claim-name-with-extra-suffix-for-restore-prepare-data"
	identifier := "restore-prepare-data-job-with-an-overlength-derived-volume-identifier"

	volume, volumeMount, err := builder.buildPVCVolumeAndMount(claim, claimName, identifier)
	if err != nil {
		t.Fatalf("buildPVCVolumeAndMount returned error: %v", err)
	}
	if volume == nil || volumeMount == nil {
		t.Fatalf("expected volume and volumeMount to be created")
	}
	if len(volume.Name) > constant.KubeNameMaxLength {
		t.Fatalf("volume name length = %d, want <= %d", len(volume.Name), constant.KubeNameMaxLength)
	}
	if volume.Name != volumeMount.Name {
		t.Fatalf("volume name %q and volumeMount name %q should match", volume.Name, volumeMount.Name)
	}
	if volumeMount.MountPath != claim.MountPath {
		t.Fatalf("volumeMount path = %q, want %q", volumeMount.MountPath, claim.MountPath)
	}
	wantSource := corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}}
	if volume.VolumeSource.PersistentVolumeClaim == nil || volume.VolumeSource.PersistentVolumeClaim.ClaimName != wantSource.PersistentVolumeClaim.ClaimName {
		t.Fatalf("volume claim name = %q, want %q", volume.VolumeSource.PersistentVolumeClaim.ClaimName, claimName)
	}
}
