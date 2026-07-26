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
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	shardingScaleInMemberRecordVersion = "KB-SHARD-SCALE-IN-MEMBER/1"

	ShardingScaleInMemberFieldMaxBytes = 4 * 1024

	shardingScaleInMemberRecordMaxBytes = 16 * 1024
	shardingScaleInMemberMinPodCount    = 1
	shardingScaleInMemberMaxPodCount    = 5
	shardingScaleInMemberSetMinCount    = 2
	shardingScaleInMemberSetMaxCount    = 32
)

var ErrInvalidShardingScaleInMemberRecord = errors.New(
	"invalid sharding scale-in member record")

type ShardingScaleInMemberPartition string

const (
	ShardingScaleInMemberPartitionLeaving ShardingScaleInMemberPartition = "Leaving"
	ShardingScaleInMemberPartitionStaying ShardingScaleInMemberPartition = "Staying"
)

type ShardingScaleInMemberPod struct {
	Name                  string
	UID                   string
	FQDN                  string
	AgentImageID          string
	AgentProcessUID       string
	AgentCapabilityDigest string
}

type ShardingScaleInMember struct {
	ComponentName       string
	ComponentUID        string
	ComponentGeneration uint64
	ComponentSpecDigest string
	ComponentShortName  string
	ShardTemplateName   string
	Pods                []ShardingScaleInMemberPod
}

type ShardingScaleInMemberRecord struct {
	Partition      ShardingScaleInMemberPartition
	PartitionIndex uint32
	Member         ShardingScaleInMember
}

type ShardingScaleInMemberSet struct {
	Leaving []ShardingScaleInMember
	Staying []ShardingScaleInMember
}

func EncodeShardingScaleInMemberRecord(
	partition ShardingScaleInMemberPartition,
	partitionIndex uint32,
	member ShardingScaleInMember,
) ([]byte, error) {
	if err := validateShardingScaleInMemberPartition(partition); err != nil {
		return nil, err
	}
	if err := validateShardingScaleInMember(member); err != nil {
		return nil, err
	}

	lines := []string{
		shardingScaleInMemberRecordVersion,
		"PARTITION=" + string(partition),
		"PARTITION_INDEX=" + strconv.FormatUint(uint64(partitionIndex), 10),
		"COMPONENT_NAME_B64=" + encodeShardingScaleInMemberField(member.ComponentName),
		"COMPONENT_UID_B64=" + encodeShardingScaleInMemberField(member.ComponentUID),
		"COMPONENT_GENERATION=" + strconv.FormatUint(member.ComponentGeneration, 10),
		"COMPONENT_SPEC_DIGEST=" + member.ComponentSpecDigest,
		"COMPONENT_SHORT_NAME_B64=" + encodeShardingScaleInMemberField(member.ComponentShortName),
		"SHARD_TEMPLATE_NAME_B64=" + encodeShardingScaleInMemberField(member.ShardTemplateName),
		"POD_COUNT=" + strconv.Itoa(len(member.Pods)),
	}
	for index, pod := range member.Pods {
		prefix := fmt.Sprintf("POD_%03d_", index)
		lines = append(lines,
			prefix+"NAME_B64="+encodeShardingScaleInMemberField(pod.Name),
			prefix+"UID_B64="+encodeShardingScaleInMemberField(pod.UID),
			prefix+"FQDN_B64="+encodeShardingScaleInMemberField(pod.FQDN),
			prefix+"AGENT_IMAGE_ID_B64="+encodeShardingScaleInMemberField(pod.AgentImageID),
			prefix+"AGENT_PROCESS_UID_B64="+encodeShardingScaleInMemberField(pod.AgentProcessUID),
			prefix+"AGENT_CAPABILITY_DIGEST="+pod.AgentCapabilityDigest,
		)
	}

	body := strings.Join(lines, "\n") + "\n"
	digest := sha256.Sum256([]byte(body))
	record := []byte(body + "MEMBER_SHA256=" + hex.EncodeToString(digest[:]) + "\n")
	if len(record) > shardingScaleInMemberRecordMaxBytes {
		return nil, invalidShardingScaleInMemberRecordf(
			"encoded member exceeds %d bytes", shardingScaleInMemberRecordMaxBytes)
	}
	return record, nil
}

func DecodeShardingScaleInMemberRecord(value []byte) (ShardingScaleInMemberRecord, error) {
	var zero ShardingScaleInMemberRecord
	if len(value) == 0 || len(value) > shardingScaleInMemberRecordMaxBytes {
		return zero, invalidShardingScaleInMemberRecordf(
			"member size must be between 1 and %d bytes", shardingScaleInMemberRecordMaxBytes)
	}
	if value[len(value)-1] != '\n' ||
		strings.ContainsRune(string(value), '\r') ||
		strings.ContainsRune(string(value), '\x00') ||
		strings.HasPrefix(string(value), "\ufeff") {
		return zero, invalidShardingScaleInMemberRecordf("invalid member framing")
	}

	lines := strings.Split(string(value), "\n")
	if len(lines) < 2 || lines[len(lines)-1] != "" {
		return zero, invalidShardingScaleInMemberRecordf("invalid member line framing")
	}
	lines = lines[:len(lines)-1]
	if lines[0] != shardingScaleInMemberRecordVersion {
		return zero, invalidShardingScaleInMemberRecordf("unsupported member record version")
	}
	if len(lines) < 11 {
		return zero, invalidShardingScaleInMemberRecordf("member has too few lines")
	}

	partitionValue, err := exactShardingScaleInMemberLine(lines[1], "PARTITION")
	if err != nil {
		return zero, err
	}
	partition := ShardingScaleInMemberPartition(partitionValue)
	if err := validateShardingScaleInMemberPartition(partition); err != nil {
		return zero, err
	}
	partitionIndex, err := parseCanonicalShardingScaleInMemberUint(
		lines[2], "PARTITION_INDEX", 0, uint64(^uint32(0)))
	if err != nil {
		return zero, err
	}
	componentName, err := decodeShardingScaleInMemberFieldLine(lines[3], "COMPONENT_NAME_B64")
	if err != nil {
		return zero, err
	}
	componentUID, err := decodeShardingScaleInMemberFieldLine(lines[4], "COMPONENT_UID_B64")
	if err != nil {
		return zero, err
	}
	componentGeneration, err := parseCanonicalShardingScaleInMemberUint(
		lines[5], "COMPONENT_GENERATION", 1, math.MaxInt64)
	if err != nil {
		return zero, err
	}
	componentSpecDigest, err := exactShardingScaleInMemberLine(
		lines[6], "COMPONENT_SPEC_DIGEST")
	if err != nil {
		return zero, err
	}
	if err := validateShardingScaleInMemberDigest(
		"component spec digest", componentSpecDigest); err != nil {
		return zero, err
	}
	componentShortName, err := decodeShardingScaleInMemberFieldLine(
		lines[7], "COMPONENT_SHORT_NAME_B64")
	if err != nil {
		return zero, err
	}
	shardTemplateName, err := decodeShardingScaleInMemberFieldLine(
		lines[8], "SHARD_TEMPLATE_NAME_B64")
	if err != nil {
		return zero, err
	}
	podCount, err := parseCanonicalShardingScaleInMemberUint(
		lines[9], "POD_COUNT",
		shardingScaleInMemberMinPodCount, shardingScaleInMemberMaxPodCount)
	if err != nil {
		return zero, err
	}
	if len(lines) != 11+6*int(podCount) {
		return zero, invalidShardingScaleInMemberRecordf("member line count does not match Pod count")
	}

	member := ShardingScaleInMember{
		ComponentName:       componentName,
		ComponentUID:        componentUID,
		ComponentGeneration: componentGeneration,
		ComponentSpecDigest: componentSpecDigest,
		ComponentShortName:  componentShortName,
		ShardTemplateName:   shardTemplateName,
		Pods:                make([]ShardingScaleInMemberPod, 0, podCount),
	}
	lineIndex := 10
	for podIndex := uint64(0); podIndex < podCount; podIndex++ {
		prefix := fmt.Sprintf("POD_%03d_", podIndex)
		name, decodeErr := decodeShardingScaleInMemberFieldLine(
			lines[lineIndex], prefix+"NAME_B64")
		if decodeErr != nil {
			return zero, decodeErr
		}
		uid, decodeErr := decodeShardingScaleInMemberFieldLine(
			lines[lineIndex+1], prefix+"UID_B64")
		if decodeErr != nil {
			return zero, decodeErr
		}
		fqdn, decodeErr := decodeShardingScaleInMemberFieldLine(
			lines[lineIndex+2], prefix+"FQDN_B64")
		if decodeErr != nil {
			return zero, decodeErr
		}
		agentImageID, decodeErr := decodeShardingScaleInMemberFieldLine(
			lines[lineIndex+3], prefix+"AGENT_IMAGE_ID_B64")
		if decodeErr != nil {
			return zero, decodeErr
		}
		agentProcessUID, decodeErr := decodeShardingScaleInMemberFieldLine(
			lines[lineIndex+4], prefix+"AGENT_PROCESS_UID_B64")
		if decodeErr != nil {
			return zero, decodeErr
		}
		agentCapabilityDigest, decodeErr := exactShardingScaleInMemberLine(
			lines[lineIndex+5], prefix+"AGENT_CAPABILITY_DIGEST")
		if decodeErr != nil {
			return zero, decodeErr
		}
		if decodeErr := validateShardingScaleInMemberDigest(
			"agent capability digest", agentCapabilityDigest); decodeErr != nil {
			return zero, decodeErr
		}
		member.Pods = append(member.Pods, ShardingScaleInMemberPod{
			Name:                  name,
			UID:                   uid,
			FQDN:                  fqdn,
			AgentImageID:          agentImageID,
			AgentProcessUID:       agentProcessUID,
			AgentCapabilityDigest: agentCapabilityDigest,
		})
		lineIndex += 6
	}

	memberDigest, err := exactShardingScaleInMemberLine(lines[lineIndex], "MEMBER_SHA256")
	if err != nil {
		return zero, err
	}
	if err := validateShardingScaleInMemberDigest("member digest", memberDigest); err != nil {
		return zero, err
	}
	body := strings.Join(lines[:lineIndex], "\n") + "\n"
	digest := sha256.Sum256([]byte(body))
	if memberDigest != hex.EncodeToString(digest[:]) {
		return zero, invalidShardingScaleInMemberRecordf("member digest mismatch")
	}
	if err := validateShardingScaleInMember(member); err != nil {
		return zero, err
	}

	return ShardingScaleInMemberRecord{
		Partition:      partition,
		PartitionIndex: uint32(partitionIndex),
		Member:         member,
	}, nil
}

func EncodeShardingScaleInMemberSet(value ShardingScaleInMemberSet) ([]string, error) {
	if err := validateShardingScaleInMemberSet(value); err != nil {
		return nil, err
	}

	count := len(value.Leaving) + len(value.Staying)
	lines := make([]string, 0, 1+count)
	lines = append(lines, "MEMBER_COUNT="+strconv.Itoa(count))
	appendPartition := func(
		partition ShardingScaleInMemberPartition,
		members []ShardingScaleInMember,
	) error {
		for index, member := range members {
			record, err := EncodeShardingScaleInMemberRecord(
				partition, uint32(index), member)
			if err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("MEMBER_%03d=%s",
				len(lines)-1, base64.StdEncoding.EncodeToString(record)))
		}
		return nil
	}
	if err := appendPartition(ShardingScaleInMemberPartitionLeaving, value.Leaving); err != nil {
		return nil, err
	}
	if err := appendPartition(ShardingScaleInMemberPartitionStaying, value.Staying); err != nil {
		return nil, err
	}
	return lines, nil
}

func DecodeShardingScaleInMemberSet(lines []string) (ShardingScaleInMemberSet, error) {
	var zero ShardingScaleInMemberSet
	if len(lines) == 0 {
		return zero, invalidShardingScaleInMemberRecordf("member set must not be empty")
	}
	count, err := parseCanonicalShardingScaleInMemberUint(
		lines[0], "MEMBER_COUNT",
		shardingScaleInMemberSetMinCount, shardingScaleInMemberSetMaxCount)
	if err != nil {
		return zero, err
	}
	if len(lines) != 1+int(count) {
		return zero, invalidShardingScaleInMemberRecordf(
			"member line count does not match MEMBER_COUNT")
	}

	result := ShardingScaleInMemberSet{}
	stayingStarted := false
	for index := 0; index < int(count); index++ {
		encoded, err := exactShardingScaleInMemberLine(
			lines[index+1], fmt.Sprintf("MEMBER_%03d", index))
		if err != nil {
			return zero, err
		}
		if len(encoded) > base64.StdEncoding.EncodedLen(shardingScaleInMemberRecordMaxBytes) {
			return zero, invalidShardingScaleInMemberRecordf(
				"encoded member exceeds its allowed size")
		}
		recordBytes, err := decodeCanonicalShardingScaleInMemberBase64(encoded)
		if err != nil {
			return zero, err
		}
		record, err := DecodeShardingScaleInMemberRecord(recordBytes)
		if err != nil {
			return zero, err
		}
		switch record.Partition {
		case ShardingScaleInMemberPartitionLeaving:
			if stayingStarted || record.PartitionIndex != uint32(len(result.Leaving)) {
				return zero, invalidShardingScaleInMemberRecordf(
					"noncanonical Leaving partition placement or index")
			}
			result.Leaving = append(result.Leaving, record.Member)
		case ShardingScaleInMemberPartitionStaying:
			stayingStarted = true
			if record.PartitionIndex != uint32(len(result.Staying)) {
				return zero, invalidShardingScaleInMemberRecordf(
					"noncanonical Staying partition index")
			}
			result.Staying = append(result.Staying, record.Member)
		default:
			return zero, invalidShardingScaleInMemberRecordf("invalid member partition")
		}
	}
	if err := validateShardingScaleInMemberSet(result); err != nil {
		return zero, err
	}
	return result, nil
}

func validateShardingScaleInMemberPartition(partition ShardingScaleInMemberPartition) error {
	switch partition {
	case ShardingScaleInMemberPartitionLeaving, ShardingScaleInMemberPartitionStaying:
		return nil
	default:
		return invalidShardingScaleInMemberRecordf("invalid member partition %q", partition)
	}
}

func validateShardingScaleInMember(member ShardingScaleInMember) error {
	for name, value := range map[string]string{
		"component name":       member.ComponentName,
		"component UID":        member.ComponentUID,
		"component short name": member.ComponentShortName,
		"shard template name":  member.ShardTemplateName,
	} {
		if err := validateShardingScaleInMemberField(name, value); err != nil {
			return err
		}
	}
	if member.ComponentGeneration == 0 || member.ComponentGeneration > math.MaxInt64 {
		return invalidShardingScaleInMemberRecordf(
			"component generation must be a positive int64")
	}
	if err := validateShardingScaleInMemberDigest(
		"component spec digest", member.ComponentSpecDigest); err != nil {
		return err
	}
	if len(member.Pods) < shardingScaleInMemberMinPodCount ||
		len(member.Pods) > shardingScaleInMemberMaxPodCount {
		return invalidShardingScaleInMemberRecordf("Pod count must be between %d and %d",
			shardingScaleInMemberMinPodCount, shardingScaleInMemberMaxPodCount)
	}
	for index, pod := range member.Pods {
		for name, value := range map[string]string{
			"Pod name":          pod.Name,
			"Pod UID":           pod.UID,
			"Pod FQDN":          pod.FQDN,
			"agent image ID":    pod.AgentImageID,
			"agent process UID": pod.AgentProcessUID,
		} {
			if err := validateShardingScaleInMemberField(name, value); err != nil {
				return err
			}
		}
		if err := validateShardingScaleInMemberDigest(
			"agent capability digest", pod.AgentCapabilityDigest); err != nil {
			return err
		}
		if index > 0 &&
			compareShardingScaleInMemberTuple(
				member.Pods[index-1].Name, member.Pods[index-1].UID,
				pod.Name, pod.UID) >= 0 {
			return invalidShardingScaleInMemberRecordf("Pod roster is not canonical")
		}
	}
	return nil
}

func validateShardingScaleInMemberSet(value ShardingScaleInMemberSet) error {
	if len(value.Leaving) == 0 || len(value.Staying) == 0 {
		return invalidShardingScaleInMemberRecordf(
			"Leaving and Staying member partitions must both be nonempty")
	}
	count := len(value.Leaving) + len(value.Staying)
	if count < shardingScaleInMemberSetMinCount || count > shardingScaleInMemberSetMaxCount {
		return invalidShardingScaleInMemberRecordf("member count must be between %d and %d",
			shardingScaleInMemberSetMinCount, shardingScaleInMemberSetMaxCount)
	}

	componentNames := map[string]struct{}{}
	componentUIDs := map[string]struct{}{}
	componentShortNames := map[string]struct{}{}
	podNames := map[string]struct{}{}
	podUIDs := map[string]struct{}{}
	podFQDNs := map[string]struct{}{}
	insertUnique := func(kind, value string, set map[string]struct{}) error {
		if _, ok := set[value]; ok {
			return invalidShardingScaleInMemberRecordf(
				"duplicate %s roster identity %q", kind, value)
		}
		set[value] = struct{}{}
		return nil
	}
	validatePartition := func(members []ShardingScaleInMember) error {
		for index, member := range members {
			if err := validateShardingScaleInMember(member); err != nil {
				return err
			}
			if index > 0 &&
				compareShardingScaleInMemberTuple(
					members[index-1].ComponentName, members[index-1].ComponentUID,
					member.ComponentName, member.ComponentUID) >= 0 {
				return invalidShardingScaleInMemberRecordf("Component roster is not canonical")
			}
			if err := insertUnique("Component name",
				member.ComponentName, componentNames); err != nil {
				return err
			}
			if err := insertUnique("Component UID",
				member.ComponentUID, componentUIDs); err != nil {
				return err
			}
			if err := insertUnique("Component short name",
				member.ComponentShortName, componentShortNames); err != nil {
				return err
			}
			for _, pod := range member.Pods {
				if err := insertUnique("Pod name", pod.Name, podNames); err != nil {
					return err
				}
				if err := insertUnique("Pod UID", pod.UID, podUIDs); err != nil {
					return err
				}
				if err := insertUnique("Pod FQDN", pod.FQDN, podFQDNs); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := validatePartition(value.Leaving); err != nil {
		return err
	}
	return validatePartition(value.Staying)
}

func validateShardingScaleInMemberField(name, value string) error {
	if value == "" || len(value) > ShardingScaleInMemberFieldMaxBytes ||
		!utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
		return invalidShardingScaleInMemberRecordf("%s is invalid", name)
	}
	return nil
}

func validateShardingScaleInMemberDigest(name, value string) error {
	if len(value) != sha256.Size*2 {
		return invalidShardingScaleInMemberRecordf("%s must be lowercase SHA-256", name)
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return invalidShardingScaleInMemberRecordf("%s must be lowercase SHA-256", name)
		}
	}
	return nil
}

func exactShardingScaleInMemberLine(line, key string) (string, error) {
	prefix := key + "="
	if !strings.HasPrefix(line, prefix) {
		return "", invalidShardingScaleInMemberRecordf("expected field %s", key)
	}
	return strings.TrimPrefix(line, prefix), nil
}

func parseCanonicalShardingScaleInMemberUint(
	line, key string,
	minimum, maximum uint64,
) (uint64, error) {
	value, err := exactShardingScaleInMemberLine(line, key)
	if err != nil {
		return 0, err
	}
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, invalidShardingScaleInMemberRecordf("%s is not a canonical integer", key)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, invalidShardingScaleInMemberRecordf("%s is not a canonical integer", key)
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, invalidShardingScaleInMemberRecordf("%s is outside its allowed range", key)
	}
	return parsed, nil
}

func encodeShardingScaleInMemberField(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

func decodeShardingScaleInMemberFieldLine(line, key string) (string, error) {
	encoded, err := exactShardingScaleInMemberLine(line, key)
	if err != nil {
		return "", err
	}
	decoded, err := decodeCanonicalShardingScaleInMemberBase64(encoded)
	if err != nil {
		return "", err
	}
	value := string(decoded)
	if err := validateShardingScaleInMemberField(key, value); err != nil {
		return "", err
	}
	return value, nil
}

func decodeCanonicalShardingScaleInMemberBase64(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, invalidShardingScaleInMemberRecordf("value is not canonical standard base64")
	}
	return decoded, nil
}

func compareShardingScaleInMemberTuple(
	leftName, leftUID, rightName, rightUID string,
) int {
	return strings.Compare(leftName+"\x00"+leftUID, rightName+"\x00"+rightUID)
}

func invalidShardingScaleInMemberRecordf(format string, args ...any) error {
	return fmt.Errorf("%w: %s",
		ErrInvalidShardingScaleInMemberRecord, fmt.Sprintf(format, args...))
}
