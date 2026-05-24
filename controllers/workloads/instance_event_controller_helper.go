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
	"fmt"
	"strconv"
	"strings"
)

// roleProbeVersionMode classifies how a roleProbe stdout payload encodes its
// engine-authoritative role version.
type roleProbeVersionMode int

const (
	// roleProbeVersionModeNone means the stdout carries only the role name on
	// its first line, with no kb-role-version trailer. This is the legacy
	// addon contract; the staleness gate falls back to EventTime.
	roleProbeVersionModeNone roleProbeVersionMode = iota

	// roleProbeVersionModeEngine means the stdout carries exactly one
	// `kb-role-version=<uint64>` trailer line and the value parses cleanly.
	// The staleness gate uses the engine-authoritative version.
	roleProbeVersionModeEngine

	// roleProbeVersionModeMalformed means the stdout carries at least one
	// `kb-role-version=` line but the value is empty, not a uint64, or
	// duplicated. The event must be rejected; falling back to EventTime would
	// let a single typo silently bypass the new staleness gate.
	roleProbeVersionModeMalformed
)

// roleProbeVersionLinePrefix marks the trailer line that carries the
// engine-authoritative role version. The prefix is intentionally narrow
// (instead of a bare `version=`) so it does not collide with addon-specific
// stdout that already uses `version=...` as part of business output.
const roleProbeVersionLinePrefix = "kb-role-version="

// engineVersionAnnotationPrefix distinguishes the engine-authoritative role
// version recorded on the Pod annotation from the legacy bare EventTime
// micros that older controllers used to record. Mixed-format handling lives
// in checkRoleProbeStale.
const engineVersionAnnotationPrefix = "engine:"

// roleProbeOutput is the parsed view of a kbagent roleProbe stdout payload.
type roleProbeOutput struct {
	role    string
	version uint64
	mode    roleProbeVersionMode
}

// parseRoleProbeOutput parses the kbagent roleProbe stdout into a role name
// plus an optional engine-authoritative version. The grammar is:
//
//	<role>\n[<other-trailer-lines>\n][<role-version-line>\n]...
//
// where <role-version-line> is exactly `kb-role-version=<uint64>`. The first
// line is always treated as the role name to preserve compatibility with the
// pre-existing single-line contract; legacy addons that emit only a role name
// still parse with mode=None and fall back to the EventTime staleness path.
//
// Strictness for the version trailer is deliberate: addons that try to use
// the new contract but emit a broken value (empty, non-uint64, or repeated)
// are flagged Malformed so the controller can reject the event rather than
// silently fall back. A silent fall back would let a typo or accidental
// double-line bypass the staleness gate the addon meant to install.
func parseRoleProbeOutput(stdout []byte) roleProbeOutput {
	if len(stdout) == 0 {
		return roleProbeOutput{mode: roleProbeVersionModeNone}
	}
	lines := strings.Split(string(stdout), "\n")
	out := roleProbeOutput{
		role: strings.ToLower(strings.TrimSpace(lines[0])),
		mode: roleProbeVersionModeNone,
	}

	versionLineFound := false
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, roleProbeVersionLinePrefix) {
			continue
		}
		if versionLineFound {
			// Multiple kb-role-version lines: addon attempted the new contract
			// but emitted a duplicate. Reject explicitly.
			out.mode = roleProbeVersionModeMalformed
			out.version = 0
			return out
		}
		versionLineFound = true
		raw := strings.TrimPrefix(trimmed, roleProbeVersionLinePrefix)
		if raw == "" {
			out.mode = roleProbeVersionModeMalformed
			out.version = 0
			continue
		}
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			out.mode = roleProbeVersionModeMalformed
			out.version = 0
			continue
		}
		out.mode = roleProbeVersionModeEngine
		out.version = v
	}
	return out
}

// roleProbeGateDecision is the outcome of the staleness gate that determines
// whether a parsed roleProbe event may write the Pod role label and advance
// the LastRoleEventVersionAnnotationKey.
type roleProbeGateDecision int

const (
	// roleProbeGateAccept lets the caller write the Pod role label and
	// advance the annotation to newAnnotation.
	roleProbeGateAccept roleProbeGateDecision = iota

	// roleProbeGateRejectStale rejects the event because its version is not
	// strictly newer than the recorded version. The caller must not write
	// the Pod role label and must not change the annotation.
	roleProbeGateRejectStale

	// roleProbeGateRejectMalformed rejects the event because its stdout
	// carried a kb-role-version line that did not parse. The caller must
	// not write the Pod role label and must not change the annotation.
	roleProbeGateRejectMalformed
)

// checkRoleProbeStale decides whether the parsed roleProbe event should
// advance the Pod role label and what the new annotation value should be
// when the event is accepted. The four legitimate annotation/mode
// combinations are pinned by unit tests:
//
//   - engine event vs engine annotation: numeric compare; reject if not
//     strictly newer.
//   - engine event vs legacy bare EventTime annotation: accept and upgrade.
//   - legacy event vs engine annotation: reject; the engine-version gate
//     cannot be silently downgraded back to EventTime.
//   - legacy event vs legacy bare EventTime annotation: bare-numeric
//     compare on EventTime micros (preserves the pre-existing behaviour for
//     addons that have not adopted the new contract).
//
// A Malformed event is always rejected with the annotation untouched.
func checkRoleProbeStale(parsed roleProbeOutput, lastAnnotation string, eventTimeMicros int64) (roleProbeGateDecision, string) {
	switch parsed.mode {
	case roleProbeVersionModeMalformed:
		return roleProbeGateRejectMalformed, lastAnnotation
	case roleProbeVersionModeEngine:
		newAnnotation := fmt.Sprintf("%s%d", engineVersionAnnotationPrefix, parsed.version)
		if strings.HasPrefix(lastAnnotation, engineVersionAnnotationPrefix) {
			lastRaw := strings.TrimPrefix(lastAnnotation, engineVersionAnnotationPrefix)
			lastV, err := strconv.ParseUint(lastRaw, 10, 64)
			if err == nil && parsed.version <= lastV {
				return roleProbeGateRejectStale, lastAnnotation
			}
		}
		return roleProbeGateAccept, newAnnotation
	default: // roleProbeVersionModeNone (legacy event)
		if strings.HasPrefix(lastAnnotation, engineVersionAnnotationPrefix) {
			return roleProbeGateRejectStale, lastAnnotation
		}
		newAnnotation := fmt.Sprintf("%d", eventTimeMicros)
		if lastAnnotation != "" {
			lastV, err := strconv.ParseUint(lastAnnotation, 10, 64)
			if err == nil && uint64(eventTimeMicros) <= lastV {
				return roleProbeGateRejectStale, lastAnnotation
			}
		}
		return roleProbeGateAccept, newAnnotation
	}
}
