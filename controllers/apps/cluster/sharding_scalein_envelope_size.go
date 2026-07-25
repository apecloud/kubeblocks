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

package cluster

import (
	"errors"
	"fmt"
	"math"
)

const (
	shardingScaleInDecodedEnvelopeLimitBytes         uint64 = 180 * 1024
	shardingScaleInBase64PayloadLimitBytes           uint64 = 240 * 1024
	shardingScaleInSerializedActionRequestLimitBytes uint64 = 256 * 1024
	shardingScaleInWorstCaseStatusLimitBytes         uint64 = 512 * 1024
	shardingScaleInMaxEnvelopeChunkPayloadBytes      uint64 = 16 * 1024
	shardingScaleInMaxEnvelopeChunkCount                    = 15
)

var (
	errInvalidShardingScaleInEnvelopeSize = errors.New(
		"invalid sharding scale-in envelope size preflight")
	errShardingScaleInPlanEnvelopeTooLarge = errors.New(
		"sharding scale-in plan envelope too large")
)

// shardingScaleInEnvelopeSizeInput contains the exact serialized bytes that
// the caller intends to persist or dispatch. PlanMaterial must already contain
// the complete request-authority sources and all executor base records. This
// size-only contract treats nil and empty optional parts as zero bytes; their
// semantic absent/bound validation belongs to the typed material builders.
type shardingScaleInEnvelopeSizeInput struct {
	PlanMaterial                 []byte
	CurrentReceipt               []byte
	CompletedHolderTombstones    [][]byte
	MutationBasis                []byte
	LargestLaunchRequest         []byte
	ReceiptCandidate             []byte
	Base64ChunkPayloads          [][]byte
	SerializedActionRequest      []byte
	WorstCaseScaleInStatus       []byte
	TypedLifecycleDispatchFences []byte
}

type shardingScaleInEnvelopeSizeUsage struct {
	DecodedActionEnvelopeBytes   uint64
	ReceiptCandidateBytes        uint64
	Base64ChunkPayloadBytes      uint64
	SerializedActionRequestBytes uint64
	WorstCaseStatusBytes         uint64
}

func validateShardingScaleInEnvelopeSize(
	input shardingScaleInEnvelopeSizeInput,
) (shardingScaleInEnvelopeSizeUsage, error) {
	var usage shardingScaleInEnvelopeSizeUsage
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInEnvelopeSize,
			fmt.Sprintf(format, args...))
	}
	tooLarge := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errShardingScaleInPlanEnvelopeTooLarge,
			fmt.Sprintf(format, args...))
	}

	switch {
	case len(input.PlanMaterial) == 0:
		return usage, invalid("plan material must not be empty")
	case len(input.LargestLaunchRequest) == 0:
		return usage, invalid("largest launch request must not be empty")
	case len(input.Base64ChunkPayloads) == 0:
		return usage, invalid("base64 chunk payloads must not be empty")
	case len(input.SerializedActionRequest) == 0:
		return usage, invalid("serialized action request must not be empty")
	case len(input.WorstCaseScaleInStatus) == 0:
		return usage, invalid("worst-case scale-in status must not be empty")
	case len(input.TypedLifecycleDispatchFences) == 0:
		return usage, invalid("typed lifecycle dispatch fences must not be empty")
	}
	if len(input.Base64ChunkPayloads) > shardingScaleInMaxEnvelopeChunkCount {
		return usage, tooLarge("base64 chunk count %d exceeds %d",
			len(input.Base64ChunkPayloads), shardingScaleInMaxEnvelopeChunkCount)
	}

	decodedParts := make([][]byte, 0, 4+len(input.CompletedHolderTombstones))
	decodedParts = append(decodedParts,
		input.PlanMaterial,
		input.CurrentReceipt,
	)
	decodedParts = append(decodedParts, input.CompletedHolderTombstones...)
	decodedParts = append(decodedParts,
		input.MutationBasis,
		input.LargestLaunchRequest,
	)
	var overflow bool
	usage.DecodedActionEnvelopeBytes, overflow = sumShardingScaleInSerializedBytes(decodedParts)
	if overflow ||
		usage.DecodedActionEnvelopeBytes > shardingScaleInDecodedEnvelopeLimitBytes {
		return usage, tooLarge("decoded action envelope is %d bytes, limit is %d",
			usage.DecodedActionEnvelopeBytes, shardingScaleInDecodedEnvelopeLimitBytes)
	}

	usage.ReceiptCandidateBytes = uint64(len(input.ReceiptCandidate))
	if usage.ReceiptCandidateBytes > shardingScaleInDecodedEnvelopeLimitBytes {
		return usage, tooLarge("receipt candidate is %d bytes, limit is %d",
			usage.ReceiptCandidateBytes, shardingScaleInDecodedEnvelopeLimitBytes)
	}

	for index, chunk := range input.Base64ChunkPayloads {
		if len(chunk) == 0 {
			return usage, invalid("base64 chunk %d must not be empty", index)
		}
		if uint64(len(chunk)) > shardingScaleInMaxEnvelopeChunkPayloadBytes {
			return usage, tooLarge("base64 chunk %d is %d bytes, limit is %d",
				index, len(chunk), shardingScaleInMaxEnvelopeChunkPayloadBytes)
		}
	}
	usage.Base64ChunkPayloadBytes, overflow =
		sumShardingScaleInSerializedBytes(input.Base64ChunkPayloads)
	if overflow ||
		usage.Base64ChunkPayloadBytes > shardingScaleInBase64PayloadLimitBytes {
		return usage, tooLarge("base64 chunk payload aggregate is %d bytes, limit is %d",
			usage.Base64ChunkPayloadBytes, shardingScaleInBase64PayloadLimitBytes)
	}

	usage.SerializedActionRequestBytes = uint64(len(input.SerializedActionRequest))
	if usage.SerializedActionRequestBytes >
		shardingScaleInSerializedActionRequestLimitBytes {
		return usage, tooLarge("serialized action request is %d bytes, limit is %d",
			usage.SerializedActionRequestBytes,
			shardingScaleInSerializedActionRequestLimitBytes)
	}

	usage.WorstCaseStatusBytes, overflow = sumShardingScaleInSerializedBytes([][]byte{
		input.WorstCaseScaleInStatus,
		input.TypedLifecycleDispatchFences,
	})
	if overflow || usage.WorstCaseStatusBytes > shardingScaleInWorstCaseStatusLimitBytes {
		return usage, tooLarge("worst-case status is %d bytes, limit is %d",
			usage.WorstCaseStatusBytes, shardingScaleInWorstCaseStatusLimitBytes)
	}
	return usage, nil
}

func sumShardingScaleInSerializedBytes(parts [][]byte) (uint64, bool) {
	var total uint64
	for _, part := range parts {
		size := uint64(len(part))
		if size > math.MaxUint64-total {
			return math.MaxUint64, true
		}
		total += size
	}
	return total, false
}
