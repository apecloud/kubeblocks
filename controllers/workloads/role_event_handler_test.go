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

func TestParseRoleProbeOutputLegacySingleToken(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary"))
	if out.role != "primary" || out.mode != roleProbeVersionModeNone || out.version != 0 {
		t.Fatalf("got %+v, want role=primary mode=None version=0", out)
	}
}

func TestParseRoleProbeOutputEngineSpaceSeparated(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary 10"))
	if out.role != "primary" || out.mode != roleProbeVersionModeEngine || out.version != 10 {
		t.Fatalf("got %+v, want role=primary mode=Engine version=10", out)
	}
}

func TestParseRoleProbeOutputEngineNewlineSeparated(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary\n10"))
	if out.role != "primary" || out.mode != roleProbeVersionModeEngine || out.version != 10 {
		t.Fatalf("got %+v, want role=primary mode=Engine version=10", out)
	}
}

func TestParseRoleProbeOutputEngineTolerantOfSurroundingWhitespace(t *testing.T) {
	out := parseRoleProbeOutput([]byte("\tprimary\t42\n"))
	if out.role != "primary" || out.mode != roleProbeVersionModeEngine || out.version != 42 {
		t.Fatalf("got %+v, want role=primary mode=Engine version=42", out)
	}
}

func TestParseRoleProbeOutputMalformedSecondTokenNotUint64(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary abc"))
	if out.mode != roleProbeVersionModeMalformed {
		t.Fatalf("got %+v, want mode=Malformed", out)
	}
}

func TestParseRoleProbeOutputMalformedThreeOrMoreTokens(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary 10 extra"))
	if out.mode != roleProbeVersionModeMalformed {
		t.Fatalf("got %+v, want mode=Malformed", out)
	}
}

func TestParseRoleProbeOutputEmpty(t *testing.T) {
	out := parseRoleProbeOutput([]byte(""))
	if out.mode != roleProbeVersionModeNone || out.role != "" {
		t.Fatalf("got %+v, want mode=None role=\"\"", out)
	}
}

// --- gate tests: each path consults only its own annotation key ---

func TestGateEngineRejectsOlderVersion(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEngineVersionAnnotationKey: "10"})
	parsed := roleProbeOutput{role: "primary", version: 9, mode: roleProbeVersionModeEngine}
	if got := gateRoleProbeEvent(parsed, pod, 0); got != roleProbeGateRejectStale {
		t.Fatalf("got %d, want RejectStale", got)
	}
}

func TestGateEngineRejectsEqualVersion(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEngineVersionAnnotationKey: "10"})
	parsed := roleProbeOutput{role: "primary", version: 10, mode: roleProbeVersionModeEngine}
	if got := gateRoleProbeEvent(parsed, pod, 0); got != roleProbeGateRejectStale {
		t.Fatalf("got %d, want RejectStale (strict-newer)", got)
	}
}

func TestGateEngineAcceptsNewerVersion(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEngineVersionAnnotationKey: "10"})
	parsed := roleProbeOutput{role: "primary", version: 11, mode: roleProbeVersionModeEngine}
	if got := gateRoleProbeEvent(parsed, pod, 0); got != roleProbeGateAccept {
		t.Fatalf("got %d, want Accept", got)
	}
}

// Engine events do not consult the legacy annotation key — an engine event
// on a pod that has only a legacy annotation must be accepted regardless of
// EventTime.
func TestGateEngineIgnoresLegacyAnnotation(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550000000000"})
	parsed := roleProbeOutput{role: "primary", version: 1, mode: roleProbeVersionModeEngine}
	if got := gateRoleProbeEvent(parsed, pod, 1); got != roleProbeGateAccept {
		t.Fatalf("got %d, want Accept (engine key empty, legacy key ignored)", got)
	}
}

func TestGateLegacyAcceptsNewerEventTime(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550000000000"})
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	if got := gateRoleProbeEvent(parsed, pod, 1779550600000000); got != roleProbeGateAccept {
		t.Fatalf("got %d, want Accept", got)
	}
}

func TestGateLegacyRejectsOlderEventTime(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEventVersionAnnotationKey: "1779550600000000"})
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	if got := gateRoleProbeEvent(parsed, pod, 1779550000000000); got != roleProbeGateRejectStale {
		t.Fatalf("got %d, want RejectStale", got)
	}
}

// A Pod that has accepted any engine-versioned event commits to engine
// mode: subsequent legacy single-token events from the same Pod are
// rejected. Without this guard a legacy event could overwrite an
// engine-accepted role label and then the next same-version engine
// event would be rejected by the strict-newer gate, leaving the role
// label unrecoverable on that Pod.
func TestGateLegacyRejectedOnPodAlreadyAcceptedEngineEvent(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEngineVersionAnnotationKey: "10"})
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	if got := gateRoleProbeEvent(parsed, pod, 1779550600000000); got != roleProbeGateRejectStale {
		t.Fatalf("got %d, want RejectStale (same-Pod legacy must not overwrite accepted engine state)", got)
	}
}

// The same-Pod commit-to-engine rule also blocks legacy events even if
// the Pod has accumulated a legacy annotation alongside the engine
// annotation (mixed history from a misbehaving probe script).
func TestGateLegacyRejectedOnPodWithBothAnnotationsWhenEnginePresent(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "10",
		constant.LastRoleEventVersionAnnotationKey:  "1000000",
	})
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	if got := gateRoleProbeEvent(parsed, pod, 2000000); got != roleProbeGateRejectStale {
		t.Fatalf("got %d, want RejectStale", got)
	}
}

func TestGateMalformedRejects(t *testing.T) {
	pod := podWithAnnotations(map[string]string{constant.LastRoleEngineVersionAnnotationKey: "10"})
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeMalformed}
	if got := gateRoleProbeEvent(parsed, pod, 1779550600000000); got != roleProbeGateRejectMalformed {
		t.Fatalf("got %d, want RejectMalformed", got)
	}
}

// --- end-to-end handler tests via fake client ---

func TestRoleEventHandlerHandlesInstanceSetLegacyAndExclusiveCleanupStampsPeerAnnotation(t *testing.T) {
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

	wantLegacy := fmt.Sprintf("%d", event.EventTime.UnixMicro())
	assertPodRole(t, ctx, cli, pod, "leader", wantLegacy)
	// Peer's exclusive role label is stripped. Legacy cleanup also stamps
	// the peer's LastRoleEventVersionAnnotationKey with the cleanup event's
	// EventTime: without this stamp a delayed legacy event from the demoted
	// primary whose EventTime is older than the cleanup but newer than the
	// peer's own previous annotation would slip back through the gate.
	assertPodRole(t, ctx, cli, otherPod, "", wantLegacy)
	assertPodLastEngineVersion(t, ctx, cli, otherPod, "")
}

// Engine path peer cleanup must strip the label but leave the peer's
// LastRoleEngineVersionAnnotationKey untouched. Stamping it would let the
// strict-newer gate later reject a legitimate event from the peer at the
// same engine epoch (e.g. demoted pod emitting `secondary <same-epoch>`).
func TestRoleEventHandlerHandlesInstanceSetEngineAndExclusiveCleanupDoesNotStampPeerEngineAnnotation(t *testing.T) {
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
		constant.LastRoleEngineVersionAnnotationKey: "0",
	}
	event := roleProbeEventWithOutput("default", "event-1", pod, "leader 1", now)
	cli := roleEventFakeClient(t, its, pod, otherPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastEngineVersion(t, ctx, cli, pod, "1")
	assertPodRole(t, ctx, cli, otherPod, "", "")
	// Critical contract: peer engine annotation is NOT advanced; otherwise
	// the peer's own next event at engine version 1 would be rejected as
	// stale by the strict-newer gate.
	assertPodLastEngineVersion(t, ctx, cli, otherPod, "0")
}

// Regression for Valkey r4 mixed-mode bug on PR #10283 head 714f684b: a
// legacy `primary` event from a non-quorum probe script fallback path must not
// strip the exclusive role label off an engine-versioned peer. Without
// this guard the legacy event runs exclusive cleanup against the
// engine-held primary (the gate consults only the legacy annotation,
// which on the engine peer is empty, so cleanup is accepted); after the
// label is stripped the engine peer's next same-version event is
// rejected by the strict-newer gate and the role label can never be
// restored.
func TestRoleEventHandlerLegacyExclusiveEventBlockedByEngineHoldingPeer(t *testing.T) {
	ctx := context.Background()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	enginePeer := roleEventPod("default", "vlk-3", "uid-3", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	enginePeer.Annotations = map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "3",
	}
	legacyPod := roleEventPod("default", "vlk-2", "uid-2", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	event := roleProbeEvent("default", "event-1", legacyPod, "leader", time.Now())
	cli := roleEventFakeClient(t, its, enginePeer, legacyPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled (skipped + reason=engineHeldExclusiveRole)")
	}

	// legacy pod must not have received the leader label; engine peer's
	// label and annotations must be unchanged.
	assertPodRole(t, ctx, cli, legacyPod, "", "")
	assertPodLastRoleVersion(t, ctx, cli, legacyPod, "")
	assertPodRole(t, ctx, cli, enginePeer, "leader", "")
	assertPodLastEngineVersion(t, ctx, cli, enginePeer, "3")
	assertPodLastRoleVersion(t, ctx, cli, enginePeer, "")
}

// When the role being claimed is non-exclusive, the engine-held-peer
// guard does not apply: a legacy event for a non-exclusive role still
// goes through normally even when another peer carries an engine
// annotation.
func TestRoleEventHandlerLegacyNonExclusiveEventNotBlockedByEnginePeer(t *testing.T) {
	ctx := context.Background()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	enginePeer := roleEventPod("default", "vlk-3", "uid-3", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	enginePeer.Annotations = map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "3",
	}
	legacyPod := roleEventPod("default", "vlk-2", "uid-2", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	event := roleProbeEvent("default", "event-1", legacyPod, "follower", time.Now())
	cli := roleEventFakeClient(t, its, enginePeer, legacyPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	wantLegacy := fmt.Sprintf("%d", event.EventTime.UnixMicro())
	assertPodRole(t, ctx, cli, legacyPod, "follower", wantLegacy)
	// Engine peer untouched (cleanup only runs for exclusive roles).
	assertPodRole(t, ctx, cli, enginePeer, "leader", "")
	assertPodLastEngineVersion(t, ctx, cli, enginePeer, "3")
}

// A legacy exclusive event still runs normally when no peer holds the
// exclusive role with an engine annotation. This pins that the guard
// only fires in mixed-mode coexistence.
func TestRoleEventHandlerLegacyExclusiveEventAcceptedWhenNoEnginePeerHoldsRole(t *testing.T) {
	ctx := context.Background()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	// peer holds leader but only via legacy annotation, no engine annotation
	legacyPeer := roleEventPod("default", "vlk-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	legacyPeer.Annotations = map[string]string{
		constant.LastRoleEventVersionAnnotationKey: "1000000",
	}
	legacyPod := roleEventPod("default", "vlk-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	now := time.Unix(0, 2000000*int64(time.Microsecond))
	event := roleProbeEvent("default", "event-1", legacyPod, "leader", now)
	cli := roleEventFakeClient(t, its, legacyPeer, legacyPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	wantLegacy := fmt.Sprintf("%d", event.EventTime.UnixMicro())
	assertPodRole(t, ctx, cli, legacyPod, "leader", wantLegacy)
	// legacy peer stripped + legacy annotation stamped per the
	// legacy-path one-way ratchet contract.
	assertPodRole(t, ctx, cli, legacyPeer, "", wantLegacy)
}

func TestRoleEventHandlerEngineExclusiveEventBlockedByPeerWithNewerEngineVersion(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	enginePeer := roleEventPod("default", "vlk-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	enginePeer.Annotations = map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "5",
	}
	enginePod := roleEventPod("default", "vlk-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	enginePod.Annotations = map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "3",
	}
	event := roleProbeEventWithOutput("default", "event-1", enginePod, "leader 4", now)
	cli := roleEventFakeClient(t, its, enginePeer, enginePod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled (skipped + reason=stalePeerRoleEventVersion)")
	}

	assertPodRole(t, ctx, cli, enginePod, "", "")
	assertPodLastEngineVersion(t, ctx, cli, enginePod, "3")
	assertPodRole(t, ctx, cli, enginePeer, "leader", "")
	assertPodLastEngineVersion(t, ctx, cli, enginePeer, "5")
}

// Engine events for an exclusive role are not affected by the
// legacy-only engine-held-peer guard: an engine event from one pod is
// allowed to take the role from an older engine peer (and cleanup will
// strip the peer label as usual, gated by the engine-version comparison).
func TestRoleEventHandlerEngineExclusiveEventNotBlockedByEnginePeerGuard(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "vlk"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	enginePeer := roleEventPod("default", "vlk-2", "uid-2", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "vlk",
		constant.RoleLabelKey:                  "leader",
	})
	enginePeer.Annotations = map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "3",
	}
	enginePod := roleEventPod("default", "vlk-3", "uid-3", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "vlk",
	})
	event := roleProbeEventWithOutput("default", "event-1", enginePod, "leader 5", now)
	cli := roleEventFakeClient(t, its, enginePeer, enginePod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, enginePod, "leader", "")
	assertPodLastEngineVersion(t, ctx, cli, enginePod, "5")
	// Peer label stripped by engine-path cleanup (engine version 5 > peer's 3).
	assertPodRole(t, ctx, cli, enginePeer, "", "")
	// Peer engine annotation untouched per the V1 fix.
	assertPodLastEngineVersion(t, ctx, cli, enginePeer, "3")
}

// Regression for the prior V1 round: after engine-path cleanup strips the
// old primary's label, the old primary's own next roleProbe event at the
// same engine epoch must be accepted. With per-path keys this is enforced
// directly by gateRoleProbeEvent: the peer's annotation stays at the older
// version so a same-epoch secondary event from the peer is strictly newer.
func TestRoleEventEngineCleanupLeavesPeerAbleToAcceptItsOwnSameEpochSecondaryEvent(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		constant.LastRoleEngineVersionAnnotationKey: "0",
	})
	parsed := roleProbeOutput{role: "secondary", version: 1, mode: roleProbeVersionModeEngine}
	if got := gateRoleProbeEvent(parsed, pod, 0); got != roleProbeGateAccept {
		t.Fatalf("got %d, want Accept (peer engine annotation must not have been advanced by cleanup)", got)
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
	assertPodLastEngineVersion(t, ctx, cli, pod, "")
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
	assertPodLastEngineVersion(t, ctx, cli, pod, "")
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
	// the malformed output is rejected by the gate before any write.
	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
	assertPodLastEngineVersion(t, ctx, cli, pod, "")
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
	assertPodLastEngineVersion(t, ctx, cli, pod, "")
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
			assertPodLastEngineVersion(t, ctx, cli, pod, "")
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
		t.Fatalf("expected legacy version %q, got %q", version, stored.Annotations[constant.LastRoleEventVersionAnnotationKey])
	}
}

func assertPodLastRoleVersion(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if stored.Annotations[constant.LastRoleEventVersionAnnotationKey] != version {
		t.Fatalf("expected legacy version %q, got %q", version, stored.Annotations[constant.LastRoleEventVersionAnnotationKey])
	}
}

func assertPodLastEngineVersion(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if stored.Annotations[constant.LastRoleEngineVersionAnnotationKey] != version {
		t.Fatalf("expected engine version %q, got %q", version, stored.Annotations[constant.LastRoleEngineVersionAnnotationKey])
	}
}
