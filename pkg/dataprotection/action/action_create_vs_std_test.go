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

package action

import (
	"context"
	"testing"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

func vsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	return s
}

func TestNewPersistentVolumeClaimWrapper(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-1", Namespace: "default"},
	}
	w := NewPersistentVolumeClaimWrapper(pvc, "data-vol")
	assert.Equal(t, "data-vol", w.VolumeName)
	assert.Equal(t, "pvc-1", w.PersistentVolumeClaim.Name)
}

func TestCreateVolumeSnapshotAction_GetName(t *testing.T) {
	a := &CreateVolumeSnapshotAction{Name: "vs-action"}
	assert.Equal(t, "vs-action", a.GetName())
}

func TestCreateVolumeSnapshotAction_Type(t *testing.T) {
	a := &CreateVolumeSnapshotAction{}
	assert.Equal(t, dpv1alpha1.ActionTypeNone, a.Type())
}

func TestCreateVolumeSnapshotAction_Validate_Empty(t *testing.T) {
	a := &CreateVolumeSnapshotAction{}
	err := a.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "persistent volume claims are required")
}

func TestCreateVolumeSnapshotAction_Validate_TooMany(t *testing.T) {
	a := &CreateVolumeSnapshotAction{
		PersistentVolumeClaimWrappers: []PersistentVolumeClaimWrapper{
			{VolumeName: "v1"},
			{VolumeName: "v2"},
		},
	}
	err := a.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one persistent volume claim is supported")
}

func TestCreateVolumeSnapshotAction_Validate_Single(t *testing.T) {
	a := &CreateVolumeSnapshotAction{
		PersistentVolumeClaimWrappers: []PersistentVolumeClaimWrapper{
			{VolumeName: "v1"},
		},
	}
	err := a.validate()
	assert.NoError(t, err)
}

func TestIsVolumeSnapshotConfigError_NilStatus(t *testing.T) {
	snap := &vsv1.VolumeSnapshot{}
	assert.False(t, isVolumeSnapshotConfigError(snap))
}

func TestIsVolumeSnapshotConfigError_NoError(t *testing.T) {
	snap := &vsv1.VolumeSnapshot{
		Status: &vsv1.VolumeSnapshotStatus{},
	}
	assert.False(t, isVolumeSnapshotConfigError(snap))
}

func TestIsVolumeSnapshotConfigError_UnrelatedError(t *testing.T) {
	msg := "some random error"
	snap := &vsv1.VolumeSnapshot{
		Status: &vsv1.VolumeSnapshotStatus{
			Error: &vsv1.VolumeSnapshotError{Message: &msg},
		},
	}
	assert.False(t, isVolumeSnapshotConfigError(snap))
}

func TestIsVolumeSnapshotConfigError_MatchingErrors(t *testing.T) {
	tests := []struct {
		name string
		msg  string
	}{
		{"default snapshot class error", "Failed to set default snapshot class with error: something"},
		{"get snapshot class error", "Failed to get snapshot class with error: missing"},
		{"CSI PV source error", "Failed to create snapshot content with error cannot find CSI PersistentVolumeSource for volume xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.msg
			snap := &vsv1.VolumeSnapshot{
				Status: &vsv1.VolumeSnapshotStatus{
					Error: &vsv1.VolumeSnapshotError{Message: &msg},
				},
			}
			assert.True(t, isVolumeSnapshotConfigError(snap))
		})
	}
}

func TestEnsureVolumeSnapshotReady_NotFound(t *testing.T) {
	scheme := vsTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	key := client.ObjectKey{Namespace: "default", Name: "missing-snap"}

	ok, snap, err := ensureVolumeSnapshotReady(context.Background(), cli, key)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotNil(t, snap)
}

func TestEnsureVolumeSnapshotReady_NotReady(t *testing.T) {
	scheme := vsTestScheme()
	existing := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "snap-1", Namespace: "default"},
		Status:     &vsv1.VolumeSnapshotStatus{},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	key := client.ObjectKey{Namespace: "default", Name: "snap-1"}

	ok, snap, err := ensureVolumeSnapshotReady(context.Background(), cli, key)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotNil(t, snap)
}

func TestEnsureVolumeSnapshotReady_Ready(t *testing.T) {
	scheme := vsTestScheme()
	trueVal := true
	existing := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "snap-1", Namespace: "default"},
		Status: &vsv1.VolumeSnapshotStatus{
			ReadyToUse: &trueVal,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	key := client.ObjectKey{Namespace: "default", Name: "snap-1"}

	ok, snap, err := ensureVolumeSnapshotReady(context.Background(), cli, key)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, snap)
}

func TestEnsureVolumeSnapshotReady_ConfigError(t *testing.T) {
	scheme := vsTestScheme()
	msg := "Failed to set default snapshot class with error: no class"
	existing := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "snap-1", Namespace: "default"},
		Status: &vsv1.VolumeSnapshotStatus{
			Error: &vsv1.VolumeSnapshotError{Message: &msg},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	key := client.ObjectKey{Namespace: "default", Name: "snap-1"}

	ok, _, err := ensureVolumeSnapshotReady(context.Background(), cli, key)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "Failed to set default snapshot class")
}

func TestGetVolumeSnapshotClassName_NoCSI(t *testing.T) {
	scheme := vsTestScheme()
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec:       corev1.PersistentVolumeSpec{
			// no CSI
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv).Build()

	a := &CreateVolumeSnapshotAction{}
	name, err := a.getVolumeSnapshotClassName(context.Background(), cli, "pv-1")
	require.NoError(t, err)
	assert.Empty(t, name)
}

func TestGetVolumeSnapshotClassName_MatchingDriver(t *testing.T) {
	scheme := vsTestScheme()
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "ebs.csi.aws.com",
				},
			},
		},
	}
	vsc := &vsv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{Name: "ebs-snap-class"},
		Driver:     "ebs.csi.aws.com",
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv, vsc).Build()

	a := &CreateVolumeSnapshotAction{}
	name, err := a.getVolumeSnapshotClassName(context.Background(), cli, "pv-1")
	require.NoError(t, err)
	assert.Equal(t, "ebs-snap-class", name)
}

func TestGetVolumeSnapshotClassName_NoMatchingDriver(t *testing.T) {
	scheme := vsTestScheme()
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "ebs.csi.aws.com",
				},
			},
		},
	}
	vsc := &vsv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{Name: "gce-snap-class"},
		Driver:     "pd.csi.storage.gke.io",
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv, vsc).Build()

	a := &CreateVolumeSnapshotAction{}
	name, err := a.getVolumeSnapshotClassName(context.Background(), cli, "pv-1")
	require.NoError(t, err)
	assert.Empty(t, name)
}

func TestCreateVolumeSnapshotAction_Execute_ValidationFails(t *testing.T) {
	a := &CreateVolumeSnapshotAction{}
	status, err := a.Execute(ActionContext{})
	require.Error(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseFailed, status.Phase)
}

func TestCreateVolumeSnapshotAction_Execute_CreatesSnapshot(t *testing.T) {
	scheme := vsTestScheme()
	owner := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-1",
			Namespace: "default",
			UID:       "uid-1",
		},
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "pv-data",
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-data"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: "test-driver"},
			},
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv).Build()

	a := &CreateVolumeSnapshotAction{
		Name:  "snap-action",
		Owner: owner,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snap-action",
			Namespace: "default",
		},
		PersistentVolumeClaimWrappers: []PersistentVolumeClaimWrapper{
			{PersistentVolumeClaim: pvc, VolumeName: "data"},
		},
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := a.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	// snapshot created but not yet ready
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
}

func TestCreateVolumeSnapshotAction_Execute_SnapshotAlreadyReady(t *testing.T) {
	scheme := vsTestScheme()
	owner := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-1",
			Namespace: "default",
			UID:       "uid-1",
		},
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "pv-data",
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-data"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: "test-driver"},
			},
		},
	}
	trueVal := true
	now := metav1.Now()
	// pre-create the VS in ready state
	existingSnap := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snap-action-0-data",
			Namespace: "default",
		},
		Status: &vsv1.VolumeSnapshotStatus{
			ReadyToUse:   &trueVal,
			CreationTime: &now,
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv, existingSnap).Build()

	a := &CreateVolumeSnapshotAction{
		Name:  "snap-action",
		Owner: owner,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snap-action",
			Namespace: "default",
		},
		PersistentVolumeClaimWrappers: []PersistentVolumeClaimWrapper{
			{PersistentVolumeClaim: pvc, VolumeName: "data"},
		},
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := a.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseCompleted, status.Phase)
	require.Len(t, status.VolumeSnapshots, 1)
	assert.Equal(t, "data", status.VolumeSnapshots[0].VolumeName)
}

func TestCreateVolumeSnapshotAction_CreateVolumeSnapshotIfNotExist_AlreadyExists(t *testing.T) {
	scheme := vsTestScheme()
	existingSnap := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-snap",
			Namespace: "default",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSnap).Build()

	owner := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "b1", Namespace: "default", UID: "uid"},
	}
	a := &CreateVolumeSnapshotAction{
		Owner:      owner,
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-1", Namespace: "default"},
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}
	key := client.ObjectKey{Namespace: "default", Name: "existing-snap"}
	err := a.createVolumeSnapshotIfNotExist(actCtx, pvc, key)
	assert.NoError(t, err)
}
