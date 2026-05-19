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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func stsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	return s
}

func int32Ptr(i int32) *int32 { return &i }

func newTestBackup(labels map[string]string) *dpv1alpha1.Backup {
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup",
			Namespace: "default",
			UID:       "backup-uid",
			Labels:    labels,
		},
		Spec: dpv1alpha1.BackupSpec{
			BackupMethod: "test-method",
		},
		Status: dpv1alpha1.BackupStatus{},
	}
}

func newTestStsAction(backup *dpv1alpha1.Backup) *StatefulSetAction {
	return &StatefulSetAction{
		Name:   "sts-action",
		Backup: backup,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
			Labels:    map[string]string{"app": "backup"},
		},
		Replicas: int32Ptr(1),
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "worker", Image: "busybox", Command: []string{"sh"}},
			},
		},
	}
}

func TestStatefulSetAction_GetName(t *testing.T) {
	s := &StatefulSetAction{Name: "my-sts"}
	assert.Equal(t, "my-sts", s.GetName())
}

func TestStatefulSetAction_Type(t *testing.T) {
	s := &StatefulSetAction{}
	assert.Equal(t, dpv1alpha1.ActionTypeStatefulSet, s.Type())
}

func TestGetIntervalSeconds(t *testing.T) {
	tests := []struct {
		name string
		cron string
		want string
	}{
		{"every 5 min", "*/5 * * * *", "300s"},
		{"every 10 min", "*/10 * * * *", "600s"},
		{"every 2 hours", "0 */2 * * *", "7200s"},
		{"fixed minute no star-slash", "0 0 * * *", "60s"},
		{"macro syntax", "@daily", "60s"},
		{"timezone prefix TZ", "TZ=UTC */15 * * * *", "900s"},
		{"timezone prefix CRON_TZ", "CRON_TZ=Asia/Shanghai */30 * * * *", "1800s"},
		{"fixed minute 30", "30 * * * *", "60s"},
		{"every 1 min", "*/1 * * * *", "60s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &StatefulSetAction{}
			got := s.getIntervalSeconds(tt.cron)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatefulSetAction_Execute_CreatesSTS(t *testing.T) {
	scheme := stsTestScheme()
	backup := newTestBackup(nil)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	a := newTestStsAction(backup)
	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := a.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
	assert.Equal(t, "sts-action", status.Name)
	assert.Equal(t, dpv1alpha1.ActionTypeStatefulSet, status.ActionType)

	// verify STS was created
	sts := &appsv1.StatefulSet{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test-sts"}, sts)
	require.NoError(t, err)
	assert.Contains(t, sts.Finalizers, dptypes.DataProtectionFinalizerName)
	assert.Equal(t, int32(1), *sts.Spec.Replicas)
	assert.Equal(t, corev1.RestartPolicyAlways, sts.Spec.Template.Spec.RestartPolicy)
}

func TestStatefulSetAction_Execute_UpdatesSTS(t *testing.T) {
	scheme := stsTestScheme()
	backup := newTestBackup(nil)

	existingSTS := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
			Labels:    map[string]string{"app": "backup"},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "backup"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "backup"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "old-worker", Image: "old-image"},
					},
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSTS).Build()

	a := newTestStsAction(backup)
	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := a.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)

	// verify STS was updated
	updated := &appsv1.StatefulSet{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test-sts"}, updated)
	require.NoError(t, err)
	require.Len(t, updated.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, "worker", updated.Spec.Template.Spec.Containers[0].Name)
}

func TestStatefulSetAction_StsIsFailed_PodNotFound(t *testing.T) {
	scheme := stsTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	a := &StatefulSetAction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
		},
	}
	actCtx := ActionContext{
		Ctx:    context.Background(),
		Client: cli,
	}

	assert.False(t, a.stsIsFailed(actCtx))
}

func TestStatefulSetAction_StsIsFailed_PodRunning(t *testing.T) {
	scheme := stsTestScheme()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts-0",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()

	a := &StatefulSetAction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
		},
	}
	actCtx := ActionContext{
		Ctx:    context.Background(),
		Client: cli,
	}

	assert.False(t, a.stsIsFailed(actCtx))
}

func TestStatefulSetAction_CreateStatefulSet(t *testing.T) {
	scheme := stsTestScheme()
	backup := newTestBackup(nil)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	a := newTestStsAction(backup)
	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1", Image: "img"}},
	}
	err := a.createStatefulSet(actCtx, podSpec)
	require.NoError(t, err)

	sts := &appsv1.StatefulSet{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test-sts"}, sts)
	require.NoError(t, err)
	assert.Contains(t, sts.Finalizers, dptypes.DataProtectionFinalizerName)
	assert.Equal(t, map[string]string{"app": "backup"}, sts.Spec.Selector.MatchLabels)
	assert.Equal(t, map[string]string{"app": "backup"}, sts.Spec.Template.Labels)
	require.Len(t, sts.OwnerReferences, 1)
}

func TestStatefulSetAction_InjectContinuousEnv_NoSchedule(t *testing.T) {
	scheme := stsTestScheme()
	backup := newTestBackup(map[string]string{
		dptypes.BackupScheduleLabelKey: "nonexistent-schedule",
	})
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	a := &StatefulSetAction{
		Backup: backup,
	}
	actCtx := ActionContext{
		Ctx:    context.Background(),
		Client: cli,
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1", Image: "img"}},
	}

	err := a.injectContinuousEnvForPodSpec(actCtx, podSpec)
	assert.NoError(t, err)
	// no env vars added since schedule not found
	assert.Empty(t, podSpec.Containers[0].Env)
}

func TestStatefulSetAction_InjectContinuousEnv_WithSchedule(t *testing.T) {
	scheme := stsTestScheme()
	backup := newTestBackup(map[string]string{
		dptypes.BackupScheduleLabelKey: "test-schedule",
	})
	backup.Spec.RetentionPeriod = dpv1alpha1.RetentionPeriod("1h")

	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "default",
		},
		Spec: dpv1alpha1.BackupScheduleSpec{
			Schedules: []dpv1alpha1.SchedulePolicy{
				{
					BackupMethod:   "test-method",
					CronExpression: "*/5 * * * *",
					Enabled:        boolPtr(true),
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(schedule).Build()

	a := &StatefulSetAction{
		Backup: backup,
	}
	actCtx := ActionContext{
		Ctx:    context.Background(),
		Client: cli,
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1", Image: "img"}},
	}

	err := a.injectContinuousEnvForPodSpec(actCtx, podSpec)
	require.NoError(t, err)

	envMap := make(map[string]string)
	for _, env := range podSpec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}
	assert.Equal(t, "300s", envMap[dptypes.DPArchiveInterval])
	assert.Equal(t, "3600", envMap[dptypes.DPContinuousTTLSeconds])
}

func boolPtr(b bool) *bool { return &b }
