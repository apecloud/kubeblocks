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
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	shardingScaleInEnvelopeVersionV2 = "KB-SHARD-SCALE-IN-ENVELOPE/2"

	shardingScaleInHolderParameterName    = "KB_REMOVE_SHARD_NAME"
	shardingScaleInHolderSourceVersion    = "kb.sharding.scalein.holder-target/v1"
	shardingScaleInHolderValueVersion     = "kb.sharding.scalein.holder-target-value/v1"
	shardingScaleInEnvelopeMaxBytes       = 180 * 1024
	shardingScaleInEnvelopeFixedLineCount = 17
)

var ErrInvalidShardingScaleInEnvelope = errors.New(
	"invalid sharding scale-in envelope")

type ShardingScaleInHolderTarget struct {
	ParameterName      string
	HolderIndex        uint32
	ComponentName      string
	ComponentUID       string
	ComponentShortName string
	SourceDigest       string
	ValueB64           string
	ValueDigest        string
}

type ShardingScaleInEnvelopeV2 struct {
	PlanID                 string
	TopologyFenceToken     string
	RequestAuthorityDigest string
	BaseParameterDigest    string
	Phase                  string
	Holder                 ShardingScaleInHolderTarget
	Members                ShardingScaleInMemberSet
	ReceiptID              string
}

func EncodeShardingScaleInEnvelopeV2(value ShardingScaleInEnvelopeV2) ([]byte, error) {
	if err := validateShardingScaleInEnvelopeV2(value); err != nil {
		return nil, err
	}
	encoded, err := encodeShardingScaleInEnvelopeV2Unchecked(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) > shardingScaleInEnvelopeMaxBytes {
		return nil, invalidShardingScaleInEnvelopef(
			"encoded envelope exceeds %d bytes", shardingScaleInEnvelopeMaxBytes)
	}

	decoded, err := DecodeShardingScaleInEnvelopeV2(encoded)
	if err != nil {
		return nil, invalidShardingScaleInEnvelopef(
			"self-parse failed: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		return nil, invalidShardingScaleInEnvelopef(
			"self-parse round trip mismatch")
	}
	return encoded, nil
}

func DecodeShardingScaleInEnvelopeV2(value []byte) (ShardingScaleInEnvelopeV2, error) {
	var zero ShardingScaleInEnvelopeV2
	if len(value) == 0 || len(value) > shardingScaleInEnvelopeMaxBytes {
		return zero, invalidShardingScaleInEnvelopef(
			"envelope size must be between 1 and %d bytes",
			shardingScaleInEnvelopeMaxBytes)
	}
	if value[len(value)-1] != '\n' ||
		strings.ContainsRune(string(value), '\r') ||
		strings.ContainsRune(string(value), '\x00') ||
		strings.HasPrefix(string(value), "\ufeff") {
		return zero, invalidShardingScaleInEnvelopef("invalid envelope framing")
	}

	lines := strings.Split(string(value), "\n")
	if len(lines) < shardingScaleInEnvelopeFixedLineCount+2 ||
		lines[len(lines)-1] != "" {
		return zero, invalidShardingScaleInEnvelopef(
			"invalid envelope line framing")
	}
	lines = lines[:len(lines)-1]
	if lines[0] != shardingScaleInEnvelopeVersionV2 {
		return zero, invalidShardingScaleInEnvelopef(
			"unsupported envelope version")
	}

	planID, err := exactShardingScaleInEnvelopeLine(lines[1], "PLAN_ID")
	if err != nil {
		return zero, err
	}
	topologyFenceToken, err := exactShardingScaleInEnvelopeLine(
		lines[2], "TOPOLOGY_FENCE_TOKEN")
	if err != nil {
		return zero, err
	}
	requestAuthorityDigest, err := exactShardingScaleInEnvelopeLine(
		lines[3], "REQUEST_AUTHORITY_DIGEST")
	if err != nil {
		return zero, err
	}
	baseParameterDigest, err := exactShardingScaleInEnvelopeLine(
		lines[4], "BASE_PARAMETER_DIGEST")
	if err != nil {
		return zero, err
	}
	phase, err := exactShardingScaleInEnvelopeLine(lines[5], "PHASE")
	if err != nil {
		return zero, err
	}
	parameterName, err := exactShardingScaleInEnvelopeLine(
		lines[6], "HOLDER_PARAMETER_NAME")
	if err != nil {
		return zero, err
	}
	holderIndex, err := parseCanonicalShardingScaleInEnvelopeUint(
		lines[7], "HOLDER_INDEX", 0, math.MaxUint32)
	if err != nil {
		return zero, err
	}
	componentName, err := decodeShardingScaleInEnvelopeFieldLine(
		lines[8], "HOLDER_COMPONENT_NAME_B64")
	if err != nil {
		return zero, err
	}
	componentUID, err := decodeShardingScaleInEnvelopeFieldLine(
		lines[9], "HOLDER_COMPONENT_UID_B64")
	if err != nil {
		return zero, err
	}
	componentShortName, err := decodeShardingScaleInEnvelopeFieldLine(
		lines[10], "HOLDER_COMPONENT_SHORT_NAME_B64")
	if err != nil {
		return zero, err
	}
	sourceDigest, err := exactShardingScaleInEnvelopeLine(
		lines[11], "HOLDER_TARGET_SOURCE_DIGEST")
	if err != nil {
		return zero, err
	}
	valueB64, err := exactShardingScaleInEnvelopeLine(
		lines[12], "HOLDER_TARGET_VALUE_B64")
	if err != nil {
		return zero, err
	}
	if _, err := decodeCanonicalShardingScaleInEnvelopeBase64(valueB64); err != nil {
		return zero, err
	}
	valueDigest, err := exactShardingScaleInEnvelopeLine(
		lines[13], "HOLDER_TARGET_VALUE_DIGEST")
	if err != nil {
		return zero, err
	}
	memberCount, err := parseCanonicalShardingScaleInEnvelopeUint(
		lines[14], "MEMBER_COUNT",
		shardingScaleInMemberSetMinCount, shardingScaleInMemberSetMaxCount)
	if err != nil {
		return zero, err
	}
	if len(lines) != shardingScaleInEnvelopeFixedLineCount+int(memberCount) {
		return zero, invalidShardingScaleInEnvelopef(
			"envelope line count does not match MEMBER_COUNT")
	}

	memberEnd := 15 + int(memberCount)
	members, err := DecodeShardingScaleInMemberSet(lines[14:memberEnd])
	if err != nil {
		return zero, invalidShardingScaleInEnvelopef(
			"member set is invalid: %v", err)
	}
	receiptID, err := exactShardingScaleInEnvelopeLine(
		lines[memberEnd], "RECEIPT_ID")
	if err != nil {
		return zero, err
	}
	envelopeDigest, err := exactShardingScaleInEnvelopeLine(
		lines[memberEnd+1], "ENVELOPE_SHA256")
	if err != nil {
		return zero, err
	}
	if !isShardingScaleInEnvelopeSHA256(envelopeDigest) {
		return zero, invalidShardingScaleInEnvelopef(
			"envelope digest must be lowercase SHA-256")
	}
	body := strings.Join(lines[:memberEnd+1], "\n") + "\n"
	digest := sha256.Sum256([]byte(body))
	if envelopeDigest != hex.EncodeToString(digest[:]) {
		return zero, invalidShardingScaleInEnvelopef(
			"envelope digest mismatch")
	}

	decoded := ShardingScaleInEnvelopeV2{
		PlanID:                 planID,
		TopologyFenceToken:     topologyFenceToken,
		RequestAuthorityDigest: requestAuthorityDigest,
		BaseParameterDigest:    baseParameterDigest,
		Phase:                  phase,
		Holder: ShardingScaleInHolderTarget{
			ParameterName:      parameterName,
			HolderIndex:        uint32(holderIndex),
			ComponentName:      componentName,
			ComponentUID:       componentUID,
			ComponentShortName: componentShortName,
			SourceDigest:       sourceDigest,
			ValueB64:           valueB64,
			ValueDigest:        valueDigest,
		},
		Members:   members,
		ReceiptID: receiptID,
	}
	if err := validateShardingScaleInEnvelopeV2(decoded); err != nil {
		return zero, err
	}
	canonical, err := encodeShardingScaleInEnvelopeV2Unchecked(decoded)
	if err != nil {
		return zero, err
	}
	if !reflect.DeepEqual(canonical, value) {
		return zero, invalidShardingScaleInEnvelopef(
			"envelope is not canonical")
	}
	return decoded, nil
}

func encodeShardingScaleInEnvelopeV2Unchecked(
	value ShardingScaleInEnvelopeV2,
) ([]byte, error) {
	memberLines, err := EncodeShardingScaleInMemberSet(value.Members)
	if err != nil {
		return nil, invalidShardingScaleInEnvelopef(
			"member set is invalid: %v", err)
	}
	lines := []string{
		shardingScaleInEnvelopeVersionV2,
		"PLAN_ID=" + value.PlanID,
		"TOPOLOGY_FENCE_TOKEN=" + value.TopologyFenceToken,
		"REQUEST_AUTHORITY_DIGEST=" + value.RequestAuthorityDigest,
		"BASE_PARAMETER_DIGEST=" + value.BaseParameterDigest,
		"PHASE=" + value.Phase,
		"HOLDER_PARAMETER_NAME=" + value.Holder.ParameterName,
		"HOLDER_INDEX=" + strconv.FormatUint(uint64(value.Holder.HolderIndex), 10),
		"HOLDER_COMPONENT_NAME_B64=" +
			base64.StdEncoding.EncodeToString([]byte(value.Holder.ComponentName)),
		"HOLDER_COMPONENT_UID_B64=" +
			base64.StdEncoding.EncodeToString([]byte(value.Holder.ComponentUID)),
		"HOLDER_COMPONENT_SHORT_NAME_B64=" +
			base64.StdEncoding.EncodeToString([]byte(value.Holder.ComponentShortName)),
		"HOLDER_TARGET_SOURCE_DIGEST=" + value.Holder.SourceDigest,
		"HOLDER_TARGET_VALUE_B64=" + value.Holder.ValueB64,
		"HOLDER_TARGET_VALUE_DIGEST=" + value.Holder.ValueDigest,
	}
	lines = append(lines, memberLines...)
	lines = append(lines, "RECEIPT_ID="+value.ReceiptID)
	body := strings.Join(lines, "\n") + "\n"
	digest := sha256.Sum256([]byte(body))
	return []byte(body + "ENVELOPE_SHA256=" +
		hex.EncodeToString(digest[:]) + "\n"), nil
}

func validateShardingScaleInEnvelopeV2(value ShardingScaleInEnvelopeV2) error {
	for name, digest := range map[string]string{
		"plan ID":                  value.PlanID,
		"topology fence token":     value.TopologyFenceToken,
		"request authority digest": value.RequestAuthorityDigest,
		"base parameter digest":    value.BaseParameterDigest,
	} {
		if !isShardingScaleInEnvelopeSHA256(digest) {
			return invalidShardingScaleInEnvelopef(
				"%s must be lowercase SHA-256", name)
		}
	}
	if !isShardingScaleInEnvelopePhase(value.Phase) {
		return invalidShardingScaleInEnvelopef(
			"phase is not dispatchable")
	}
	if value.Phase == "Draining" {
		if value.ReceiptID != "" {
			return invalidShardingScaleInEnvelopef(
				"Draining must not bind a receipt")
		}
	} else if !isShardingScaleInEnvelopeSHA256(value.ReceiptID) {
		return invalidShardingScaleInEnvelopef(
			"%s must bind a receipt", value.Phase)
	}

	if _, err := EncodeShardingScaleInMemberSet(value.Members); err != nil {
		return invalidShardingScaleInEnvelopef(
			"member set is invalid: %v", err)
	}
	holder := value.Holder
	if holder.ParameterName != shardingScaleInHolderParameterName {
		return invalidShardingScaleInEnvelopef(
			"holder parameter name is invalid")
	}
	if int(holder.HolderIndex) >= len(value.Members.Leaving) {
		return invalidShardingScaleInEnvelopef(
			"holder index is outside Leaving")
	}
	for name, field := range map[string]string{
		"holder component name":       holder.ComponentName,
		"holder component UID":        holder.ComponentUID,
		"holder component short name": holder.ComponentShortName,
	} {
		if err := validateShardingScaleInEnvelopeField(name, field); err != nil {
			return err
		}
	}
	target := value.Members.Leaving[holder.HolderIndex]
	if holder.ComponentName != target.ComponentName ||
		holder.ComponentUID != target.ComponentUID ||
		holder.ComponentShortName != target.ComponentShortName {
		return invalidShardingScaleInEnvelopef(
			"holder identity does not match Leaving at holder index")
	}
	if !isShardingScaleInEnvelopeSHA256(holder.SourceDigest) ||
		!isShardingScaleInEnvelopeSHA256(holder.ValueDigest) {
		return invalidShardingScaleInEnvelopef(
			"holder digests must be lowercase SHA-256")
	}
	expectedSourceDigest, err := digestShardingScaleInHolderSource(
		value.PlanID, holder)
	if err != nil {
		return err
	}
	if holder.SourceDigest != expectedSourceDigest {
		return invalidShardingScaleInEnvelopef(
			"holder source digest mismatch")
	}
	decodedValue, err := decodeCanonicalShardingScaleInEnvelopeBase64(
		holder.ValueB64)
	if err != nil {
		return err
	}
	if string(decodedValue) != holder.ComponentName {
		return invalidShardingScaleInEnvelopef(
			"holder target value does not match holder component name")
	}
	expectedValueDigest, err := digestShardingScaleInHolderValue(
		holder.ValueB64)
	if err != nil {
		return err
	}
	if holder.ValueDigest != expectedValueDigest {
		return invalidShardingScaleInEnvelopef(
			"holder value digest mismatch")
	}
	return nil
}

func digestShardingScaleInHolderSource(
	planID string,
	holder ShardingScaleInHolderTarget,
) (string, error) {
	return digestShardingScaleInEnvelopeJSON(struct {
		Version            string `json:"version"`
		PlanID             string `json:"planID"`
		HolderIndex        int32  `json:"holderIndex"`
		ComponentName      string `json:"componentName"`
		ComponentUID       string `json:"componentUID"`
		ComponentShortName string `json:"componentShortName"`
	}{
		Version:            shardingScaleInHolderSourceVersion,
		PlanID:             planID,
		HolderIndex:        int32(holder.HolderIndex),
		ComponentName:      holder.ComponentName,
		ComponentUID:       holder.ComponentUID,
		ComponentShortName: holder.ComponentShortName,
	})
}

func digestShardingScaleInHolderValue(valueB64 string) (string, error) {
	return digestShardingScaleInEnvelopeJSON(struct {
		Version  string `json:"version"`
		ValueB64 string `json:"valueB64"`
	}{
		Version:  shardingScaleInHolderValueVersion,
		ValueB64: valueB64,
	})
}

func digestShardingScaleInEnvelopeJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", invalidShardingScaleInEnvelopef(
			"canonical JSON failed: %v", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func isShardingScaleInEnvelopePhase(value string) bool {
	switch value {
	case "Draining", "PurgePrepared", "Resetting", "Forgetting", "Verified":
		return true
	default:
		return false
	}
}

func isShardingScaleInEnvelopeSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validateShardingScaleInEnvelopeField(name, value string) error {
	if value == "" || len(value) > ShardingScaleInMemberFieldMaxBytes ||
		!utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
		return invalidShardingScaleInEnvelopef("%s is invalid", name)
	}
	return nil
}

func decodeShardingScaleInEnvelopeFieldLine(
	line, key string,
) (string, error) {
	encoded, err := exactShardingScaleInEnvelopeLine(line, key)
	if err != nil {
		return "", err
	}
	decoded, err := decodeCanonicalShardingScaleInEnvelopeBase64(encoded)
	if err != nil {
		return "", err
	}
	value := string(decoded)
	if err := validateShardingScaleInEnvelopeField(key, value); err != nil {
		return "", err
	}
	return value, nil
}

func decodeCanonicalShardingScaleInEnvelopeBase64(
	value string,
) ([]byte, error) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, invalidShardingScaleInEnvelopef(
			"value is not canonical standard base64")
	}
	return decoded, nil
}

func exactShardingScaleInEnvelopeLine(line, key string) (string, error) {
	prefix := key + "="
	if !strings.HasPrefix(line, prefix) {
		return "", invalidShardingScaleInEnvelopef(
			"expected field %s", key)
	}
	return strings.TrimPrefix(line, prefix), nil
}

func parseCanonicalShardingScaleInEnvelopeUint(
	line, key string,
	minimum, maximum uint64,
) (uint64, error) {
	value, err := exactShardingScaleInEnvelopeLine(line, key)
	if err != nil {
		return 0, err
	}
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, invalidShardingScaleInEnvelopef(
			"%s is not a canonical integer", key)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, invalidShardingScaleInEnvelopef(
				"%s is not a canonical integer", key)
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, invalidShardingScaleInEnvelopef(
			"%s is outside its allowed range", key)
	}
	return parsed, nil
}

func invalidShardingScaleInEnvelopef(format string, args ...any) error {
	return fmt.Errorf("%w: %s",
		ErrInvalidShardingScaleInEnvelope, fmt.Sprintf(format, args...))
}
