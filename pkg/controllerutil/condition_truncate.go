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

package controllerutil

import (
	"unicode/utf8"
)

// metav1.Condition.message has a CRD maxLength of 32768 bytes. Leave a margin
// so the truncation marker fits and writes still succeed when the unbounded
// underlying error (for example, a kbagent action stderr dump that already
// exceeds 32768 bytes by itself) would otherwise cause the API server to
// reject the status patch with "Too long: may not be more than 32768 bytes".
const maxConditionMessageBytes = 32000

// metav1.Condition.reason has a CRD maxLength of 1024 bytes.
const maxConditionReasonBytes = 1024

// conditionMsgTruncationMarker is appended to a truncated message so readers
// can tell the message was cut off rather than naturally short. The full
// underlying error remains available in the controller log; this marker only
// covers the on-cluster Condition surface.
const conditionMsgTruncationMarker = "\n[...truncated; see controller log for full error]"

// TruncateConditionMessage returns msg unchanged when it fits in the Condition
// message budget, otherwise it returns a UTF-8 safe prefix followed by the
// truncation marker. The result is guaranteed to be valid UTF-8 and no longer
// than maxConditionMessageBytes bytes.
func TruncateConditionMessage(msg string) string {
	if len(msg) <= maxConditionMessageBytes {
		return msg
	}
	limit := maxConditionMessageBytes - len(conditionMsgTruncationMarker)
	truncated := msg[:limit]
	// A naive byte-level cut can land in the middle of a multi-byte rune.
	// Walk back one byte at a time until the remaining prefix parses as
	// valid UTF-8 — this also handles the case where the final byte is a
	// rune start but the rune itself is incomplete.
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + conditionMsgTruncationMarker
}

// TruncateConditionReason returns reason unchanged when it fits in the
// Condition reason budget, otherwise a UTF-8 safe prefix cut to fit. No marker
// is appended — reason is a short identifier consumed by automation, not a
// human-readable message.
func TruncateConditionReason(reason string) string {
	if len(reason) <= maxConditionReasonBytes {
		return reason
	}
	truncated := reason[:maxConditionReasonBytes]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}
