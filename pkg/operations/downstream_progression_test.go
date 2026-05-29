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

package operations

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// TestThresholdForReason locks in the per-reason persistence threshold map.
// RoleProbePermanentFail is short (60s, kbagent probe interval × 4) because
// the underlying signal is itself definitive; the other reasons use 5min
// initial conservative thresholds and may be tuned shorter once chart marker
// state-machine root-cause work (Direction B alpha.111) lands.
func TestThresholdForReason(t *testing.T) {
	cases := []struct {
		reason string
		want   time.Duration
	}{
		{opsv1alpha1.ReasonRoleProbePermanentFail, 60 * time.Second},
		{opsv1alpha1.ReasonDownstreamFailClosedMarkerPersistent, 5 * time.Minute},
		{opsv1alpha1.ReasonClusterReconcileStuck, 5 * time.Minute},
		{opsv1alpha1.ReasonComponentConditionStuck, 5 * time.Minute},
		{opsv1alpha1.ReasonInstanceSetAlignmentStuck, 5 * time.Minute},
		{opsv1alpha1.ReasonChartMarkerDriftPersistent, 5 * time.Minute},
		{"UnknownReason", 0},
	}
	for _, tc := range cases {
		if got := ThresholdForReason(tc.reason); got != tc.want {
			t.Errorf("ThresholdForReason(%q) = %v, want %v", tc.reason, got, tc.want)
		}
	}
}

// TestReadDownstreamFailClosedMarkers verifies the addon-extensible marker
// registry parses correctly from a comma-separated annotation value.
func TestReadDownstreamFailClosedMarkers(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]string
		want []string
	}{
		{
			name: "nil annotations returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "missing key returns nil",
			in:   map[string]string{"unrelated": "value"},
			want: nil,
		},
		{
			name: "single marker",
			in: map[string]string{
				constant.DownstreamFailClosedMarkersAnnotationKey: ".replication-divergence-pending",
			},
			want: []string{".replication-divergence-pending"},
		},
		{
			name: "comma-separated with whitespace",
			in: map[string]string{
				constant.DownstreamFailClosedMarkersAnnotationKey: ".replication-divergence-pending, .bootstrap-fenced ,",
			},
			want: []string{".replication-divergence-pending", ".bootstrap-fenced"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := readDownstreamFailClosedMarkers(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got[%d]=%q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestShouldWriteCondition_Idempotent verifies the idempotency guarantee
// documented on SetDownstreamProgressionFailedCondition: same-state reconciles
// must not rewrite the condition. Only a transition in (Status, Reason)
// triggers a write.
func TestShouldWriteCondition_Idempotent(t *testing.T) {
	ops := &opsv1alpha1.OpsRequest{}
	existing := metav1.Condition{
		Type:   opsv1alpha1.ConditionTypeDownstreamProgressionFailed,
		Status: metav1.ConditionTrue,
		Reason: opsv1alpha1.ReasonRoleProbePermanentFail,
	}
	ops.Status.Conditions = []metav1.Condition{existing}

	// Same type+status+reason: skip write.
	same := opsv1alpha1.NewDownstreamProgressionFailedCondition(
		ops, opsv1alpha1.ReasonRoleProbePermanentFail, "still failing")
	if shouldWriteCondition(ops, same) {
		t.Errorf("same (status, reason) tuple should not trigger write")
	}

	// Reason change: write.
	reasonChanged := opsv1alpha1.NewDownstreamProgressionFailedCondition(
		ops, opsv1alpha1.ReasonChartMarkerDriftPersistent, "different trigger")
	if !shouldWriteCondition(ops, reasonChanged) {
		t.Errorf("reason change should trigger write")
	}

	// Status flip True→False: write.
	cleared := opsv1alpha1.NewDownstreamProgressionClearedCondition(ops, "all clear")
	if !shouldWriteCondition(ops, cleared) {
		t.Errorf("status flip True→False should trigger write")
	}

	// Type does not exist yet: write.
	emptyOps := &opsv1alpha1.OpsRequest{}
	if !shouldWriteCondition(emptyOps, same) {
		t.Errorf("absent condition should trigger initial write")
	}
}

// TestEvaluateClusterReconcileStuck covers the ClusterReconcileStuck trigger.
// The signal is straightforward — a Cluster parked in Updating after the
// OpsRequest reached Succeed — but the test pins the three cases the caller
// relies on: phase mismatch returns nil, missing CompletionTimestamp returns
// nil, and an active observation carries the Succeed timestamp as FirstSeen
// so the orchestration above can apply the 5-minute persistence threshold.
func TestEvaluateClusterReconcileStuck(t *testing.T) {
	succeed := time.Now().Add(-10 * time.Minute)
	opsRequest := &opsv1alpha1.OpsRequest{
		Status: opsv1alpha1.OpsRequestStatus{
			Phase:               opsv1alpha1.OpsSucceedPhase,
			CompletionTimestamp: metav1.Time{Time: succeed},
		},
	}

	t.Run("not updating returns nil", func(t *testing.T) {
		cluster := &appsv1.Cluster{}
		cluster.Status.Phase = appsv1.RunningClusterPhase
		if got := evaluateClusterReconcileStuck(cluster, opsRequest); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("missing completion timestamp returns nil", func(t *testing.T) {
		cluster := &appsv1.Cluster{}
		cluster.Status.Phase = appsv1.UpdatingClusterPhase
		opsNoTS := &opsv1alpha1.OpsRequest{
			Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsSucceedPhase},
		}
		if got := evaluateClusterReconcileStuck(cluster, opsNoTS); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("updating with completion returns observation", func(t *testing.T) {
		cluster := &appsv1.Cluster{}
		cluster.Namespace = "ns-a"
		cluster.Name = "demo"
		cluster.Status.Phase = appsv1.UpdatingClusterPhase
		obs := evaluateClusterReconcileStuck(cluster, opsRequest)
		if obs == nil {
			t.Fatalf("expected observation, got nil")
		}
		if obs.Reason != opsv1alpha1.ReasonClusterReconcileStuck {
			t.Errorf("reason: got %q, want %q", obs.Reason, opsv1alpha1.ReasonClusterReconcileStuck)
		}
		if !obs.FirstSeen.Equal(succeed) {
			t.Errorf("FirstSeen: got %v, want %v", obs.FirstSeen, succeed)
		}
		if obs.Detail == "" {
			t.Errorf("Detail should be non-empty")
		}
	})
}

// TestIsPhaseEligibleForDownstreamObservation locks in the observation gate:
// only OpsSucceedPhase with a non-zero CompletionTimestamp is eligible. This
// guarantees the observation never runs against a still-running or
// not-yet-completed OpsRequest.
func TestIsPhaseEligibleForDownstreamObservation(t *testing.T) {
	cases := []struct {
		name string
		ops  *opsv1alpha1.OpsRequest
		want bool
	}{
		{
			name: "succeed with completion timestamp",
			ops: &opsv1alpha1.OpsRequest{
				Status: opsv1alpha1.OpsRequestStatus{
					Phase:               opsv1alpha1.OpsSucceedPhase,
					CompletionTimestamp: metav1.Time{Time: time.Now()},
				},
			},
			want: true,
		},
		{
			name: "succeed without completion timestamp",
			ops: &opsv1alpha1.OpsRequest{
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsSucceedPhase},
			},
			want: false,
		},
		{
			name: "running",
			ops: &opsv1alpha1.OpsRequest{
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase},
			},
			want: false,
		},
		{
			name: "failed",
			ops: &opsv1alpha1.OpsRequest{
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsFailedPhase},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPhaseEligibleForDownstreamObservation(tc.ops); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
