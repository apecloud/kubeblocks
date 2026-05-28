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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type mockAction struct {
	name       string
	actionType dpv1alpha1.ActionType
}

func (m *mockAction) Execute(_ ActionContext) (*dpv1alpha1.ActionStatus, error) { return nil, nil }
func (m *mockAction) GetName() string                                           { return m.name }
func (m *mockAction) Type() dpv1alpha1.ActionType                               { return m.actionType }

func TestNewStatusBuilder(t *testing.T) {
	a := &mockAction{name: "test-action", actionType: dpv1alpha1.ActionTypeJob}
	sb := newStatusBuilder(a)
	status := sb.build()

	assert.Equal(t, "test-action", status.Name)
	assert.Equal(t, dpv1alpha1.ActionTypeJob, status.ActionType)
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
	require.NotNil(t, status.StartTimestamp)
}

func TestStatusBuilder_Phase(t *testing.T) {
	a := &mockAction{name: "a"}
	status := newStatusBuilder(a).phase(dpv1alpha1.ActionPhaseCompleted).build()
	assert.Equal(t, dpv1alpha1.ActionPhaseCompleted, status.Phase)
}

func TestStatusBuilder_Reason(t *testing.T) {
	a := &mockAction{name: "a"}
	status := newStatusBuilder(a).reason("something failed").build()
	assert.Equal(t, "something failed", status.FailureReason)
}

func TestStatusBuilder_StartTimestamp_WithValue(t *testing.T) {
	a := &mockAction{name: "a"}
	ts := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	status := newStatusBuilder(a).startTimestamp(&ts).build()
	assert.Equal(t, ts.Time, status.StartTimestamp.Time)
}

func TestStatusBuilder_StartTimestamp_Nil(t *testing.T) {
	a := &mockAction{name: "a"}
	before := time.Now().Add(-time.Second)
	status := newStatusBuilder(a).startTimestamp(nil).build()
	assert.True(t, status.StartTimestamp.Time.After(before))
}

func TestStatusBuilder_VolumeSnapshots(t *testing.T) {
	a := &mockAction{name: "a"}
	vs := []dpv1alpha1.VolumeSnapshotStatus{
		{Name: "snap-1", VolumeName: "vol-1"},
	}
	status := newStatusBuilder(a).volumeSnapshots(vs).build()
	require.Len(t, status.VolumeSnapshots, 1)
	assert.Equal(t, "snap-1", status.VolumeSnapshots[0].Name)
}

func TestStatusBuilder_CompletionTimestamp_WithValue(t *testing.T) {
	a := &mockAction{name: "a"}
	ts := metav1.NewTime(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	status := newStatusBuilder(a).completionTimestamp(&ts).build()
	require.NotNil(t, status.CompletionTimestamp)
	assert.Equal(t, ts.Time, status.CompletionTimestamp.Time)
}

func TestStatusBuilder_CompletionTimestamp_Nil(t *testing.T) {
	a := &mockAction{name: "a"}
	before := time.Now().Add(-time.Second)
	status := newStatusBuilder(a).completionTimestamp(nil).build()
	require.NotNil(t, status.CompletionTimestamp)
	assert.True(t, status.CompletionTimestamp.Time.After(before))
}

func TestStatusBuilder_ObjectRef(t *testing.T) {
	a := &mockAction{name: "a"}
	ref := &corev1.ObjectReference{
		Kind:      "Job",
		Name:      "my-job",
		Namespace: "default",
	}
	status := newStatusBuilder(a).objectRef(ref).build()
	require.NotNil(t, status.ObjectRef)
	assert.Equal(t, "my-job", status.ObjectRef.Name)
}

func TestStatusBuilder_WithErr_NonNil(t *testing.T) {
	a := &mockAction{name: "a"}
	status := newStatusBuilder(a).withErr(errors.New("broken")).build()
	assert.Equal(t, dpv1alpha1.ActionPhaseFailed, status.Phase)
	assert.Equal(t, "broken", status.FailureReason)
}

func TestStatusBuilder_WithErr_Nil(t *testing.T) {
	a := &mockAction{name: "a"}
	status := newStatusBuilder(a).withErr(nil).build()
	assert.Equal(t, dpv1alpha1.ActionPhaseRunning, status.Phase)
	assert.Empty(t, status.FailureReason)
}

func TestStatusBuilder_TotalSize(t *testing.T) {
	a := &mockAction{name: "a"}
	status := newStatusBuilder(a).totalSize("1Gi").build()
	assert.Equal(t, "1Gi", status.TotalSize)
}

func TestStatusBuilder_TimeRange(t *testing.T) {
	a := &mockAction{name: "a"}
	start := &metav1.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	end := &metav1.Time{Time: time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)}
	status := newStatusBuilder(a).timeRange(start, end).build()
	require.NotNil(t, status.TimeRange)
	assert.Equal(t, start, status.TimeRange.Start)
	assert.Equal(t, end, status.TimeRange.End)
}

func TestStatusBuilder_Chaining(t *testing.T) {
	a := &mockAction{name: "chain-test", actionType: dpv1alpha1.ActionTypeJob}
	ref := &corev1.ObjectReference{Kind: "Job", Name: "j1"}
	status := newStatusBuilder(a).
		phase(dpv1alpha1.ActionPhaseCompleted).
		reason("done").
		totalSize("500Mi").
		objectRef(ref).
		completionTimestamp(nil).
		build()

	assert.Equal(t, "chain-test", status.Name)
	assert.Equal(t, dpv1alpha1.ActionPhaseCompleted, status.Phase)
	assert.Equal(t, "done", status.FailureReason)
	assert.Equal(t, "500Mi", status.TotalSize)
	assert.Equal(t, "j1", status.ObjectRef.Name)
	require.NotNil(t, status.CompletionTimestamp)
}
