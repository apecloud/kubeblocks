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
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateConditionMessage_EmptyAndSmall(t *testing.T) {
	for name, in := range map[string]string{
		"empty":      "",
		"short":      "boom",
		"at_limit":   strings.Repeat("a", maxConditionMessageBytes),
		"just_below": strings.Repeat("a", maxConditionMessageBytes-1),
	} {
		t.Run(name, func(t *testing.T) {
			got := TruncateConditionMessage(in)
			if got != in {
				t.Fatalf("expected unchanged message, got len=%d (in len=%d)", len(got), len(in))
			}
		})
	}
}

func TestTruncateConditionMessage_LargeASCII(t *testing.T) {
	in := strings.Repeat("x", maxConditionMessageBytes*2)
	got := TruncateConditionMessage(in)
	if len(got) > maxConditionMessageBytes {
		t.Fatalf("expected len <= %d, got %d", maxConditionMessageBytes, len(got))
	}
	if !strings.HasSuffix(got, conditionMsgTruncationMarker) {
		t.Fatalf("expected trailing truncation marker, got %q", got[len(got)-100:])
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8 result")
	}
}

func TestTruncateConditionMessage_UTF8RuneBoundaryAfterFirstByte(t *testing.T) {
	// "中" is 3 bytes: 0xE4 0xB8 0xAD. Build a payload whose naive
	// byte-cut at maxConditionMessageBytes - len(marker) lands one byte
	// past a "中" start, leaving 0xE4 alone at the tail (rune start but
	// incomplete). The implementation must walk back to a valid UTF-8
	// boundary.
	limit := maxConditionMessageBytes - len(conditionMsgTruncationMarker)
	// Build a prefix of (limit-1) ASCII bytes, then "中". The byte at
	// index (limit-1) is the first byte 0xE4 of the multi-byte rune.
	in := strings.Repeat("a", limit-1) + "中" + strings.Repeat("a", maxConditionMessageBytes)

	got := TruncateConditionMessage(in)
	if !utf8.ValidString(got) {
		t.Fatalf("result is not valid UTF-8: %q", got[len(got)-10:])
	}
	if len(got) > maxConditionMessageBytes {
		t.Fatalf("expected len <= %d, got %d", maxConditionMessageBytes, len(got))
	}
	if !strings.HasSuffix(got, conditionMsgTruncationMarker) {
		t.Fatalf("expected marker suffix")
	}
}

func TestTruncateConditionMessage_UTF8RuneBoundaryAfterSecondByte(t *testing.T) {
	// Variant: naive cut lands after the second byte of "中" (0xB8 is
	// not a rune start, but walking back to the prior byte 0xE4 still
	// leaves an incomplete rune — the loop must continue until the
	// result parses as valid UTF-8.
	limit := maxConditionMessageBytes - len(conditionMsgTruncationMarker)
	in := strings.Repeat("a", limit-2) + "中" + strings.Repeat("a", maxConditionMessageBytes)

	got := TruncateConditionMessage(in)
	if !utf8.ValidString(got) {
		t.Fatalf("result is not valid UTF-8")
	}
	if len(got) > maxConditionMessageBytes {
		t.Fatalf("expected len <= %d, got %d", maxConditionMessageBytes, len(got))
	}
	if !strings.HasSuffix(got, conditionMsgTruncationMarker) {
		t.Fatalf("expected marker suffix")
	}
}

func TestTruncateConditionMessage_RealWorldErrorShape(t *testing.T) {
	// Approximates the C03 scenario: a single kbagent action error that
	// already exceeds 32 KiB on its own. Verify it fits the post-fix
	// budget and stays valid UTF-8.
	payload := "action: udf-shardingShardRemove, executed on pod: rds-x, error: exit code: 1, stderr: "
	payload += strings.Repeat("rds-cluster-shard-pod-fqdn-line ", 2000) // ~64 KiB
	got := TruncateConditionMessage(payload)
	if len(got) > maxConditionMessageBytes {
		t.Fatalf("expected len <= %d, got %d", maxConditionMessageBytes, len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8 result")
	}
	if !strings.HasSuffix(got, conditionMsgTruncationMarker) {
		t.Fatalf("expected truncation marker")
	}
}

func TestTruncateConditionReason_EmptyAndSmall(t *testing.T) {
	for name, in := range map[string]string{
		"empty":    "",
		"short":    "ApplyResourcesFailed",
		"at_limit": strings.Repeat("a", maxConditionReasonBytes),
	} {
		t.Run(name, func(t *testing.T) {
			got := TruncateConditionReason(in)
			if got != in {
				t.Fatalf("expected unchanged reason, got len=%d", len(got))
			}
		})
	}
}

func TestTruncateConditionReason_LargeASCII(t *testing.T) {
	in := strings.Repeat("R", maxConditionReasonBytes*3)
	got := TruncateConditionReason(in)
	if len(got) > maxConditionReasonBytes {
		t.Fatalf("expected len <= %d, got %d", maxConditionReasonBytes, len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8 result")
	}
}

func TestTruncateConditionReason_UTF8RuneBoundary(t *testing.T) {
	// Naive cut lands inside a 3-byte rune. The walk-back must stop on
	// a valid UTF-8 prefix.
	in := strings.Repeat("R", maxConditionReasonBytes-1) + "中" + strings.Repeat("R", maxConditionReasonBytes)
	got := TruncateConditionReason(in)
	if !utf8.ValidString(got) {
		t.Fatalf("result not valid UTF-8")
	}
	if len(got) > maxConditionReasonBytes {
		t.Fatalf("expected len <= %d, got %d", maxConditionReasonBytes, len(got))
	}
}
