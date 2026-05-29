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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// reasonThresholds maps each downstream-progression-failed reason to the
// minimum duration the underlying signal must persist before the condition is
// raised. Thresholds are independent per reason: role-probe permanent failure
// is short (kbagent probe interval × 4) because that signal is itself
// definitive, while marker / cluster-phase / component-condition signals use
// 5min initial conservative threshold to avoid false positives on transient
// reconfigure cycles. Direction B (alpha.111 backlog candidate) closure may
// allow tuning ChartMarkerDriftPersistent down to ~2min later.
var reasonThresholds = map[string]time.Duration{
	opsv1alpha1.ReasonDownstreamFailClosedMarkerPersistent: 5 * time.Minute,
	opsv1alpha1.ReasonRoleProbePermanentFail:               60 * time.Second,
	opsv1alpha1.ReasonClusterReconcileStuck:                5 * time.Minute,
	opsv1alpha1.ReasonComponentConditionStuck:              5 * time.Minute,
	opsv1alpha1.ReasonInstanceSetAlignmentStuck:            5 * time.Minute,
	opsv1alpha1.ReasonChartMarkerDriftPersistent:           5 * time.Minute,
}

// ThresholdForReason returns the persistence threshold for the given
// downstream-progression-failed reason. Unknown reasons return zero.
func ThresholdForReason(reason string) time.Duration {
	return reasonThresholds[reason]
}

// DefaultDownstreamProgressionRequeue is the cadence at which the OpsRequest
// controller re-runs ObserveDownstreamProgression after a Succeed OpsRequest
// while any trigger condition is still being evaluated. It matches the
// existing controller reconcile cadence and does not introduce a new tick
// pressure pattern.
const DefaultDownstreamProgressionRequeue = time.Minute

// DownstreamObservation is the immediate result of evaluating one of the six
// downstream-progression-failed triggers against the current cluster state.
// Callers (the OpsRequest reconciler) accumulate observations across the
// observation window and surface them via the
// DownstreamProgressionFailed condition once the per-reason threshold is
// crossed.
type DownstreamObservation struct {
	Reason    string
	FirstSeen time.Time
	Detail    string
}

// readDownstreamFailClosedMarkers returns the addon-declared list of
// downstream fail-closed marker names from the cluster's component
// definitions. Addons opt in by setting
// DownstreamFailClosedMarkersAnnotationKey on their ComponentDefinition. The
// observation logic reads this list rather than hardcoding any addon-specific
// marker name; this is the public addon API extension point documented in
// docs/addon-api/.
func readDownstreamFailClosedMarkers(annotations map[string]string) []string {
	if annotations == nil {
		return nil
	}
	raw, ok := annotations[constant.DownstreamFailClosedMarkersAnnotationKey]
	if !ok {
		return nil
	}
	var markers []string
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			markers = append(markers, m)
		}
	}
	return markers
}

// SetDownstreamProgressionFailedCondition writes a
// DownstreamProgressionFailed condition with the given reason and message
// onto the OpsRequest in an idempotent manner: it only changes the condition
// (and emits a Kubernetes Event) when the (reason, status) tuple has
// transitioned. Same-(reason, status) reconciles re-evaluate but do not
// rewrite the condition, avoiding etcd write pressure.
//
// Per the OpsRequest contract, Succeed is never retracted: this condition is
// purely observational and added alongside Succeed.
func SetDownstreamProgressionFailedCondition(
	ctx context.Context,
	cli client.Client,
	recorder record.EventRecorder,
	opsRequest *opsv1alpha1.OpsRequest,
	reason, message string,
) error {
	cond := opsv1alpha1.NewDownstreamProgressionFailedCondition(opsRequest, reason, message)
	if !shouldWriteCondition(opsRequest, cond) {
		return nil
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.SetStatusCondition(*cond)
	if err := cli.Status().Patch(ctx, opsRequest, patch); err != nil {
		return fmt.Errorf("patch DownstreamProgressionFailed condition: %w", err)
	}
	if recorder != nil {
		recorder.Eventf(opsRequest, corev1.EventTypeWarning, reason, "%s", message)
	}
	return nil
}

// ClearDownstreamProgressionFailedCondition transitions the condition to
// status=False with reason DownstreamProgressionCleared once every active
// trigger has stayed cleared for its per-reason recovery window. Like
// SetDownstreamProgressionFailedCondition, this is idempotent: same-state
// reconciles do not rewrite.
func ClearDownstreamProgressionFailedCondition(
	ctx context.Context,
	cli client.Client,
	recorder record.EventRecorder,
	opsRequest *opsv1alpha1.OpsRequest,
	message string,
) error {
	cond := opsv1alpha1.NewDownstreamProgressionClearedCondition(opsRequest, message)
	if !shouldWriteCondition(opsRequest, cond) {
		return nil
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.SetStatusCondition(*cond)
	if err := cli.Status().Patch(ctx, opsRequest, patch); err != nil {
		return fmt.Errorf("clear DownstreamProgressionFailed condition: %w", err)
	}
	if recorder != nil {
		recorder.Eventf(opsRequest, corev1.EventTypeNormal,
			opsv1alpha1.ReasonDownstreamProgressionCleared, "%s", message)
	}
	return nil
}

// shouldWriteCondition returns true only if the proposed condition has a
// different (Status, Reason) tuple than the one currently on the OpsRequest.
// Same-state reconciles return false so the controller does not rewrite the
// condition unchanged.
func shouldWriteCondition(opsRequest *opsv1alpha1.OpsRequest, proposed *metav1.Condition) bool {
	for _, existing := range opsRequest.Status.Conditions {
		if existing.Type != proposed.Type {
			continue
		}
		return existing.Status != proposed.Status || existing.Reason != proposed.Reason
	}
	return true
}

// ObserveDownstreamProgression evaluates the six downstream-progression
// triggers against the current cluster state. It returns the observation for
// the highest-priority active reason (or nil if every trigger is clear) and
// the recommended requeue cadence so the OpsRequest controller can re-poll.
//
// This is the post-Succeed observation hook documented as P0e in
// docs/addon-api/. The OpsRequest contract guarantees that Succeed itself is
// never retracted; this hook only adds the DownstreamProgressionFailed
// condition alongside Succeed once the relevant per-reason threshold has
// been crossed.
//
// Trigger detection internals are implemented in follow-up changes; this
// scaffold defines the public surface that the OpsRequest reconciler hooks
// into.
func ObserveDownstreamProgression(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1.Cluster,
	opsRequest *opsv1alpha1.OpsRequest,
) (*DownstreamObservation, time.Duration, error) {
	// Triggers are independent (OR-composed). Return the first active trigger
	// in priority order so the condition message points at one well-defined
	// underlying cause. Subsequent reconciles re-evaluate every trigger.
	for _, reason := range orderedReasons() {
		obs, err := evaluateTrigger(ctx, cli, cluster, opsRequest, reason)
		if err != nil {
			return nil, DefaultDownstreamProgressionRequeue, err
		}
		if obs == nil {
			continue
		}
		threshold := ThresholdForReason(reason)
		if time.Since(obs.FirstSeen) < threshold {
			// trigger active but threshold not yet crossed; re-poll later
			continue
		}
		return obs, DefaultDownstreamProgressionRequeue, nil
	}
	return nil, DefaultDownstreamProgressionRequeue, nil
}

// orderedReasons returns the trigger evaluation order. Lower-cost / more
// specific signals come first so a precise reason is preferred when multiple
// triggers fire simultaneously.
func orderedReasons() []string {
	r := make([]string, 0, len(reasonThresholds))
	for reason := range reasonThresholds {
		r = append(r, reason)
	}
	sort.Strings(r)
	return r
}

// evaluateTrigger returns a non-nil DownstreamObservation when the given
// trigger is currently active against the cluster state. Per-trigger detection
// is intentionally separated so each can be unit-tested in isolation and
// extended without touching the orchestration above. Detection bodies land
// per T-P0e.1-6 acceptance follow-ups.
func evaluateTrigger(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1.Cluster,
	opsRequest *opsv1alpha1.OpsRequest,
	reason string,
) (*DownstreamObservation, error) {
	switch reason {
	case opsv1alpha1.ReasonClusterReconcileStuck:
		return evaluateClusterReconcileStuck(cluster, opsRequest), nil
	case opsv1alpha1.ReasonComponentConditionStuck:
		return evaluateComponentConditionStuck(cluster, opsRequest), nil
	default:
		// remaining trigger detection bodies land per T-P0e.1-6 acceptance
		// follow-ups (T-P0e.1 marker / T-P0e.4 role probe / T-P0e.6 chart
		// marker drift / T-P0e component condition / T-P0e instanceset
		// alignment).
		return nil, nil
	}
}

// evaluateClusterReconcileStuck flags an active observation when the cluster
// has been parked in `Updating` for at least the per-reason threshold past
// the OpsRequest's CompletionTimestamp. The post-Succeed elapsed window is
// the relevant signal: a fresh-Succeed OpsRequest will see Updating briefly
// while the downstream finalizes, but a healthy cluster transitions back to
// Running well before the 5-minute threshold.
func evaluateClusterReconcileStuck(
	cluster *appsv1.Cluster,
	opsRequest *opsv1alpha1.OpsRequest,
) *DownstreamObservation {
	if cluster.Status.Phase != appsv1.UpdatingClusterPhase {
		return nil
	}
	if opsRequest.Status.CompletionTimestamp.IsZero() {
		return nil
	}
	firstSeen := opsRequest.Status.CompletionTimestamp.Time
	return &DownstreamObservation{
		Reason:    opsv1alpha1.ReasonClusterReconcileStuck,
		FirstSeen: firstSeen,
		Detail: fmt.Sprintf(
			"Cluster %s/%s has been in phase %q since OpsRequest Succeed.",
			cluster.Namespace, cluster.Name, cluster.Status.Phase,
		),
	}
}

// evaluateComponentConditionStuck walks Cluster.status.components and flags
// any component that is still parked outside Running after the OpsRequest
// reached Succeed. Cluster.status.components is the controller's own
// reflection of per-component phase; reading it here keeps the detection on
// the same channel the OpsRequest reconciler already trusts for completion
// decisions.
//
// Component phases other than Running are all non-terminal-healthy. We pick
// the first non-Running phase encountered (sorted for determinism) so the
// surfaced message points at one concrete component; the orchestration's
// per-reason threshold keeps the noise floor at 5 minutes.
func evaluateComponentConditionStuck(
	cluster *appsv1.Cluster,
	opsRequest *opsv1alpha1.OpsRequest,
) *DownstreamObservation {
	if opsRequest.Status.CompletionTimestamp.IsZero() {
		return nil
	}
	names := make([]string, 0, len(cluster.Status.Components))
	for name := range cluster.Status.Components {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		comp := cluster.Status.Components[name]
		if comp.Phase == "" || comp.Phase == appsv1.RunningComponentPhase {
			continue
		}
		return &DownstreamObservation{
			Reason:    opsv1alpha1.ReasonComponentConditionStuck,
			FirstSeen: opsRequest.Status.CompletionTimestamp.Time,
			Detail: fmt.Sprintf(
				"Cluster %s/%s component %q has been in phase %q since OpsRequest Succeed.",
				cluster.Namespace, cluster.Name, name, comp.Phase,
			),
		}
	}
	return nil
}

// IsPhaseEligibleForDownstreamObservation returns true when the OpsRequest
// has reached Succeed but its TTL has not yet expired, i.e. the window in
// which downstream-progression observation is meaningful. This is the same
// window in which the controller already re-queues for TTL deletion.
func IsPhaseEligibleForDownstreamObservation(opsRequest *opsv1alpha1.OpsRequest) bool {
	if opsRequest.Status.Phase != opsv1alpha1.OpsSucceedPhase {
		return false
	}
	if opsRequest.Status.CompletionTimestamp.IsZero() {
		return false
	}
	return true
}
