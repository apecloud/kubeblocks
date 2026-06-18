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

package workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

// --- parser tests ---

func TestParseRoleProbeOutputSingleToken(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte("primary"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.role != "primary" || out.hasAuthoritativeVersion || out.authoritativeVersion != 0 {
		t.Fatalf("got %+v, want role=primary without authoritative version", out)
	}
}

func TestParseRoleProbeOutputVersionedSpaceSeparated(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte("primary 10"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.role != "primary" || !out.hasAuthoritativeVersion || out.authoritativeVersion != 10 {
		t.Fatalf("got %+v, want role=primary authoritativeVersion=10", out)
	}
}

func TestParseRoleProbeOutputVersionedNewlineSeparated(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte("primary\n10"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.role != "primary" || !out.hasAuthoritativeVersion || out.authoritativeVersion != 10 {
		t.Fatalf("got %+v, want role=primary authoritativeVersion=10", out)
	}
}

func TestParseRoleProbeOutputVersionedTolerantOfSurroundingWhitespace(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte("\tprimary\t42\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.role != "primary" || !out.hasAuthoritativeVersion || out.authoritativeVersion != 42 {
		t.Fatalf("got %+v, want role=primary authoritativeVersion=42", out)
	}
}

func TestParseRoleProbeOutputMalformedSecondTokenNotUint64(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte("primary abc"))
	if err == nil {
		t.Fatalf("got nil error for %+v", out)
	}
}

func TestParseRoleProbeOutputMalformedThreeOrMoreTokens(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte("primary 10 extra"))
	if err == nil {
		t.Fatalf("got nil error for %+v", out)
	}
}

func TestParseRoleProbeOutputEmpty(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.hasAuthoritativeVersion || out.role != "" {
		t.Fatalf("got %+v, want empty role without authoritative version", out)
	}
}

func TestParseRoleProbeOutputWhitespaceOnly(t *testing.T) {
	out, err := parseRoleProbeOutput([]byte(" \n\t "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.hasAuthoritativeVersion || out.role != "" {
		t.Fatalf("got %+v, want empty role without authoritative version", out)
	}
}

// --- gate tests: each path consults only its own annotation key ---

func TestAcceptRoleProbeEventVersionedRejectsOlderVersion(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleAuthoritativeVersionAnnotationKey: "10"})
	parsed := versionedRoleProbeOutput("primary", 9)
	if acceptRoleProbeEvent(pod, "0", parsed) {
		t.Fatalf("expected stale versioned result to be rejected")
	}
}

func TestAcceptRoleProbeEventVersionedRejectsEqualVersion(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleAuthoritativeVersionAnnotationKey: "10"})
	parsed := versionedRoleProbeOutput("primary", 10)
	if acceptRoleProbeEvent(pod, "0", parsed) {
		t.Fatalf("expected equal versioned result to be rejected")
	}
}

func TestAcceptRoleProbeEventVersionedAcceptsNewerVersion(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleAuthoritativeVersionAnnotationKey: "10"})
	parsed := versionedRoleProbeOutput("primary", 11)
	if !acceptRoleProbeEvent(pod, "0", parsed) {
		t.Fatalf("expected newer versioned result to be accepted")
	}
}

// Versioned results do not consult the single-token annotation key.
func TestAcceptRoleProbeEventVersionedIgnoresSingleTokenAnnotation(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550000000000"})
	parsed := versionedRoleProbeOutput("primary", 1)
	if !acceptRoleProbeEvent(pod, "1", parsed) {
		t.Fatalf("expected versioned result to ignore single-token EventTime anchor")
	}
}

func TestAcceptRoleProbeEventSingleTokenAcceptsNewerEventTime(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550000000000"})
	parsed := roleProbeOutput{role: "primary"}
	if !acceptRoleProbeEvent(pod, "1779550600000000", parsed) {
		t.Fatalf("expected newer single-token result to be accepted")
	}
}

func TestAcceptRoleProbeEventSingleTokenRejectsEqualEventTime(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550600000000"})
	parsed := roleProbeOutput{role: "primary"}
	if acceptRoleProbeEvent(pod, "1779550600000000", parsed) {
		t.Fatalf("expected repeated single-token result with equal EventTime to be rejected")
	}
}

func TestAcceptRoleProbeEventSingleTokenRejectsOlderEventTime(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550600000000"})
	parsed := roleProbeOutput{role: "primary"}
	if acceptRoleProbeEvent(pod, "1779550000000000", parsed) {
		t.Fatalf("expected stale single-token result to be rejected")
	}
}

// A Pod that has accepted any versioned result rejects subsequent
// single-token results from the same Pod, avoiding downgrade from
// authoritative role-version ordering.
func TestAcceptRoleProbeEventSingleTokenRejectedOnPodAlreadyAcceptedVersionedEvent(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleAuthoritativeVersionAnnotationKey: "10"})
	parsed := roleProbeOutput{role: "primary"}
	if acceptRoleProbeEvent(pod, "1779550600000000", parsed) {
		t.Fatalf("expected same-Pod single-token result to be rejected after a versioned result")
	}
}

// The same-Pod downgrade rule also blocks single-token results even if the
// Pod has accumulated a single-token annotation alongside the roleVersion
// annotation.
func TestAcceptRoleProbeEventSingleTokenRejectedOnPodWithBothAnnotationsWhenRoleVersionPresent(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "10",
		constant.LastRoleEventVersionAnnotationKey:         "1000000",
	})
	parsed := roleProbeOutput{role: "primary"}
	if acceptRoleProbeEvent(pod, "2000000", parsed) {
		t.Fatalf("expected single-token result to be rejected when roleVersion anchor is present")
	}
}

func TestAcceptRoleProbeEventAcceptsUnparseableStoredOrIncomingVersions(t *testing.T) {
	parsed := versionedRoleProbeOutput("primary", 1)
	pod := podWithAnnotations(map[string]string{constant.LastRoleAuthoritativeVersionAnnotationKey: "bad"})
	if !acceptRoleProbeEvent(pod, "0", parsed) {
		t.Fatalf("expected bad stored authoritative version to be accepted")
	}

	parsed = roleProbeOutput{role: "primary"}
	pod = podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "bad"})
	if !acceptRoleProbeEvent(pod, "1", parsed) {
		t.Fatalf("expected bad stored event version to be accepted")
	}
	pod = podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1"})
	if !acceptRoleProbeEvent(pod, "bad", parsed) {
		t.Fatalf("expected bad incoming event version to be accepted")
	}
}

// --- end-to-end handler tests via fake client ---

func TestRoleEventHandlerIgnoresNonRoleProbeEvent(t *testing.T) {
	event := builder.NewEventBuilder("default", "event-1").
		SetReason("not-role-probe").
		SetReportingController(proto.ProbeEventReportingController).
		GetObject()

	handled, err := (&RoleEventHandler{}).Handle(roleEventFakeClient(t, event), intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Log: logr.Discard(),
	}, nil, event)
	if err != nil {
		t.Fatalf("handle event failed: %v", err)
	}
	if handled {
		t.Fatalf("expected non-role probe event to be ignored")
	}
}

func TestRoleEventHandlerHandlesInstanceSetSingleTokenAndExclusiveCleanupStampsPeerAnnotation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
	})
	otherPod := roleEventPod("default", "mysql-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "mysql",
		constant.RoleLabelKey:                  "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	cli := roleEventFakeClient(t, its, pod, otherPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	wantSingleToken := fmt.Sprintf("%d", event.EventTime.UnixMicro())
	assertPodRole(t, ctx, cli, pod, "leader", wantSingleToken)
	// Peer's exclusive role label is stripped. Single-token cleanup also stamps
	// the peer's LastRoleEventVersionAnnotationKey with the cleanup event's
	// EventTime: without this stamp a delayed single-token event from the demoted
	// primary whose EventTime is older than the cleanup but newer than the
	// peer's own previous annotation would slip back through the gate.
	assertPodRole(t, ctx, cli, otherPod, "", wantSingleToken)
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, otherPod, "")
}

// Versioned path peer cleanup must strip the label but leave the peer's
// LastRoleAuthoritativeVersionAnnotationKey untouched. Stamping it would let the
// strict-newer gate later reject a legitimate event from the peer at the
// same versioned epoch (e.g. demoted pod emitting `secondary <same-epoch>`).
func TestRoleEventHandlerHandlesInstanceSetVersionedAndExclusiveCleanupDoesNotStampPeerVersionedAnnotation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
	})
	otherPod := roleEventPod("default", "mysql-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "mysql",
		constant.RoleLabelKey:                  "leader",
	})
	otherPod.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "0",
	}
	event := roleProbeEventWithOutput("default", "event-1", pod, "leader 1", now)
	cli := roleEventFakeClient(t, its, pod, otherPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "1")
	assertPodRole(t, ctx, cli, otherPod, "", "")
	// Critical contract: peer versioned annotation is NOT advanced; otherwise
	// the peer's own next event at authoritative role version 1 would be rejected as
	// stale by the strict-newer gate.
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, otherPod, "0")
}

// Regression for Valkey r4 mixed-mode bug on PR #10283 head 714f684b: a
// single-token `primary` event from a non-quorum probe script fallback path must not
// strip the exclusive role label off a versioned peer. Without
// this guard the single-token event runs exclusive cleanup against the
// versioned-held primary (the gate consults only the single-token annotation,
// which on the versioned peer is empty, so cleanup is accepted); after the
// label is stripped the versioned peer's next same-roleVersion event is
// rejected by the strict-newer gate and the role label can never be
// restored.
func TestRoleEventHandlerSingleTokenExclusiveEventBlockedByVersionedHoldingPeer(t *testing.T) {
	ctx := context.Background()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	versionedPeer := roleEventPod("default", "vlk-3", "uid-3", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	versionedPeer.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "3",
	}
	singleTokenPod := roleEventPod("default", "vlk-2", "uid-2", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	event := roleProbeEvent("default", "event-1", singleTokenPod, "leader", time.Now())
	cli := roleEventFakeClient(t, its, versionedPeer, singleTokenPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled (skipped + reason=versionedHeldExclusiveRole)")
	}

	// single-token pod must not have received the leader label; versioned peer's
	// label and annotations must be unchanged.
	assertPodRole(t, ctx, cli, singleTokenPod, "", "")
	assertPodLastRoleVersion(t, ctx, cli, singleTokenPod, "")
	assertPodRole(t, ctx, cli, versionedPeer, "leader", "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, versionedPeer, "3")
	assertPodLastRoleVersion(t, ctx, cli, versionedPeer, "")
}

// When the role being claimed is non-exclusive, the versioned-held-peer
// guard does not apply: a single-token event for a non-exclusive role still
// goes through normally even when another peer carries a versioned
// annotation.
func TestRoleEventHandlerSingleTokenNonExclusiveEventNotBlockedByVersionedPeer(t *testing.T) {
	ctx := context.Background()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	versionedPeer := roleEventPod("default", "vlk-3", "uid-3", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	versionedPeer.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "3",
	}
	singleTokenPod := roleEventPod("default", "vlk-2", "uid-2", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	event := roleProbeEvent("default", "event-1", singleTokenPod, "follower", time.Now())
	cli := roleEventFakeClient(t, its, versionedPeer, singleTokenPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	wantSingleToken := fmt.Sprintf("%d", event.EventTime.UnixMicro())
	assertPodRole(t, ctx, cli, singleTokenPod, "follower", wantSingleToken)
	// Versioned peer untouched (cleanup only runs for exclusive roles).
	assertPodRole(t, ctx, cli, versionedPeer, "leader", "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, versionedPeer, "3")
}

// A single-token exclusive event still runs normally when no peer holds the
// exclusive role with a versioned annotation. This pins that the guard
// only fires in mixed-mode coexistence.
func TestRoleEventHandlerSingleTokenExclusiveEventAcceptedWhenNoVersionedPeerHoldsRole(t *testing.T) {
	ctx := context.Background()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	// peer holds leader but only via single-token annotation, no versioned annotation
	singleTokenPeer := roleEventPod("default", "vlk-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	singleTokenPeer.Annotations = map[string]string{
		constant.LastRoleEventVersionAnnotationKey: "1000000",
	}
	singleTokenPod := roleEventPod("default", "vlk-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	now := time.Unix(0, 2000000*int64(time.Microsecond))
	event := roleProbeEvent("default", "event-1", singleTokenPod, "leader", now)
	cli := roleEventFakeClient(t, its, singleTokenPeer, singleTokenPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	wantSingleToken := fmt.Sprintf("%d", event.EventTime.UnixMicro())
	assertPodRole(t, ctx, cli, singleTokenPod, "leader", wantSingleToken)
	// single-token peer stripped + single-token annotation stamped per the
	// single-token-path one-way ratchet contract.
	assertPodRole(t, ctx, cli, singleTokenPeer, "", wantSingleToken)
}

func TestRoleEventHandlerVersionedExclusiveEventBlockedByPeerWithNewerVersionedVersion(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	versionedPeer := roleEventPod("default", "vlk-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	versionedPeer.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "5",
	}
	versionedPod := roleEventPod("default", "vlk-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	versionedPod.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "3",
	}
	event := roleProbeEventWithOutput("default", "event-1", versionedPod, "leader 4", now)
	cli := roleEventFakeClient(t, its, versionedPeer, versionedPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled (skipped + reason=stalePeerRoleEventVersion)")
	}

	assertPodRole(t, ctx, cli, versionedPod, "", "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, versionedPod, "3")
	assertPodRole(t, ctx, cli, versionedPeer, "leader", "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, versionedPeer, "5")
}

// Versioned events for an exclusive role are not affected by the
// single-token-only versioned-held-peer guard: a versioned event from one pod is
// allowed to take the role from an older versioned peer (and cleanup will
// strip the peer label as usual, gated by the versioned-version comparison).
func TestRoleEventHandlerVersionedExclusiveEventNotBlockedByVersionedPeerGuard(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	versionedPeer := roleEventPod("default", "vlk-2", "uid-2", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	versionedPeer.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "3",
	}
	versionedPod := roleEventPod("default", "vlk-3", "uid-3", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	event := roleProbeEventWithOutput("default", "event-1", versionedPod, "leader 5", now)
	cli := roleEventFakeClient(t, its, versionedPeer, versionedPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, versionedPod, "leader", "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, versionedPod, "5")
	// Peer label stripped by versioned-path cleanup (authoritative role version 5 > peer's 3).
	assertPodRole(t, ctx, cli, versionedPeer, "", "")
	// Peer versioned annotation untouched per the V1 fix.
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, versionedPeer, "3")
}

// Regression for the prior V1 round: after versioned-path cleanup strips the
// old primary's label, the old primary's own next roleProbe event at the
// same versioned epoch must be accepted. With per-path keys this is enforced
// directly by acceptRoleProbeEvent: the peer's annotation stays at the older
// version so a same-epoch secondary event from the peer is strictly newer.
func TestRoleEventVersionedCleanupLeavesPeerAbleToAcceptItsOwnSameEpochSecondaryEvent(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "0",
	})
	parsed := versionedRoleProbeOutput("secondary", 1)
	if !acceptRoleProbeEvent(pod, "0", parsed) {
		t.Fatalf("expected peer to accept same-epoch secondary event")
	}
}

func TestRoleEventHandlerHandlesInstanceRoleWithoutExclusiveCleanup(t *testing.T) {
	ctx := context.Background()
	newer := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloads.InstanceSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	})
	otherPod := roleEventPod("default", "mysql-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.KBAppInstanceNameLabelKey: "mysql-1",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", newer)
	staleEvent := roleProbeEvent("default", "event-2", pod, "follower", older)
	cli := roleEventFakeClient(t, inst, pod, otherPod, event, staleEvent)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}
	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
	assertPodRole(t, ctx, cli, otherPod, "leader", "")

	if handled := handleRoleEvent(t, ctx, cli, staleEvent); !handled {
		t.Fatalf("expected stale event to be handled")
	}
	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerDeletesUndefinedInstanceSetRole(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
		constant.RoleLabelKey:                 "follower",
	})
	event := roleProbeEvent("default", "event-1", pod, "unknown", now)
	cli := roleEventFakeClient(t, its, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerDeletesUndefinedInstanceRole(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloads.InstanceSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "follower",
	})
	event := roleProbeEvent("default", "event-1", pod, "unknown", now)
	cli := roleEventFakeClient(t, inst, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerPrefersControllerRefOverLabels(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
		constant.KBAppInstanceNameLabelKey:    "mysql-0",
	})
	setControllerRef(pod, workloads.GroupVersion.String(), workloads.InstanceSetKind, its.Name)
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	cli := roleEventFakeClient(t, its, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerPrefersInstanceControllerRefOverLabels(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloads.InstanceSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
		constant.KBAppInstanceNameLabelKey:    "mysql-0",
	})
	setControllerRef(pod, workloads.GroupVersion.String(), instanceKind, inst.Name)
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	cli := roleEventFakeClient(t, inst, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerConsumesInvalidProbeMessageWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
	event.Message = "{"
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesProbeFailureWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEventWithCode("default", "event-1", pod, "follower", time.Now(), 1, "probe failed")
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerRejectsMalformedRoleProbeOutput(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloads.InstanceSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}, {Name: "follower"}}},
	}
	event := roleProbeEventWithOutput("default", "event-1", pod, "follower abc", time.Now())
	cli := roleEventFakeClient(t, inst, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	// Role label must not change; both annotations must stay empty since
	// malformed output is rejected before any write.
	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesAmbiguousPodOwnerWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
		constant.KBAppInstanceNameLabelKey:    "mysql-0",
		constant.RoleLabelKey:                 "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected ambiguous owner event to be consumed")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesStalePodUIDWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-new", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
	event.InvolvedObject.UID = "uid-old"
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
	assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesPodNotFound(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", time.Now())
	cli := roleEventFakeClient(t, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}
}

func TestRoleEventHandlerConsumesMissingWorkloadWithoutPodUpdate(t *testing.T) {
	testCases := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "missing InstanceSet",
			labels: map[string]string{
				instanceset.WorkloadsInstanceLabelKey: "mysql",
				constant.RoleLabelKey:                 "leader",
			},
		},
		{
			name: "missing Instance",
			labels: map[string]string{
				constant.KBAppInstanceNameLabelKey: "mysql-0",
				constant.RoleLabelKey:              "leader",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pod := roleEventPod("default", "mysql-0", "uid-0", tc.labels)
			event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
			cli := roleEventFakeClient(t, pod, event)

			if handled := handleRoleEvent(t, ctx, cli, event); !handled {
				t.Fatalf("expected event to be handled")
			}

			assertPodRole(t, ctx, cli, pod, "leader", "")
			assertPodLastRoleVersion(t, ctx, cli, pod, "")
			assertPodLastRoleAuthoritativeVersion(t, ctx, cli, pod, "")
		})
	}
}

func TestRoleEventHandlerIgnoresUnknownOwnerWithoutMarkingHandled(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", nil)
	event := roleProbeEvent("default", "event-1", pod, "leader", time.Now())
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); handled {
		t.Fatalf("expected event not to be handled")
	}
}

func TestResolveRoleEventBranchByControllerRefRejectsUnknownRefs(t *testing.T) {
	pod := roleEventPod("default", "mysql-0", "uid-0", nil)
	setControllerRef(pod, ":", workloads.InstanceSetKind, "mysql")
	if _, _, ok := resolveRoleEventBranchByControllerRef(pod); ok {
		t.Fatalf("expected malformed apiVersion to be rejected")
	}

	setControllerRef(pod, "apps/v1", workloads.InstanceSetKind, "mysql")
	if _, _, ok := resolveRoleEventBranchByControllerRef(pod); ok {
		t.Fatalf("expected foreign api group to be rejected")
	}

	setControllerRef(pod, workloads.GroupVersion.String(), "Other", "mysql")
	if _, _, ok := resolveRoleEventBranchByControllerRef(pod); ok {
		t.Fatalf("expected unknown kind to be rejected")
	}
}

func TestUpdatePodRoleLabelNoopsWhenRoleAndAnnotationUnchanged(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.RoleLabelKey: "leader",
	})
	pod.Annotations = map[string]string{
		constant.LastRoleEventVersionAnnotationKey: "1000",
	}
	cli := roleEventFakeClient(t, pod)

	if err := updatePodRoleLabel(ctx, cli, pod, "leader", true, "1000", roleProbeOutput{role: "leader"}); err != nil {
		t.Fatalf("update pod role label failed: %v", err)
	}

	assertPodRole(t, ctx, cli, pod, "leader", "1000")
}

func TestRemoveExclusiveRoleLabelsSkipsSelfAndStalePeers(t *testing.T) {
	ctx := context.Background()
	its := workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"}}
	self := roleEventPod("default", "mysql-0", "uid-0", instanceSetRoleLabels("mysql", "leader"))
	stalePeer := roleEventPod("default", "mysql-1", "uid-1", instanceSetRoleLabels("mysql", "leader"))
	stalePeer.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "5",
	}
	cli := roleEventFakeClient(t, self, stalePeer)

	err := removeExclusiveRoleLabels(ctx, cli, its, self.Name, "leader", "0", versionedRoleProbeOutput("leader", 4))
	if err != nil {
		t.Fatalf("remove exclusive role labels failed: %v", err)
	}

	assertPodRole(t, ctx, cli, self, "leader", "")
	assertPodRole(t, ctx, cli, stalePeer, "leader", "")
}

func TestExclusiveRolePeerHelpersSkipSelfAndMalformedVersions(t *testing.T) {
	ctx := context.Background()
	its := workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"}}
	self := roleEventPod("default", "mysql-0", "uid-0", instanceSetRoleLabels("mysql", "leader"))
	self.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "5",
	}
	peerWithoutVersion := roleEventPod("default", "mysql-1", "uid-1", instanceSetRoleLabels("mysql", "leader"))
	peerWithBadVersion := roleEventPod("default", "mysql-2", "uid-2", instanceSetRoleLabels("mysql", "leader"))
	peerWithBadVersion.Annotations = map[string]string{
		constant.LastRoleAuthoritativeVersionAnnotationKey: "bad",
	}
	cli := roleEventFakeClient(t, self, peerWithoutVersion, peerWithBadVersion)

	held, err := versionedPeerHoldsExclusiveRole(ctx, cli, its, self.Name, "leader")
	if err != nil {
		t.Fatalf("check versioned peer failed: %v", err)
	}
	if !held {
		t.Fatalf("expected peer with malformed authoritative version annotation to count as held")
	}

	stale, err := newerOrEqualVersionedExclusiveRoleHeldByPeer(ctx, cli, its, self.Name, "leader", 1)
	if err != nil {
		t.Fatalf("check newer versioned peer failed: %v", err)
	}
	if stale {
		t.Fatalf("expected peers with missing or malformed versions to be ignored")
	}
}

func TestInstanceSetReconciler2InstanceFilter(t *testing.T) {
	ctx := context.Background()
	r := &InstanceSetReconciler2{}
	testCases := []struct {
		name      string
		labels    map[string]string
		wantCount int
		wantName  string
	}{
		{
			name:      "missing managed by",
			labels:    map[string]string{},
			wantCount: 0,
		},
		{
			name: "missing cluster",
			labels: map[string]string{
				constant.AppManagedByLabelKey: constant.AppName,
			},
			wantCount: 0,
		},
		{
			name: "missing component",
			labels: map[string]string{
				constant.AppManagedByLabelKey: constant.AppName,
				constant.AppInstanceLabelKey:  "mysql",
			},
			wantCount: 0,
		},
		{
			name: "valid instance labels",
			labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    "mysql",
				constant.KBAppComponentLabelKey: "proxy",
			},
			wantCount: 1,
			wantName:  constant.GenerateWorkloadNamePattern("mysql", "proxy"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "mysql-proxy-0",
					Labels:    tc.labels,
				},
			}
			requests := r.instanceFilter(ctx, pod)
			if len(requests) != tc.wantCount {
				t.Fatalf("got %d requests, want %d", len(requests), tc.wantCount)
			}
			if tc.wantCount == 1 {
				if requests[0].NamespacedName.Namespace != pod.Namespace || requests[0].NamespacedName.Name != tc.wantName {
					t.Fatalf("got request %s, want default/%s", requests[0].NamespacedName.String(), tc.wantName)
				}
			}
		})
	}
}

func TestLogRoleProbeEventErrorPath(t *testing.T) {
	logRoleProbeEvent(logr.Discard(), &roleEventResult{}, fmt.Errorf("boom"))
}

func handleRoleEvent(t *testing.T, ctx context.Context, cli client.Client, event *corev1.Event) bool {
	t.Helper()
	handled, err := (&RoleEventHandler{}).Handle(cli, intctrlutil.RequestCtx{
		Ctx: ctx,
		Log: logr.Discard(),
	}, nil, event)
	if err != nil {
		t.Fatalf("handle event failed: %v", err)
	}
	return handled
}

func roleEventFakeClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func roleEventPod(namespace, name, uid string, labels map[string]string) *corev1.Pod {
	pod := builder.NewPodBuilder(namespace, name).SetUID(types.UID(uid)).GetObject()
	if labels != nil {
		pod.Labels = labels
	}
	return pod
}

func instanceSetRoleLabels(name, role string) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  name,
		constant.RoleLabelKey:                  role,
	}
}

func podWithAnnotations(annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Annotations: annotations},
	}
}

func setControllerRef(obj client.Object, apiVersion, kind, name string) {
	controller := true
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        types.UID(name + "-uid"),
		Controller: &controller,
	}})
}

func roleProbeEvent(namespace, name string, pod *corev1.Pod, role string, eventTime time.Time) *corev1.Event {
	return roleProbeEventWithCode(namespace, name, pod, role, eventTime, 0, "")
}

func roleProbeEventWithOutput(namespace, name string, pod *corev1.Pod, output string, eventTime time.Time) *corev1.Event {
	message, err := json.Marshal(proto.ProbeEvent{
		Probe:  "roleProbe",
		Code:   0,
		Output: []byte(output),
	})
	if err != nil {
		panic(err)
	}
	return builder.NewEventBuilder(namespace, name).
		SetInvolvedObject(corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			UID:        pod.UID,
			FieldPath:  proto.ProbeEventFieldPath,
		}).
		SetReason("roleProbe").
		SetMessage(string(message)).
		SetReportingController(proto.ProbeEventReportingController).
		SetEventTime(metav1.NewMicroTime(eventTime)).
		GetObject()
}

func roleProbeEventWithCode(namespace, name string, pod *corev1.Pod, role string, eventTime time.Time, code int32, messageText string) *corev1.Event {
	message, err := json.Marshal(proto.ProbeEvent{
		Probe:   "roleProbe",
		Code:    code,
		Output:  []byte(role),
		Message: messageText,
	})
	if err != nil {
		panic(err)
	}
	return builder.NewEventBuilder(namespace, name).
		SetInvolvedObject(corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			UID:        pod.UID,
			FieldPath:  proto.ProbeEventFieldPath,
		}).
		SetReason("roleProbe").
		SetMessage(string(message)).
		SetReportingController(proto.ProbeEventReportingController).
		SetEventTime(metav1.NewMicroTime(eventTime)).
		GetObject()
}

func versionedRoleProbeOutput(role string, version uint64) roleProbeOutput {
	return roleProbeOutput{
		role:                    role,
		authoritativeVersion:    version,
		hasAuthoritativeVersion: true,
	}
}

func assertPodRole(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, role, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if role == "" {
		if stored.Labels[constant.RoleLabelKey] != "" {
			t.Fatalf("expected role label to be empty, got %q", stored.Labels[constant.RoleLabelKey])
		}
	} else if stored.Labels[constant.RoleLabelKey] != role {
		t.Fatalf("expected role %q, got %q", role, stored.Labels[constant.RoleLabelKey])
	}
	if version != "" && stored.Annotations[constant.LastRoleEventVersionAnnotationKey] != version {
		t.Fatalf("expected single-token version %q, got %q", version, stored.Annotations[constant.LastRoleEventVersionAnnotationKey])
	}
}

func assertPodLastRoleVersion(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if stored.Annotations[constant.LastRoleEventVersionAnnotationKey] != version {
		t.Fatalf("expected single-token version %q, got %q", version, stored.Annotations[constant.LastRoleEventVersionAnnotationKey])
	}
}

func assertPodLastRoleAuthoritativeVersion(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if stored.Annotations[constant.LastRoleAuthoritativeVersionAnnotationKey] != version {
		t.Fatalf("expected authoritative role version %q, got %q", version, stored.Annotations[constant.LastRoleAuthoritativeVersionAnnotationKey])
	}
}
