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

package action

import (
	"context"
	"errors"
	"testing"
	"time"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestStatefulSetActionHelpers(t *testing.T) {
	action := &StatefulSetAction{
		Name:       "continuous",
		ObjectMeta: metav1.ObjectMeta{Name: "backup-sts", Namespace: "ns"},
	}

	assert.Equal(t, "continuous", action.GetName())
	assert.Equal(t, dpv1alpha1.ActionTypeStatefulSet, action.Type())
	assert.Equal(t, &corev1.ObjectReference{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       constant.StatefulSetKind,
		Namespace:  "ns",
		Name:       "backup-sts",
	}, action.BuildObjectRef())
	assert.Equal(t, "60s", action.getIntervalSeconds("@hourly"))
	assert.Equal(t, "300s", action.getIntervalSeconds("*/5 * * * *"))
	assert.Equal(t, "7200s", action.getIntervalSeconds("CRON_TZ=UTC 0 */2 * * *"))
}

func TestCreateVolumeSnapshotSmallHelpers(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-pvc"}}
	wrapper := NewPersistentVolumeClaimWrapper(pvc, "data")
	assert.Equal(t, "data", wrapper.VolumeName)
	assert.Equal(t, "data-pvc", wrapper.PersistentVolumeClaim.Name)

	message := "Failed to get snapshot class with error: missing default"
	assert.True(t, isVolumeSnapshotConfigError(&vsv1.VolumeSnapshot{Status: &vsv1.VolumeSnapshotStatus{Error: &vsv1.VolumeSnapshotError{Message: &message}}}))
	other := "transient api error"
	assert.False(t, isVolumeSnapshotConfigError(&vsv1.VolumeSnapshot{Status: &vsv1.VolumeSnapshotStatus{Error: &vsv1.VolumeSnapshotError{Message: &other}}}))
	assert.False(t, isVolumeSnapshotConfigError(&vsv1.VolumeSnapshot{}))
}

func TestStatusBuilderFluentFields(t *testing.T) {
	start := metav1.NewTime(time.Now())
	end := metav1.NewTime(start.Add(time.Hour))
	status := newStatusBuilder(&StatefulSetAction{Name: "continuous"}).
		phase(dpv1alpha1.ActionPhaseCompleted).
		reason("done").
		startTimestamp(&start).
		completionTimestamp(&end).
		totalSize("10Gi").
		timeRange(&start, &end).
		volumeSnapshots([]dpv1alpha1.VolumeSnapshotStatus{{Name: "snap", VolumeName: "data"}}).
		objectRef(&corev1.ObjectReference{Name: "sts"}).
		build()

	assert.Equal(t, dpv1alpha1.ActionPhaseCompleted, status.Phase)
	assert.Equal(t, "done", status.FailureReason)
	assert.Equal(t, "10Gi", status.TotalSize)
	assert.Equal(t, "snap", status.VolumeSnapshots[0].Name)
	assert.Equal(t, "sts", status.ObjectRef.Name)
	assert.Equal(t, start, *status.TimeRange.Start)

	status = newStatusBuilder(&StatefulSetAction{Name: "continuous"}).withErr(errors.New("failed")).build()
	assert.Equal(t, dpv1alpha1.ActionPhaseFailed, status.Phase)
	assert.Equal(t, "failed", status.FailureReason)
}

func TestStatefulSetActionExecuteCreatesAndUpdates(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, appsv1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "ns",
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: "schedule",
			},
		},
		Spec: dpv1alpha1.BackupSpec{BackupMethod: "continuous", RetentionPeriod: "2h"},
	}
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "schedule", Namespace: "ns"},
		Spec:       dpv1alpha1.BackupScheduleSpec{Schedules: []dpv1alpha1.SchedulePolicy{{BackupMethod: "continuous", CronExpression: "*/10 * * * *"}}},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, schedule).Build()
	action := &StatefulSetAction{
		Name:       "continuous",
		Backup:     backup,
		ObjectMeta: metav1.ObjectMeta{Name: "backup-sts", Namespace: "ns", Labels: map[string]string{"app": "backup"}},
		Replicas:   pointer.Int32(1),
		PodSpec:    &corev1.PodSpec{Containers: []corev1.Container{{Name: "manager", Image: "busybox"}}},
	}

	status, err := action.Execute(ActionContext{Ctx: context.Background(), Client: cli, Scheme: scheme})
	assert.NoError(t, err)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)

	sts := &appsv1.StatefulSet{}
	assert.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: "backup-sts", Namespace: "ns"}, sts))
	assert.Equal(t, corev1.RestartPolicyAlways, sts.Spec.Template.Spec.RestartPolicy)
	env := sts.Spec.Template.Spec.Containers[0].Env
	assert.Contains(t, env, corev1.EnvVar{Name: dptypes.DPArchiveInterval, Value: "600s"})
	assert.Contains(t, env, corev1.EnvVar{Name: dptypes.DPContinuousTTLSeconds, Value: "7200"})

	sts.Status.AvailableReplicas = 1
	assert.NoError(t, cli.Status().Update(context.Background(), sts))
	status, err = action.Execute(ActionContext{Ctx: context.Background(), Client: cli, Scheme: scheme})
	assert.NoError(t, err)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
	assert.Equal(t, int32(1), *status.AvailableReplicas)
	assert.Equal(t, dpv1alpha1.BackupPhaseRunning, backup.Status.Phase)
}
