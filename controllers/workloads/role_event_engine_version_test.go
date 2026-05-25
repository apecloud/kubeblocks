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
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
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

// --- gate tests: 4 mixed-format cases ---

func TestCheckEngineVersionStaleEngineRejectsOlderVersion(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", version: 9, mode: roleProbeVersionModeEngine},
		"engine:10", 0,
	)
	if decision != roleProbeGateRejectStale || newAnnotation != "engine:10" {
		t.Fatalf("decision=%d newAnnotation=%q, want RejectStale + engine:10 (unchanged)", decision, newAnnotation)
	}
}

func TestCheckEngineVersionStaleEngineAcceptsNewerVersion(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", version: 11, mode: roleProbeVersionModeEngine},
		"engine:10", 0,
	)
	if decision != roleProbeGateAccept || newAnnotation != "engine:11" {
		t.Fatalf("decision=%d newAnnotation=%q, want Accept + engine:11", decision, newAnnotation)
	}
}

func TestCheckEngineVersionStaleEngineUpgradesFromLegacyAnnotation(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", version: 11, mode: roleProbeVersionModeEngine},
		"1779550000000000", 1779550600000000,
	)
	if decision != roleProbeGateAccept || newAnnotation != "engine:11" {
		t.Fatalf("decision=%d newAnnotation=%q, want Accept + engine:11 (upgrade)", decision, newAnnotation)
	}
}

func TestCheckEngineVersionStaleLegacyEventRejectedAgainstEngineAnnotation(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone},
		"engine:10", 1779550600000000,
	)
	if decision != roleProbeGateRejectStale || newAnnotation != "engine:10" {
		t.Fatalf("decision=%d newAnnotation=%q, want RejectStale + engine:10 (no downgrade)", decision, newAnnotation)
	}
}

func TestCheckEngineVersionStaleLegacyEventAdvancesOnNewerEventTime(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone},
		"1779550000000000", 1779550600000000,
	)
	if decision != roleProbeGateAccept || newAnnotation != "1779550600000000" {
		t.Fatalf("decision=%d newAnnotation=%q, want Accept + 1779550600000000", decision, newAnnotation)
	}
}

func TestCheckEngineVersionStaleLegacyEventRejectsOlderEventTime(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone},
		"1779550600000000", 1779550000000000,
	)
	if decision != roleProbeGateRejectStale || newAnnotation != "1779550600000000" {
		t.Fatalf("decision=%d newAnnotation=%q, want RejectStale + 1779550600000000", decision, newAnnotation)
	}
}

func TestCheckEngineVersionStaleMalformedRejects(t *testing.T) {
	decision, newAnnotation := checkEngineVersionStale(
		roleProbeOutput{role: "primary", mode: roleProbeVersionModeMalformed},
		"engine:10", 1779550600000000,
	)
	if decision != roleProbeGateRejectMalformed || newAnnotation != "engine:10" {
		t.Fatalf("decision=%d newAnnotation=%q, want RejectMalformed + engine:10 (unchanged)", decision, newAnnotation)
	}
}

// --- exclusive peer cleanup tests (fake client) ---

func newRoleEventScheme(t *testing.T) *k8sruntime.Scheme {
	scheme := k8sruntime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := workloadsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}
	return scheme
}

func newRoleEventInstanceSet(ns, name, exclusiveRole string) *workloadsv1.InstanceSet {
	return &workloadsv1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: workloadsv1.InstanceSetSpec{
			Roles: []workloadsv1.ReplicaRole{
				{Name: exclusiveRole, ParticipatesInQuorum: true, UpdatePriority: 5, IsExclusive: true},
				{Name: "secondary", ParticipatesInQuorum: true, UpdatePriority: 3},
			},
		},
	}
}

func newRoleEventPod(ns, name, itsName, role, annotation string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:          constant.AppName,
				instanceset.WorkloadsManagedByLabelKey: workloadsv1.InstanceSetKind,
				instanceset.WorkloadsInstanceLabelKey:  itsName,
			},
		},
	}
	if role != "" {
		pod.Labels[constant.RoleLabelKey] = role
	}
	if annotation != "" {
		pod.Annotations = map[string]string{
			constant.LastRoleEventVersionAnnotationKey: annotation,
		}
	}
	return pod
}

// Legacy event whose EventTime is strictly newer than a peer's bare-EventTime
// annotation must strip the peer's exclusive role label but leave the peer's
// own LastRoleEventVersionAnnotationKey untouched. The annotation belongs to
// the peer's own kbagent stream; cleanup must not advance it on the peer's
// behalf or the peer's next legitimate event at the same epoch would be
// rejected as stale.
func TestRemoveExclusiveRoleLabelsLegacyStripsLabelOfOlderPeerWithoutAdvancingAnnotation(t *testing.T) {
	scheme := newRoleEventScheme(t)
	const (
		ns        = "default"
		itsName   = "redis-0"
		newPod    = "redis-0-0"
		peer      = "redis-0-1"
		role      = "primary"
		newMicros = int64(1779550700000000)
		peerOlder = "1779550500000000"
	)
	its := newRoleEventInstanceSet(ns, itsName, role)
	peerPod := newRoleEventPod(ns, peer, itsName, role, peerOlder)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, peerPod).Build()

	parsed := roleProbeOutput{role: role, mode: roleProbeVersionModeNone}
	if err := removeExclusiveRoleLabels(context.Background(), cli, *its, newPod, role, parsed, newMicros); err != nil {
		t.Fatalf("removeExclusiveRoleLabels err = %v", err)
	}
	got := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peer}, got); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if _, ok := got.Labels[constant.RoleLabelKey]; ok {
		t.Fatalf("peer role label still present after legacy cleanup: %v", got.Labels)
	}
	if v := got.Annotations[constant.LastRoleEventVersionAnnotationKey]; v != peerOlder {
		t.Fatalf("peer annotation = %q, want %q (unchanged)", v, peerOlder)
	}
	_ = logr.Discard()
}

// Engine event with newer version must strip the older-version peer label,
// without stamping the peer's annotation. The same contract applies as the
// legacy case.
func TestRemoveExclusiveRoleLabelsEngineStripsLabelOfOlderPeerWithoutAdvancingAnnotation(t *testing.T) {
	scheme := newRoleEventScheme(t)
	const (
		ns         = "default"
		itsName    = "redis-0"
		newPod     = "redis-0-0"
		peer       = "redis-0-1"
		role       = "primary"
		newVersion = uint64(20)
		peerOlder  = "engine:10"
	)
	its := newRoleEventInstanceSet(ns, itsName, role)
	peerPod := newRoleEventPod(ns, peer, itsName, role, peerOlder)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, peerPod).Build()

	parsed := roleProbeOutput{role: role, version: newVersion, mode: roleProbeVersionModeEngine}
	if err := removeExclusiveRoleLabels(context.Background(), cli, *its, newPod, role, parsed, 0); err != nil {
		t.Fatalf("removeExclusiveRoleLabels err = %v", err)
	}
	got := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peer}, got); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if _, ok := got.Labels[constant.RoleLabelKey]; ok {
		t.Fatalf("peer role label still present after engine cleanup: %v", got.Labels)
	}
	if v := got.Annotations[constant.LastRoleEventVersionAnnotationKey]; v != peerOlder {
		t.Fatalf("peer annotation = %q, want %q (unchanged)", v, peerOlder)
	}
}

// Engine event with strictly older version must not touch a peer whose
// annotation is already at a newer engine version.
func TestRemoveExclusiveRoleLabelsEngineDoesNotStripNewerPeer(t *testing.T) {
	scheme := newRoleEventScheme(t)
	const (
		ns         = "default"
		itsName    = "redis-0"
		newPod     = "redis-0-0"
		peer       = "redis-0-1"
		role       = "primary"
		newVersion = uint64(10)
		peerAnnot  = "engine:20"
	)
	its := newRoleEventInstanceSet(ns, itsName, role)
	peerPod := newRoleEventPod(ns, peer, itsName, role, peerAnnot)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, peerPod).Build()

	parsed := roleProbeOutput{role: role, version: newVersion, mode: roleProbeVersionModeEngine}
	if err := removeExclusiveRoleLabels(context.Background(), cli, *its, newPod, role, parsed, 0); err != nil {
		t.Fatalf("removeExclusiveRoleLabels err = %v", err)
	}
	got := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peer}, got); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if v, ok := got.Labels[constant.RoleLabelKey]; !ok || v != role {
		t.Fatalf("peer role label was stripped: %v", got.Labels)
	}
	if v := got.Annotations[constant.LastRoleEventVersionAnnotationKey]; v != peerAnnot {
		t.Fatalf("peer annotation = %q, want %q (unchanged)", v, peerAnnot)
	}
}

// Regression for V1 round 1 of the prior PR: after cleanup strips the old
// primary's label, the old primary's own next roleProbe event at the same
// engine epoch (e.g. "secondary 1" arriving right after a new primary was
// claimed at version 1) must be accepted by the gate, because cleanup did
// not silently advance the peer's annotation.
func TestRemoveExclusiveRoleLabelsThenDemotedPeerAcceptsItsOwnEvent(t *testing.T) {
	scheme := newRoleEventScheme(t)
	const (
		ns         = "default"
		itsName    = "redis-0"
		newPod     = "redis-0-0"
		peer       = "redis-0-1"
		role       = "primary"
		newVersion = uint64(1)
		peerAnnot  = "engine:0" // peer's previous accepted event at an earlier epoch
	)
	its := newRoleEventInstanceSet(ns, itsName, role)
	peerPod := newRoleEventPod(ns, peer, itsName, role, peerAnnot)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, peerPod).Build()

	parsed := roleProbeOutput{role: role, version: newVersion, mode: roleProbeVersionModeEngine}
	if err := removeExclusiveRoleLabels(context.Background(), cli, *its, newPod, role, parsed, 0); err != nil {
		t.Fatalf("removeExclusiveRoleLabels err = %v", err)
	}
	peerCurrent := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peer}, peerCurrent); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	peerLast := peerCurrent.Annotations[constant.LastRoleEventVersionAnnotationKey]
	if peerLast != peerAnnot {
		t.Fatalf("precondition: peer annotation must stay unchanged after cleanup; got %q want %q", peerLast, peerAnnot)
	}

	// Simulate the peer's own kbagent emitting its post-failover event
	// "secondary 1" — same engine epoch as the cleanup-driving event, but
	// from the peer's own roleProbe stream. The strict-newer gate must
	// accept it because the peer's annotation was not advanced by cleanup.
	peerParsed := roleProbeOutput{role: "secondary", version: newVersion, mode: roleProbeVersionModeEngine}
	decision, peerNewAnnotation := checkEngineVersionStale(peerParsed, peerLast, 0)
	if decision != roleProbeGateAccept {
		t.Fatalf("peer's own post-failover secondary event must be accepted; decision = %d, want Accept", decision)
	}
	if peerNewAnnotation != fmt.Sprintf("engine:%d", newVersion) {
		t.Fatalf("peer's post-failover annotation should advance to engine:%d, got %q", newVersion, peerNewAnnotation)
	}
}
