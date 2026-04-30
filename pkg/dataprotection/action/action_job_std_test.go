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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func jobTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = batchv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	return s
}

func newTestOwner(ns string) *dpv1alpha1.Backup {
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup",
			Namespace: ns,
			UID:       "backup-uid-123",
		},
	}
}

func newTestPodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{Name: "worker", Image: "busybox", Command: []string{"sh"}},
		},
	}
}

func TestJobAction_GetName(t *testing.T) {
	j := &JobAction{Name: "backup-job"}
	assert.Equal(t, "backup-job", j.GetName())
}

func TestJobAction_Type(t *testing.T) {
	j := &JobAction{}
	assert.Equal(t, dpv1alpha1.ActionTypeJob, j.Type())
}

func TestJobAction_Validate_MissingName(t *testing.T) {
	j := &JobAction{
		ObjectMeta: metav1.ObjectMeta{Name: ""},
		PodSpec:    newTestPodSpec(),
	}
	err := j.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestJobAction_Validate_MissingPodSpec(t *testing.T) {
	j := &JobAction{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1"},
		PodSpec:    nil,
	}
	err := j.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PodSpec is required")
}

func TestJobAction_Validate_DefaultBackOffLimit(t *testing.T) {
	j := &JobAction{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1"},
		PodSpec:    newTestPodSpec(),
	}
	err := j.validate()
	require.NoError(t, err)
	require.NotNil(t, j.BackOffLimit)
	assert.Equal(t, dptypes.DefaultBackOffLimit, *j.BackOffLimit)
}

func TestJobAction_Validate_CustomBackOffLimit(t *testing.T) {
	limit := int32(5)
	j := &JobAction{
		ObjectMeta:   metav1.ObjectMeta{Name: "job-1"},
		PodSpec:      newTestPodSpec(),
		BackOffLimit: &limit,
	}
	err := j.validate()
	require.NoError(t, err)
	assert.Equal(t, int32(5), *j.BackOffLimit)
}

func TestJobAction_Execute_CreateJob(t *testing.T) {
	scheme := jobTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := newTestOwner("default")

	j := &JobAction{
		Name: "create-test",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "create-test",
			Namespace: "default",
		},
		PodSpec: newTestPodSpec(),
		Owner:   owner,
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := j.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
	assert.Equal(t, "create-test", status.Name)

	// verify job was created
	created := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "create-test"}, created)
	require.NoError(t, err)
	assert.Contains(t, created.Finalizers, dptypes.DataProtectionFinalizerName)
}

func TestJobAction_Execute_JobExists_Running(t *testing.T) {
	scheme := jobTestScheme()
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-job",
			Namespace: "default",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()

	j := &JobAction{
		Name: "running-job",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-job",
			Namespace: "default",
		},
		PodSpec: newTestPodSpec(),
		Owner:   newTestOwner("default"),
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := j.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
}

func TestJobAction_Execute_JobExists_Completed(t *testing.T) {
	scheme := jobTestScheme()
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "completed-job",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()

	j := &JobAction{
		Name: "completed-job",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "completed-job",
			Namespace: "default",
		},
		PodSpec: newTestPodSpec(),
		Owner:   newTestOwner("default"),
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := j.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseCompleted, status.Phase)
	require.NotNil(t, status.CompletionTimestamp)
}

func TestJobAction_Execute_JobExists_Failed(t *testing.T) {
	scheme := jobTestScheme()
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failed-job",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:    batchv1.JobFailed,
					Status:  corev1.ConditionTrue,
					Message: "BackoffLimitExceeded",
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()

	j := &JobAction{
		Name: "failed-job",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failed-job",
			Namespace: "default",
		},
		PodSpec: newTestPodSpec(),
		Owner:   newTestOwner("default"),
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := j.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseFailed, status.Phase)
	require.NotNil(t, status.CompletionTimestamp)
}

func TestJobAction_Execute_CrossNamespace(t *testing.T) {
	scheme := jobTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	j := &JobAction{
		Name: "cross-ns-job",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cross-ns-job",
			Namespace: "backup-ns",
		},
		PodSpec: newTestPodSpec(),
		Owner:   newTestOwner("other-ns"), // different namespace
	}

	actCtx := ActionContext{
		Ctx:      context.Background(),
		Client:   cli,
		Recorder: record.NewFakeRecorder(10),
		Scheme:   scheme,
	}

	status, err := j.Execute(actCtx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)

	// verify job was created without owner ref (cross-namespace)
	created := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "backup-ns", Name: "cross-ns-job"}, created)
	require.NoError(t, err)
	assert.Empty(t, created.OwnerReferences)
}

func TestJobAction_Execute_ValidationFails(t *testing.T) {
	j := &JobAction{
		ObjectMeta: metav1.ObjectMeta{Name: ""},
		PodSpec:    newTestPodSpec(),
	}
	actCtx := ActionContext{
		Ctx:      context.Background(),
		Recorder: record.NewFakeRecorder(10),
	}

	status, err := j.Execute(actCtx)
	require.Error(t, err)
	require.NotNil(t, status)
	assert.Equal(t, dpv1alpha1.ActionPhaseFailed, status.Phase)
}
