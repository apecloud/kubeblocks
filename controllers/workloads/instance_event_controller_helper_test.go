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

import "testing"

func TestParseRoleProbeOutputLegacyOnly(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary"))
	if out.role != "primary" {
		t.Fatalf("role = %q, want %q", out.role, "primary")
	}
	if out.mode != roleProbeVersionModeNone {
		t.Fatalf("mode = %d, want None", out.mode)
	}
	if out.version != 0 {
		t.Fatalf("version = %d, want 0", out.version)
	}
}

func TestParseRoleProbeOutputEngineVersion(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary\nkb-role-version=10"))
	if out.role != "primary" {
		t.Fatalf("role = %q, want %q", out.role, "primary")
	}
	if out.mode != roleProbeVersionModeEngine {
		t.Fatalf("mode = %d, want Engine", out.mode)
	}
	if out.version != 10 {
		t.Fatalf("version = %d, want 10", out.version)
	}
}

func TestParseRoleProbeOutputEngineVersionWithExtraTrailerLine(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary\nfoo=bar\nkb-role-version=42"))
	if out.role != "primary" {
		t.Fatalf("role = %q, want %q", out.role, "primary")
	}
	if out.mode != roleProbeVersionModeEngine {
		t.Fatalf("mode = %d, want Engine", out.mode)
	}
	if out.version != 42 {
		t.Fatalf("version = %d, want 42", out.version)
	}
}

func TestParseRoleProbeOutputMalformedEmptyVersion(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary\nkb-role-version="))
	if out.mode != roleProbeVersionModeMalformed {
		t.Fatalf("mode = %d, want Malformed", out.mode)
	}
}

func TestParseRoleProbeOutputMalformedNonUint64(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary\nkb-role-version=abc"))
	if out.mode != roleProbeVersionModeMalformed {
		t.Fatalf("mode = %d, want Malformed", out.mode)
	}
}

func TestParseRoleProbeOutputMalformedDuplicateVersion(t *testing.T) {
	out := parseRoleProbeOutput([]byte("primary\nkb-role-version=10\nkb-role-version=20"))
	if out.mode != roleProbeVersionModeMalformed {
		t.Fatalf("mode = %d, want Malformed", out.mode)
	}
}

// RED-1: engine event with strictly older version vs already-recorded engine
// annotation must be rejected as stale.
func TestCheckRoleProbeStaleEngineRejectsOlderVersion(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", version: 9, mode: roleProbeVersionModeEngine}
	decision, newAnnotation := checkRoleProbeStale(parsed, "engine:10", 0)
	if decision != roleProbeGateRejectStale {
		t.Fatalf("decision = %d, want RejectStale", decision)
	}
	if newAnnotation != "engine:10" {
		t.Fatalf("newAnnotation = %q, want %q (unchanged)", newAnnotation, "engine:10")
	}
}

// RED-2: engine event with strictly newer version vs recorded engine
// annotation must be accepted and advance the annotation.
func TestCheckRoleProbeStaleEngineAcceptsNewerVersion(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", version: 11, mode: roleProbeVersionModeEngine}
	decision, newAnnotation := checkRoleProbeStale(parsed, "engine:10", 0)
	if decision != roleProbeGateAccept {
		t.Fatalf("decision = %d, want Accept", decision)
	}
	if newAnnotation != "engine:11" {
		t.Fatalf("newAnnotation = %q, want %q", newAnnotation, "engine:11")
	}
}

// RED-3: engine event arriving when the Pod still has a bare EventTime
// annotation from an earlier legacy controller / addon must accept and
// upgrade the annotation to engine:<n>.
func TestCheckRoleProbeStaleEngineUpgradesFromLegacyAnnotation(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", version: 11, mode: roleProbeVersionModeEngine}
	decision, newAnnotation := checkRoleProbeStale(parsed, "1779550000000000", 1779550600000000)
	if decision != roleProbeGateAccept {
		t.Fatalf("decision = %d, want Accept", decision)
	}
	if newAnnotation != "engine:11" {
		t.Fatalf("newAnnotation = %q, want %q", newAnnotation, "engine:11")
	}
}

// RED-4: a kb-role-version trailer that does not parse must reject the event
// without falling back to the EventTime path. The annotation must not change.
func TestCheckRoleProbeStaleMalformedRejects(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", version: 0, mode: roleProbeVersionModeMalformed}
	decision, newAnnotation := checkRoleProbeStale(parsed, "engine:10", 1779550600000000)
	if decision != roleProbeGateRejectMalformed {
		t.Fatalf("decision = %d, want RejectMalformed", decision)
	}
	if newAnnotation != "engine:10" {
		t.Fatalf("newAnnotation = %q, want %q (unchanged)", newAnnotation, "engine:10")
	}
}

// Legacy event arriving at a Pod whose annotation is already engine-format
// must be rejected: the engine-version gate cannot be silently downgraded
// back to EventTime by an unaware emitter.
func TestCheckRoleProbeStaleLegacyEventRejectedAgainstEngineAnnotation(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	decision, newAnnotation := checkRoleProbeStale(parsed, "engine:10", 1779550600000000)
	if decision != roleProbeGateRejectStale {
		t.Fatalf("decision = %d, want RejectStale", decision)
	}
	if newAnnotation != "engine:10" {
		t.Fatalf("newAnnotation = %q, want %q (unchanged)", newAnnotation, "engine:10")
	}
}

// Legacy event vs legacy annotation must keep the existing lexical EventTime
// behaviour: strictly newer micros advances, older or equal rejects.
func TestCheckRoleProbeStaleLegacyEventAdvancesOnNewerEventTime(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	decision, newAnnotation := checkRoleProbeStale(parsed, "1779550000000000", 1779550600000000)
	if decision != roleProbeGateAccept {
		t.Fatalf("decision = %d, want Accept", decision)
	}
	if newAnnotation != "1779550600000000" {
		t.Fatalf("newAnnotation = %q, want %q", newAnnotation, "1779550600000000")
	}
}

func TestCheckRoleProbeStaleLegacyEventRejectsOnOlderEventTime(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", mode: roleProbeVersionModeNone}
	decision, newAnnotation := checkRoleProbeStale(parsed, "1779550600000000", 1779550000000000)
	if decision != roleProbeGateRejectStale {
		t.Fatalf("decision = %d, want RejectStale", decision)
	}
	if newAnnotation != "1779550600000000" {
		t.Fatalf("newAnnotation = %q, want %q (unchanged)", newAnnotation, "1779550600000000")
	}
}

// Peer pod cleanup must honour the same engine-version gate so a stale primary
// event cannot strip the label from a peer that has already advanced past it.
func TestCheckRoleProbeStaleRejectsPeerCleanupWhenPeerHasNewerEngineVersion(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", version: 10, mode: roleProbeVersionModeEngine}
	decision, newAnnotation := checkRoleProbeStale(parsed, "engine:15", 0)
	if decision != roleProbeGateRejectStale {
		t.Fatalf("decision = %d, want RejectStale", decision)
	}
	if newAnnotation != "engine:15" {
		t.Fatalf("newAnnotation = %q, want %q (unchanged)", newAnnotation, "engine:15")
	}
}

func TestCheckRoleProbeStaleAcceptsPeerCleanupWhenPeerHasOlderEngineVersion(t *testing.T) {
	parsed := roleProbeOutput{role: "primary", version: 20, mode: roleProbeVersionModeEngine}
	decision, newAnnotation := checkRoleProbeStale(parsed, "engine:15", 0)
	if decision != roleProbeGateAccept {
		t.Fatalf("decision = %d, want Accept", decision)
	}
	if newAnnotation != "engine:20" {
		t.Fatalf("newAnnotation = %q, want %q", newAnnotation, "engine:20")
	}
}
