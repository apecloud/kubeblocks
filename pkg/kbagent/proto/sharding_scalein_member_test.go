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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestShardingScaleInMemberRecordGolden(t *testing.T) {
	member := testShardingScaleInMember("comp-a", "uid-a", "a", 1)

	decoded, err := EncodeShardingScaleInMemberRecord(
		ShardingScaleInMemberPartitionLeaving, 0, member)
	if err != nil {
		t.Fatalf("EncodeShardingScaleInMemberRecord() error = %v", err)
	}
	const want = "" +
		"KB-SHARD-SCALE-IN-MEMBER/1\n" +
		"PARTITION=Leaving\n" +
		"PARTITION_INDEX=0\n" +
		"COMPONENT_NAME_B64=Y29tcC1h\n" +
		"COMPONENT_UID_B64=dWlkLWE=\n" +
		"COMPONENT_GENERATION=7\n" +
		"COMPONENT_SPEC_DIGEST=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n" +
		"COMPONENT_SHORT_NAME_B64=YQ==\n" +
		"SHARD_TEMPLATE_NAME_B64=cmVkaXM=\n" +
		"POD_COUNT=1\n" +
		"POD_000_NAME_B64=Y29tcC1hLTA=\n" +
		"POD_000_UID_B64=cG9kLWEtMA==\n" +
		"POD_000_FQDN_B64=Y29tcC1hLTAucmVkaXMuc3Zj\n" +
		"POD_000_AGENT_IMAGE_ID_B64=c2hhMjU2OjExMQ==\n" +
		"POD_000_AGENT_PROCESS_UID_B64=cHJvYy1hLTA=\n" +
		"POD_000_AGENT_CAPABILITY_DIGEST=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n" +
		"MEMBER_SHA256=e647ba15bd3609b6c834456333a5e6329ee8f39ea1d27d2118d0443a04168980\n"
	if string(decoded) != want {
		t.Fatalf("decoded member mismatch\n got: %q\nwant: %q", decoded, want)
	}

	record, err := DecodeShardingScaleInMemberRecord(decoded)
	if err != nil {
		t.Fatalf("DecodeShardingScaleInMemberRecord() error = %v", err)
	}
	if record.Partition != ShardingScaleInMemberPartitionLeaving ||
		record.PartitionIndex != 0 || !reflect.DeepEqual(record.Member, member) {
		t.Fatalf("decoded record = %#v, want Leaving/0/%#v", record, member)
	}
}

func TestShardingScaleInMemberSetRoundTrip(t *testing.T) {
	input := ShardingScaleInMemberSet{
		Leaving: []ShardingScaleInMember{
			testShardingScaleInMember("comp-a", "uid-a", "a", 1),
			testShardingScaleInMember("comp-b", "uid-b", "b", 3),
		},
		Staying: []ShardingScaleInMember{
			testShardingScaleInMember("comp-c", "uid-c", "c", 5),
			testShardingScaleInMember("comp-d", "uid-d", "d", 1),
		},
	}

	lines, err := EncodeShardingScaleInMemberSet(input)
	if err != nil {
		t.Fatalf("EncodeShardingScaleInMemberSet() error = %v", err)
	}
	if len(lines) != 5 || lines[0] != "MEMBER_COUNT=4" {
		t.Fatalf("outer lines = %#v", lines)
	}
	for i := 1; i < len(lines); i++ {
		wantPrefix := fmt.Sprintf("MEMBER_%03d=", i-1)
		if !strings.HasPrefix(lines[i], wantPrefix) {
			t.Fatalf("line %d = %q, want prefix %q", i, lines[i], wantPrefix)
		}
		value := strings.TrimPrefix(lines[i], wantPrefix)
		decoded, err := base64.StdEncoding.Strict().DecodeString(value)
		if err != nil {
			t.Fatalf("outer member %d is not standard base64: %v", i-1, err)
		}
		if base64.StdEncoding.EncodeToString(decoded) != value {
			t.Fatalf("outer member %d is not canonical base64", i-1)
		}
	}

	got, err := DecodeShardingScaleInMemberSet(lines)
	if err != nil {
		t.Fatalf("DecodeShardingScaleInMemberSet() error = %v", err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("round trip mismatch\n got: %#v\nwant: %#v", got, input)
	}
}

func TestShardingScaleInMemberRecordRejectsMalformedBytes(t *testing.T) {
	member := testShardingScaleInMember("comp-a", "uid-a", "a", 1)
	valid, err := EncodeShardingScaleInMemberRecord(
		ShardingScaleInMemberPartitionLeaving, 0, member)
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
		{name: "extra blank", mutate: func(value []byte) []byte {
			return append(append([]byte(nil), value...), '\n')
		}},
		{name: "unknown field", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"POD_COUNT=1", "EXTRA=1\nPOD_COUNT=1")
		}},
		{name: "reordered field", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_NAME_B64=Y29tcC1h\nCOMPONENT_UID_B64=dWlkLWE=",
				"COMPONENT_UID_B64=dWlkLWE=\nCOMPONENT_NAME_B64=Y29tcC1h")
		}},
		{name: "noncanonical inner base64", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_UID_B64=dWlkLWE=", "COMPONENT_UID_B64=dWlkLWE")
		}},
		{name: "URL safe inner base64", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_UID_B64=dWlkLWE=", "COMPONENT_UID_B64=__8=")
		}},
		{name: "leading zero generation", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_GENERATION=7", "COMPONENT_GENERATION=07")
		}},
		{name: "zero generation", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_GENERATION=7", "COMPONENT_GENERATION=0")
		}},
		{name: "int64 generation overflow", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_GENERATION=7", "COMPONENT_GENERATION=9223372036854775808")
		}},
		{name: "uppercase digest", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value,
				"COMPONENT_SPEC_DIGEST=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"COMPONENT_SPEC_DIGEST=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		}},
		{name: "pod count mismatch", mutate: func(value []byte) []byte {
			return replaceAndRehashMember(t, value, "POD_COUNT=1", "POD_COUNT=2")
		}},
		{name: "member digest changed", mutate: func(value []byte) []byte {
			result := append([]byte(nil), value...)
			result[len(result)-2] ^= 1
			return result
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeShardingScaleInMemberRecord(tt.mutate(valid))
			if !errors.Is(err, ErrInvalidShardingScaleInMemberRecord) {
				t.Fatalf("DecodeShardingScaleInMemberRecord() error = %v", err)
			}
		})
	}
}

func TestShardingScaleInMemberRecordAcceptsMaxInt64Generation(t *testing.T) {
	member := testShardingScaleInMember("comp-a", "uid-a", "a", 1)
	member.ComponentGeneration = 9223372036854775807

	record, err := EncodeShardingScaleInMemberRecord(
		ShardingScaleInMemberPartitionLeaving, 0, member)
	if err != nil {
		t.Fatalf("EncodeShardingScaleInMemberRecord() error = %v", err)
	}
	decoded, err := DecodeShardingScaleInMemberRecord(record)
	if err != nil {
		t.Fatalf("DecodeShardingScaleInMemberRecord() error = %v", err)
	}
	if !reflect.DeepEqual(decoded.Member, member) {
		t.Fatalf("max-int64 generation did not round trip")
	}
}

func TestShardingScaleInMemberSetRejectsSemanticReorderAfterRehash(t *testing.T) {
	input := ShardingScaleInMemberSet{
		Leaving: []ShardingScaleInMember{
			testShardingScaleInMember("comp-a", "uid-a", "a", 1),
			testShardingScaleInMember("comp-b", "uid-b", "b", 1),
		},
		Staying: []ShardingScaleInMember{
			testShardingScaleInMember("comp-c", "uid-c", "c", 1),
		},
	}
	valid, err := EncodeShardingScaleInMemberSet(input)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func([]string) []string
	}{
		{name: "outer index gap", mutate: func(lines []string) []string {
			lines[2] = strings.Replace(lines[2], "MEMBER_001=", "MEMBER_002=", 1)
			return lines
		}},
		{name: "missing member", mutate: func(lines []string) []string {
			return lines[:len(lines)-1]
		}},
		{name: "extra member", mutate: func(lines []string) []string {
			return append(lines, lines[len(lines)-1])
		}},
		{name: "noncanonical outer base64", mutate: func(lines []string) []string {
			lines[1] = strings.TrimSuffix(lines[1], "=")
			return lines
		}},
		{name: "partition changed and fully rehashed", mutate: func(lines []string) []string {
			lines[1] = rewriteOuterMember(t, lines[1], func(decoded []byte) []byte {
				return replaceAndRehashMember(t, decoded,
					"PARTITION=Leaving", "PARTITION=Staying")
			})
			return lines
		}},
		{name: "partition index changed and fully rehashed", mutate: func(lines []string) []string {
			lines[2] = rewriteOuterMember(t, lines[2], func(decoded []byte) []byte {
				return replaceAndRehashMember(t, decoded,
					"PARTITION_INDEX=1", "PARTITION_INDEX=0")
			})
			return lines
		}},
		{name: "component tuple reordered and fully rehashed", mutate: func(lines []string) []string {
			lines[1], lines[2] = lines[2], lines[1]
			lines[1] = strings.Replace(lines[1], "MEMBER_001=", "MEMBER_000=", 1)
			lines[2] = strings.Replace(lines[2], "MEMBER_000=", "MEMBER_001=", 1)
			lines[1] = rewriteOuterMember(t, lines[1], func(decoded []byte) []byte {
				return replaceAndRehashMember(t, decoded,
					"PARTITION_INDEX=1", "PARTITION_INDEX=0")
			})
			lines[2] = rewriteOuterMember(t, lines[2], func(decoded []byte) []byte {
				return replaceAndRehashMember(t, decoded,
					"PARTITION_INDEX=0", "PARTITION_INDEX=1")
			})
			return lines
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := append([]string(nil), valid...)
			_, err := DecodeShardingScaleInMemberSet(tt.mutate(lines))
			if !errors.Is(err, ErrInvalidShardingScaleInMemberRecord) {
				t.Fatalf("DecodeShardingScaleInMemberSet() error = %v", err)
			}
		})
	}
}

func TestShardingScaleInMemberSetRejectsDuplicateRosterIdentity(t *testing.T) {
	valid := ShardingScaleInMemberSet{
		Leaving: []ShardingScaleInMember{
			testShardingScaleInMember("comp-a", "uid-a", "a", 1),
		},
		Staying: []ShardingScaleInMember{
			testShardingScaleInMember("comp-b", "uid-b", "b", 1),
		},
	}

	tests := []struct {
		name   string
		mutate func(*ShardingScaleInMemberSet)
	}{
		{name: "component name", mutate: func(value *ShardingScaleInMemberSet) {
			value.Staying[0].ComponentName = value.Leaving[0].ComponentName
		}},
		{name: "component UID", mutate: func(value *ShardingScaleInMemberSet) {
			value.Staying[0].ComponentUID = value.Leaving[0].ComponentUID
		}},
		{name: "component short name", mutate: func(value *ShardingScaleInMemberSet) {
			value.Staying[0].ComponentShortName = value.Leaving[0].ComponentShortName
		}},
		{name: "Pod name", mutate: func(value *ShardingScaleInMemberSet) {
			value.Staying[0].Pods[0].Name = value.Leaving[0].Pods[0].Name
		}},
		{name: "Pod UID", mutate: func(value *ShardingScaleInMemberSet) {
			value.Staying[0].Pods[0].UID = value.Leaving[0].Pods[0].UID
		}},
		{name: "Pod FQDN", mutate: func(value *ShardingScaleInMemberSet) {
			value.Staying[0].Pods[0].FQDN = value.Leaving[0].Pods[0].FQDN
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := cloneTestShardingScaleInMemberSet(valid)
			tt.mutate(&value)
			_, err := EncodeShardingScaleInMemberSet(value)
			if !errors.Is(err, ErrInvalidShardingScaleInMemberRecord) {
				t.Fatalf("EncodeShardingScaleInMemberSet() error = %v", err)
			}
		})
	}
}

func TestShardingScaleInMemberSetRejectsDuplicateHiddenByCrossFieldEquality(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ShardingScaleInMemberSet)
	}{
		{name: "Component name equals own UID", mutate: func(value *ShardingScaleInMemberSet) {
			value.Leaving[0].ComponentName = "same"
			value.Leaving[0].ComponentUID = "same"
			value.Staying[0].ComponentName = "same"
		}},
		{name: "Pod name equals own UID", mutate: func(value *ShardingScaleInMemberSet) {
			value.Leaving[0].Pods[0].Name = "same"
			value.Leaving[0].Pods[0].UID = "same"
			value.Staying[0].Pods[0].Name = "same"
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := ShardingScaleInMemberSet{
				Leaving: []ShardingScaleInMember{
					testShardingScaleInMember("comp-a", "uid-a", "a", 1),
				},
				Staying: []ShardingScaleInMember{
					testShardingScaleInMember("comp-b", "uid-b", "b", 1),
				},
			}
			tt.mutate(&value)
			_, err := EncodeShardingScaleInMemberSet(value)
			if !errors.Is(err, ErrInvalidShardingScaleInMemberRecord) {
				t.Fatalf("EncodeShardingScaleInMemberSet() error = %v", err)
			}
		})
	}
}

func TestShardingScaleInMemberSetRejectsOversizedOuterValueBeforeDecode(t *testing.T) {
	value := ShardingScaleInMemberSet{
		Leaving: []ShardingScaleInMember{
			testShardingScaleInMember("comp-a", "uid-a", "a", 1),
		},
		Staying: []ShardingScaleInMember{
			testShardingScaleInMember("comp-b", "uid-b", "b", 1),
		},
	}
	lines, err := EncodeShardingScaleInMemberSet(value)
	if err != nil {
		t.Fatal(err)
	}
	lines[1] = "MEMBER_000=" + strings.Repeat(
		"A", base64.StdEncoding.EncodedLen(shardingScaleInMemberRecordMaxBytes)+1)

	_, err = DecodeShardingScaleInMemberSet(lines)
	if !errors.Is(err, ErrInvalidShardingScaleInMemberRecord) {
		t.Fatalf("DecodeShardingScaleInMemberSet() error = %v", err)
	}
}

func TestShardingScaleInMemberRecordRejectsSizeOverflow(t *testing.T) {
	member := testShardingScaleInMember("comp-a", "uid-a", "a", 5)
	for index := range member.Pods {
		member.Pods[index].FQDN = strings.Repeat(
			string(rune('a'+index)), ShardingScaleInMemberFieldMaxBytes-1)
	}

	_, err := EncodeShardingScaleInMemberRecord(
		ShardingScaleInMemberPartitionLeaving, 0, member)
	if !errors.Is(err, ErrInvalidShardingScaleInMemberRecord) {
		t.Fatalf("EncodeShardingScaleInMemberRecord() error = %v", err)
	}
}

func TestShardingScaleInMemberRecordAcceptsExactSizeBoundary(t *testing.T) {
	member := testShardingScaleInMember("comp-a", "uid-a", "a", 1)
	member.ComponentName = strings.Repeat("a", ShardingScaleInMemberFieldMaxBytes)
	member.ComponentUID = strings.Repeat("b", ShardingScaleInMemberFieldMaxBytes)
	member.ComponentShortName = strings.Repeat("c", 3604)

	record, err := EncodeShardingScaleInMemberRecord(
		ShardingScaleInMemberPartitionLeaving, 10, member)
	if err != nil {
		t.Fatalf("EncodeShardingScaleInMemberRecord() error = %v", err)
	}
	if len(record) != shardingScaleInMemberRecordMaxBytes {
		t.Fatalf("record size = %d, want %d",
			len(record), shardingScaleInMemberRecordMaxBytes)
	}
	decoded, err := DecodeShardingScaleInMemberRecord(record)
	if err != nil {
		t.Fatalf("DecodeShardingScaleInMemberRecord() error = %v", err)
	}
	if decoded.PartitionIndex != 10 || !reflect.DeepEqual(decoded.Member, member) {
		t.Fatalf("exact-boundary record did not round trip")
	}

	over := append(append([]byte(nil), record...), '\n')
	if _, err := DecodeShardingScaleInMemberRecord(over); !errors.Is(
		err, ErrInvalidShardingScaleInMemberRecord) {
		t.Fatalf("DecodeShardingScaleInMemberRecord(+1) error = %v", err)
	}
}

func testShardingScaleInMember(
	name, uid, shortName string,
	podCount int,
) ShardingScaleInMember {
	member := ShardingScaleInMember{
		ComponentName:       name,
		ComponentUID:        uid,
		ComponentGeneration: 7,
		ComponentSpecDigest: strings.Repeat("a", 64),
		ComponentShortName:  shortName,
		ShardTemplateName:   "redis",
	}
	for index := 0; index < podCount; index++ {
		member.Pods = append(member.Pods, ShardingScaleInMemberPod{
			Name:                  fmt.Sprintf("%s-%d", name, index),
			UID:                   fmt.Sprintf("pod-%s-%d", shortName, index),
			FQDN:                  fmt.Sprintf("%s-%d.redis.svc", name, index),
			AgentImageID:          "sha256:111",
			AgentProcessUID:       fmt.Sprintf("proc-%s-%d", shortName, index),
			AgentCapabilityDigest: strings.Repeat("b", 64),
		})
	}
	return member
}

func replaceAndRehashMember(
	t *testing.T,
	value []byte,
	old, replacement string,
) []byte {
	t.Helper()
	lines := strings.Split(string(value), "\n")
	if len(lines) < 2 || lines[len(lines)-1] != "" {
		t.Fatalf("invalid test member fixture")
	}
	body := strings.Join(lines[:len(lines)-2], "\n") + "\n"
	if !strings.Contains(body, old) {
		t.Fatalf("test replacement source %q is absent", old)
	}
	body = strings.Replace(body, old, replacement, 1)
	digest := sha256.Sum256([]byte(body))
	return []byte(body + "MEMBER_SHA256=" + hex.EncodeToString(digest[:]) + "\n")
}

func rewriteOuterMember(
	t *testing.T,
	line string,
	mutate func([]byte) []byte,
) string {
	t.Helper()
	index := strings.IndexByte(line, '=')
	if index < 0 {
		t.Fatalf("invalid outer member line %q", line)
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(line[index+1:])
	if err != nil {
		t.Fatal(err)
	}
	return line[:index+1] + base64.StdEncoding.EncodeToString(mutate(decoded))
}

func cloneTestShardingScaleInMemberSet(
	value ShardingScaleInMemberSet,
) ShardingScaleInMemberSet {
	cloneMembers := func(input []ShardingScaleInMember) []ShardingScaleInMember {
		output := make([]ShardingScaleInMember, len(input))
		copy(output, input)
		for index := range output {
			output[index].Pods = append([]ShardingScaleInMemberPod(nil), input[index].Pods...)
		}
		return output
	}
	return ShardingScaleInMemberSet{
		Leaving: cloneMembers(value.Leaving),
		Staying: cloneMembers(value.Staying),
	}
}
