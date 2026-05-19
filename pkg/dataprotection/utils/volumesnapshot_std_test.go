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

package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// --- SupportsVolumeSnapshotV1 ---

func TestSupportsVolumeSnapshotV1_Default(t *testing.T) {
	viper.Set("VOLUMESNAPSHOT_API_BETA", "")
	defer viper.Set("VOLUMESNAPSHOT_API_BETA", nil)
	assert.True(t, SupportsVolumeSnapshotV1())
}

func TestSupportsVolumeSnapshotV1_BetaTrue(t *testing.T) {
	viper.Set("VOLUMESNAPSHOT_API_BETA", "true")
	defer viper.Set("VOLUMESNAPSHOT_API_BETA", nil)
	assert.False(t, SupportsVolumeSnapshotV1())
}

// --- IsVolumeSnapshotEnabled ---

func TestIsVolumeSnapshotEnabled_EmptyPVName(t *testing.T) {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	cli := fake.NewClientBuilder().WithScheme(s).Build()

	ok, err := IsVolumeSnapshotEnabled(context.Background(), cli, "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIsVolumeSnapshotEnabled_NoCSI(t *testing.T) {
	// Force v1 support to avoid conversion path
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	defer func() { supportsVolumeSnapshotV1 = nil }()

	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec:       corev1.PersistentVolumeSpec{},
	}
	cli := fake.NewClientBuilder().WithScheme(s).WithObjects(pv).Build()

	ok, err := IsVolumeSnapshotEnabled(context.Background(), cli, "pv-1")
	require.NoError(t, err)
	assert.False(t, ok, "PV without CSI should not support snapshots")
}

func TestIsVolumeSnapshotEnabled_MatchingDriver(t *testing.T) {
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	defer func() { supportsVolumeSnapshotV1 = nil }()

	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)

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
		ObjectMeta: metav1.ObjectMeta{Name: "vsc-1"},
		Driver:     "ebs.csi.aws.com",
	}
	cli := fake.NewClientBuilder().WithScheme(s).WithObjects(pv, vsc).Build()

	ok, err := IsVolumeSnapshotEnabled(context.Background(), cli, "pv-1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestIsVolumeSnapshotEnabled_NoMatchingDriver(t *testing.T) {
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	defer func() { supportsVolumeSnapshotV1 = nil }()

	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)

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
		ObjectMeta: metav1.ObjectMeta{Name: "vsc-1"},
		Driver:     "disk.csi.azure.com",
	}
	cli := fake.NewClientBuilder().WithScheme(s).WithObjects(pv, vsc).Build()

	ok, err := IsVolumeSnapshotEnabled(context.Background(), cli, "pv-1")
	require.NoError(t, err)
	assert.False(t, ok)
}
