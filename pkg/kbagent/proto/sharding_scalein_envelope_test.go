/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package proto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestShardingScaleInEnvelopeV2RoundTrip(t *testing.T) {
	input := testShardingScaleInEnvelopeV2(t)

	first, err := EncodeShardingScaleInEnvelopeV2(input)
	if err != nil {
		t.Fatalf("EncodeShardingScaleInEnvelopeV2() error = %v", err)
	}
	for range 100 {
		next, err := EncodeShardingScaleInEnvelopeV2(input)
		if err != nil {
			t.Fatalf("EncodeShardingScaleInEnvelopeV2() error = %v", err)
		}
		if !reflect.DeepEqual(next, first) {
			t.Fatal("envelope encoding is not deterministic")
		}
	}

	wantPrefix := "" +
		"KB-SHARD-SCALE-IN-ENVELOPE/2\n" +
		"PLAN_ID=" + strings.Repeat("1", 64) + "\n" +
		"TOPOLOGY_FENCE_TOKEN=" + strings.Repeat("2", 64) + "\n" +
		"REQUEST_AUTHORITY_DIGEST=" + strings.Repeat("3", 64) + "\n" +
		"BASE_PARAMETER_DIGEST=" + strings.Repeat("4", 64) + "\n" +
		"PHASE=Draining\n" +
		"HOLDER_PARAMETER_NAME=KB_REMOVE_SHARD_NAME\n" +
		"HOLDER_INDEX=0\n" +
		"HOLDER_COMPONENT_NAME_B64=Y29tcC1h\n" +
		"HOLDER_COMPONENT_UID_B64=dWlkLWE=\n" +
		"HOLDER_COMPONENT_SHORT_NAME_B64=YQ==\n" +
		"HOLDER_TARGET_SOURCE_DIGEST=" + input.Holder.SourceDigest + "\n" +
		"HOLDER_TARGET_VALUE_B64=Y29tcC1h\n" +
		"HOLDER_TARGET_VALUE_DIGEST=" + input.Holder.ValueDigest + "\n" +
		"MEMBER_COUNT=2\n"
	if !strings.HasPrefix(string(first), wantPrefix) {
		t.Fatalf("envelope prefix mismatch\n got: %q\nwant prefix: %q", first, wantPrefix)
	}
	if !strings.Contains(string(first), "\nRECEIPT_ID=\nENVELOPE_SHA256=") {
		t.Fatalf("envelope is missing the empty receipt/digest suffix: %q", first)
	}
	if first[len(first)-1] != '\n' || strings.HasSuffix(string(first), "\n\n") {
		t.Fatalf("envelope must end with exactly one LF: %q", first)
	}

	decoded, err := DecodeShardingScaleInEnvelopeV2(first)
	if err != nil {
		t.Fatalf("DecodeShardingScaleInEnvelopeV2() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, input) {
		t.Fatalf("round trip mismatch\n got: %#v\nwant: %#v", decoded, input)
	}
}

func TestShardingScaleInEnvelopeV2RejectsMalformedBytesAfterOuterRehash(t *testing.T) {
	valid, err := EncodeShardingScaleInEnvelopeV2(testShardingScaleInEnvelopeV2(t))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "missing final LF", mutate: func(value []byte) []byte {
			return append([]byte(nil), value[:len(value)-1]...)
		}},
		{name: "CRLF", mutate: func(value []byte) []byte {
			return []byte(strings.ReplaceAll(string(value), "\n", "\r\n"))
		}},
		{name: "BOM", mutate: func(value []byte) []byte {
			return append([]byte{0xef, 0xbb, 0xbf}, value...)
		}},
		{name: "NUL", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "PHASE=Draining", "PHASE=Drain\x00ing")
		}},
		{name: "extra blank line", mutate: func(value []byte) []byte {
			return append(append([]byte(nil), value...), '\n')
		}},
		{name: "unsupported version", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"KB-SHARD-SCALE-IN-ENVELOPE/2",
				"KB-SHARD-SCALE-IN-ENVELOPE/1")
		}},
		{name: "unknown field", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"PHASE=Draining", "UNKNOWN=1\nPHASE=Draining")
		}},
		{name: "reordered field", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"PLAN_ID="+strings.Repeat("1", 64)+"\n"+
					"TOPOLOGY_FENCE_TOKEN="+strings.Repeat("2", 64),
				"TOPOLOGY_FENCE_TOKEN="+strings.Repeat("2", 64)+"\n"+
					"PLAN_ID="+strings.Repeat("1", 64))
		}},
		{name: "uppercase digest", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"PLAN_ID="+strings.Repeat("1", 64),
				"PLAN_ID="+strings.Repeat("A", 64))
		}},
		{name: "unknown phase", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "PHASE=Draining", "PHASE=Planned")
		}},
		{name: "wrong parameter", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"HOLDER_PARAMETER_NAME=KB_REMOVE_SHARD_NAME",
				"HOLDER_PARAMETER_NAME=OTHER")
		}},
		{name: "leading-zero holder index", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "HOLDER_INDEX=0", "HOLDER_INDEX=00")
		}},
		{name: "holder index outside Leaving", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "HOLDER_INDEX=0", "HOLDER_INDEX=1")
		}},
		{name: "noncanonical holder base64", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"HOLDER_COMPONENT_UID_B64=dWlkLWE=",
				"HOLDER_COMPONENT_UID_B64=dWlkLWE")
		}},
		{name: "holder identity differs from Leaving", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"HOLDER_COMPONENT_UID_B64=dWlkLWE=",
				"HOLDER_COMPONENT_UID_B64=dWlkLXg=")
		}},
		{name: "holder source digest mismatch", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value,
				"HOLDER_TARGET_SOURCE_DIGEST="+testEnvelopeLineValue(
					t, value, "HOLDER_TARGET_SOURCE_DIGEST"),
				"HOLDER_TARGET_SOURCE_DIGEST="+strings.Repeat("5", 64))
		}},
		{name: "holder value differs from name with recomputed value digest", mutate: func(value []byte) []byte {
			otherValueB64 := base64.StdEncoding.EncodeToString([]byte("comp-x"))
			otherValueDigest := testShardingScaleInHolderValueDigest(t, otherValueB64)
			value = replaceAndRehashEnvelope(t, value,
				"HOLDER_TARGET_VALUE_B64=Y29tcC1h",
				"HOLDER_TARGET_VALUE_B64="+otherValueB64)
			return replaceAndRehashEnvelope(t, value,
				"HOLDER_TARGET_VALUE_DIGEST="+testEnvelopeLineValue(
					t, value, "HOLDER_TARGET_VALUE_DIGEST"),
				"HOLDER_TARGET_VALUE_DIGEST="+otherValueDigest)
		}},
		{name: "member count mismatch", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "MEMBER_COUNT=2", "MEMBER_COUNT=3")
		}},
		{name: "member index gap", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "MEMBER_001=", "MEMBER_002=")
		}},
		{name: "invalid receipt", mutate: func(value []byte) []byte {
			return replaceAndRehashEnvelope(t, value, "RECEIPT_ID=", "RECEIPT_ID=not-a-digest")
		}},
		{name: "envelope digest mismatch", mutate: func(value []byte) []byte {
			result := append([]byte(nil), value...)
			result[len(result)-2] ^= 1
			return result
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeShardingScaleInEnvelopeV2(tt.mutate(valid))
			if !errors.Is(err, ErrInvalidShardingScaleInEnvelope) {
				t.Fatalf("DecodeShardingScaleInEnvelopeV2() error = %v", err)
			}
		})
	}
}

func TestShardingScaleInEnvelopeV2RejectsChangedHolderWithEveryDigestRecomputed(
	t *testing.T,
) {
	valid, err := EncodeShardingScaleInEnvelopeV2(testShardingScaleInEnvelopeV2(t))
	if err != nil {
		t.Fatal(err)
	}

	const otherUID = "uid-x"
	sourceDigest := testShardingScaleInHolderSourceDigest(t,
		strings.Repeat("1", 64), 0, "comp-a", otherUID, "a")
	mutated := replaceAndRehashEnvelope(t, valid,
		"HOLDER_COMPONENT_UID_B64=dWlkLWE=",
		"HOLDER_COMPONENT_UID_B64="+base64.StdEncoding.EncodeToString([]byte(otherUID)))
	mutated = replaceAndRehashEnvelope(t, mutated,
		"HOLDER_TARGET_SOURCE_DIGEST="+testEnvelopeLineValue(
			t, mutated, "HOLDER_TARGET_SOURCE_DIGEST"),
		"HOLDER_TARGET_SOURCE_DIGEST="+sourceDigest)

	_, err = DecodeShardingScaleInEnvelopeV2(mutated)
	if !errors.Is(err, ErrInvalidShardingScaleInEnvelope) {
		t.Fatalf("DecodeShardingScaleInEnvelopeV2() error = %v", err)
	}
}

func TestShardingScaleInEnvelopeV2RejectsOversizedInput(t *testing.T) {
	_, err := DecodeShardingScaleInEnvelopeV2(
		[]byte(strings.Repeat("x", shardingScaleInEnvelopeMaxBytes+1)))
	if !errors.Is(err, ErrInvalidShardingScaleInEnvelope) {
		t.Fatalf("DecodeShardingScaleInEnvelopeV2() error = %v", err)
	}
}

func TestShardingScaleInEnvelopeV2PhaseReceiptContract(t *testing.T) {
	tests := []struct {
		name      string
		phase     string
		receiptID string
		wantError bool
	}{
		{name: "Draining", phase: "Draining"},
		{name: "PurgePrepared", phase: "PurgePrepared",
			receiptID: strings.Repeat("5", 64)},
		{name: "Resetting", phase: "Resetting",
			receiptID: strings.Repeat("5", 64)},
		{name: "Forgetting", phase: "Forgetting",
			receiptID: strings.Repeat("5", 64)},
		{name: "Verified", phase: "Verified",
			receiptID: strings.Repeat("5", 64)},
		{name: "Draining with receipt", phase: "Draining",
			receiptID: strings.Repeat("5", 64), wantError: true},
		{name: "PurgePrepared without receipt", phase: "PurgePrepared",
			wantError: true},
		{name: "Resetting without receipt", phase: "Resetting",
			wantError: true},
		{name: "Forgetting without receipt", phase: "Forgetting",
			wantError: true},
		{name: "Verified without receipt", phase: "Verified",
			wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := testShardingScaleInEnvelopeV2(t)
			input.Phase = tt.phase
			input.ReceiptID = tt.receiptID
			_, err := EncodeShardingScaleInEnvelopeV2(input)
			if tt.wantError {
				if !errors.Is(err, ErrInvalidShardingScaleInEnvelope) {
					t.Fatalf("EncodeShardingScaleInEnvelopeV2() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("EncodeShardingScaleInEnvelopeV2() error = %v", err)
			}
		})
	}
}

func TestShardingScaleInEnvelopeV2RejectsOversizedEncoding(t *testing.T) {
	input := testShardingScaleInEnvelopeV2(t)
	input.Members = ShardingScaleInMemberSet{}
	for index := 0; index < 12; index++ {
		prefix := string(rune('a' + index))
		member := ShardingScaleInMember{
			ComponentName:       strings.Repeat(prefix, ShardingScaleInMemberFieldMaxBytes),
			ComponentUID:        strings.Repeat(prefix, ShardingScaleInMemberFieldMaxBytes-1) + "u",
			ComponentGeneration: 7,
			ComponentSpecDigest: strings.Repeat("a", 64),
			ComponentShortName:  prefix,
			ShardTemplateName:   "redis",
			Pods: []ShardingScaleInMemberPod{{
				Name:                  "pod-" + prefix,
				UID:                   "pod-uid-" + prefix,
				FQDN:                  "pod-" + prefix + ".redis.svc",
				AgentImageID:          "sha256:111",
				AgentProcessUID:       "process-" + prefix,
				AgentCapabilityDigest: strings.Repeat("b", 64),
			}},
		}
		if index < 6 {
			input.Members.Leaving = append(input.Members.Leaving, member)
		} else {
			input.Members.Staying = append(input.Members.Staying, member)
		}
	}
	holderMember := input.Members.Leaving[0]
	input.Holder.ComponentName = holderMember.ComponentName
	input.Holder.ComponentUID = holderMember.ComponentUID
	input.Holder.ComponentShortName = holderMember.ComponentShortName
	input.Holder.ValueB64 = base64.StdEncoding.EncodeToString(
		[]byte(holderMember.ComponentName))
	input.Holder.SourceDigest = testShardingScaleInHolderSourceDigest(t,
		input.PlanID, input.Holder.HolderIndex,
		input.Holder.ComponentName, input.Holder.ComponentUID,
		input.Holder.ComponentShortName)
	input.Holder.ValueDigest = testShardingScaleInHolderValueDigest(
		t, input.Holder.ValueB64)

	_, err := EncodeShardingScaleInEnvelopeV2(input)
	if !errors.Is(err, ErrInvalidShardingScaleInEnvelope) {
		t.Fatalf("EncodeShardingScaleInEnvelopeV2() error = %v", err)
	}
}

func testShardingScaleInEnvelopeV2(t *testing.T) ShardingScaleInEnvelopeV2 {
	t.Helper()
	holder := ShardingScaleInHolderTarget{
		ParameterName:      "KB_REMOVE_SHARD_NAME",
		HolderIndex:        0,
		ComponentName:      "comp-a",
		ComponentUID:       "uid-a",
		ComponentShortName: "a",
		ValueB64:           base64.StdEncoding.EncodeToString([]byte("comp-a")),
	}
	holder.SourceDigest = testShardingScaleInHolderSourceDigest(t,
		strings.Repeat("1", 64), holder.HolderIndex,
		holder.ComponentName, holder.ComponentUID, holder.ComponentShortName)
	holder.ValueDigest = testShardingScaleInHolderValueDigest(t, holder.ValueB64)
	return ShardingScaleInEnvelopeV2{
		PlanID:                 strings.Repeat("1", 64),
		TopologyFenceToken:     strings.Repeat("2", 64),
		RequestAuthorityDigest: strings.Repeat("3", 64),
		BaseParameterDigest:    strings.Repeat("4", 64),
		Phase:                  "Draining",
		Holder:                 holder,
		Members: ShardingScaleInMemberSet{
			Leaving: []ShardingScaleInMember{
				testShardingScaleInMember("comp-a", "uid-a", "a", 1),
			},
			Staying: []ShardingScaleInMember{
				testShardingScaleInMember("comp-b", "uid-b", "b", 1),
			},
		},
	}
}

func testShardingScaleInHolderSourceDigest(
	t *testing.T,
	planID string,
	holderIndex uint32,
	componentName, componentUID, componentShortName string,
) string {
	t.Helper()
	value := struct {
		Version            string `json:"version"`
		PlanID             string `json:"planID"`
		HolderIndex        int32  `json:"holderIndex"`
		ComponentName      string `json:"componentName"`
		ComponentUID       string `json:"componentUID"`
		ComponentShortName string `json:"componentShortName"`
	}{
		Version:            "kb.sharding.scalein.holder-target/v1",
		PlanID:             planID,
		HolderIndex:        int32(holderIndex),
		ComponentName:      componentName,
		ComponentUID:       componentUID,
		ComponentShortName: componentShortName,
	}
	return testShardingScaleInJSONDigest(t, value)
}

func testShardingScaleInHolderValueDigest(t *testing.T, valueB64 string) string {
	t.Helper()
	return testShardingScaleInJSONDigest(t, struct {
		Version  string `json:"version"`
		ValueB64 string `json:"valueB64"`
	}{
		Version:  "kb.sharding.scalein.holder-target-value/v1",
		ValueB64: valueB64,
	})
}

func testShardingScaleInJSONDigest(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func replaceAndRehashEnvelope(
	t *testing.T,
	value []byte,
	old, replacement string,
) []byte {
	t.Helper()
	lines := strings.Split(string(value), "\n")
	if len(lines) < 2 || lines[len(lines)-1] != "" {
		t.Fatalf("invalid test envelope fixture")
	}
	body := strings.Join(lines[:len(lines)-2], "\n") + "\n"
	if !strings.Contains(body, old) {
		t.Fatalf("test replacement source %q is absent", old)
	}
	body = strings.Replace(body, old, replacement, 1)
	digest := sha256.Sum256([]byte(body))
	return []byte(body + "ENVELOPE_SHA256=" + hex.EncodeToString(digest[:]) + "\n")
}

func testEnvelopeLineValue(t *testing.T, value []byte, key string) string {
	t.Helper()
	prefix := key + "="
	for _, line := range strings.Split(string(value), "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("test envelope key %q is absent", key)
	return strconv.Itoa(-1)
}
