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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func compatScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)
	return s
}

func compatClient(objs ...client.Object) *CompatClient {
	cli := fake.NewClientBuilder().WithScheme(compatScheme()).WithObjects(objs...).Build()
	return NewCompatClient(cli)
}

// --- NewCompatClient ---

func TestNewCompatClient(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(compatScheme()).Build()
	cc := NewCompatClient(cli)
	require.NotNil(t, cc)
	assert.Equal(t, cli, cc.Client)
}

// --- typeofV1Beta1 ---

func TestTypeofV1Beta1_VolumeSnapshot(t *testing.T) {
	got := typeofV1Beta1(&vsv1.VolumeSnapshot{})
	assert.NotNil(t, got)
}

func TestTypeofV1Beta1_VolumeSnapshotClass(t *testing.T) {
	got := typeofV1Beta1(&vsv1.VolumeSnapshotClass{})
	assert.NotNil(t, got)
}

func TestTypeofV1Beta1_CronJob(t *testing.T) {
	got := typeofV1Beta1(&batchv1.CronJob{})
	assert.NotNil(t, got)
}

func TestTypeofV1Beta1_VolumeSnapshotList(t *testing.T) {
	got := typeofV1Beta1(&vsv1.VolumeSnapshotList{})
	assert.NotNil(t, got)
}

func TestTypeofV1Beta1_VolumeSnapshotClassList(t *testing.T) {
	got := typeofV1Beta1(&vsv1.VolumeSnapshotClassList{})
	assert.NotNil(t, got)
}

func TestTypeofV1Beta1_CronJobList(t *testing.T) {
	got := typeofV1Beta1(&batchv1.CronJobList{})
	assert.NotNil(t, got)
}

func TestTypeofV1Beta1_Unknown(t *testing.T) {
	got := typeofV1Beta1(&corev1.Pod{})
	assert.Nil(t, got)
}

// --- convertObjectBetweenAPIVersion ---

func TestConvertObjectBetweenAPIVersion(t *testing.T) {
	src := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
	}
	dst := &corev1.Pod{}
	err := convertObjectBetweenAPIVersion(src, dst)
	require.NoError(t, err)
	assert.Equal(t, "test-pod", dst.Name)
	assert.Equal(t, "default", dst.Namespace)
}

// --- shouldConvert ---

func TestShouldConvert_Pod(t *testing.T) {
	// Reset package vars to force re-detection
	supportsVolumeSnapshotV1 = nil
	supportsCronJobV1 = nil
	defer func() {
		supportsVolumeSnapshotV1 = nil
		supportsCronJobV1 = nil
	}()

	result := shouldConvert(&corev1.Pod{})
	assert.False(t, result, "regular Pod should not need conversion")
}

// --- CompatClient CRUD with v1 support (no conversion needed) ---

func TestCompatClient_CreateGet_NormalObject(t *testing.T) {
	// Force v1 support
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	supportsCronJobV1 = &bTrue
	defer func() {
		supportsVolumeSnapshotV1 = nil
		supportsCronJobV1 = nil
	}()

	cc := compatClient()
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
	}
	require.NoError(t, cc.Create(ctx, pod))

	got := &corev1.Pod{}
	require.NoError(t, cc.Get(ctx, client.ObjectKeyFromObject(pod), got))
	assert.Equal(t, "test-pod", got.Name)
}

func TestCompatClient_List_NormalObject(t *testing.T) {
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	supportsCronJobV1 = &bTrue
	defer func() {
		supportsVolumeSnapshotV1 = nil
		supportsCronJobV1 = nil
	}()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
	}
	cc := compatClient(pod)
	ctx := context.Background()

	podList := &corev1.PodList{}
	require.NoError(t, cc.List(ctx, podList, client.InNamespace("default")))
	assert.Len(t, podList.Items, 1)
}

func TestCompatClient_Patch_NormalObject(t *testing.T) {
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	supportsCronJobV1 = &bTrue
	defer func() {
		supportsVolumeSnapshotV1 = nil
		supportsCronJobV1 = nil
	}()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
	}
	cc := compatClient(pod)
	ctx := context.Background()

	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels = map[string]string{"patched": "true"}
	require.NoError(t, cc.Patch(ctx, pod, patch))

	got := &corev1.Pod{}
	require.NoError(t, cc.Get(ctx, client.ObjectKeyFromObject(pod), got))
	assert.Equal(t, "true", got.Labels["patched"])
}

func TestCompatClient_Delete_NormalObject(t *testing.T) {
	bTrue := true
	supportsVolumeSnapshotV1 = &bTrue
	supportsCronJobV1 = &bTrue
	defer func() {
		supportsVolumeSnapshotV1 = nil
		supportsCronJobV1 = nil
	}()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
	}
	cc := compatClient(pod)
	ctx := context.Background()

	require.NoError(t, cc.Delete(ctx, pod))
}
