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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sharding scale-in envelope size preflight", func() {
	bytesOfSize := func(size uint64) []byte {
		return make([]byte, int(size))
	}
	validInput := func() shardingScaleInEnvelopeSizeInput {
		return shardingScaleInEnvelopeSizeInput{
			PlanMaterial:                 bytesOfSize(64),
			LargestLaunchRequest:         bytesOfSize(64),
			Base64ChunkPayloads:          [][]byte{bytesOfSize(128)},
			SerializedActionRequest:      bytesOfSize(256),
			WorstCaseScaleInStatus:       bytesOfSize(256),
			TypedLifecycleDispatchFences: bytesOfSize(256),
		}
	}

	It("accepts exact boundaries and reports the measured aggregate", func() {
		input := validInput()
		input.PlanMaterial = bytesOfSize(80 * 1024)
		input.CurrentReceipt = bytesOfSize(20 * 1024)
		input.CompletedHolderTombstones = [][]byte{
			bytesOfSize(10 * 1024),
			bytesOfSize(10 * 1024),
		}
		input.MutationBasis = bytesOfSize(20 * 1024)
		input.LargestLaunchRequest = bytesOfSize(40 * 1024)
		input.ReceiptCandidate = bytesOfSize(shardingScaleInDecodedEnvelopeLimitBytes)
		input.Base64ChunkPayloads = make([][]byte, shardingScaleInMaxEnvelopeChunkCount)
		for index := range input.Base64ChunkPayloads {
			input.Base64ChunkPayloads[index] =
				bytesOfSize(shardingScaleInMaxEnvelopeChunkPayloadBytes)
		}
		input.SerializedActionRequest =
			bytesOfSize(shardingScaleInSerializedActionRequestLimitBytes)
		input.WorstCaseScaleInStatus = bytesOfSize(320 * 1024)
		input.TypedLifecycleDispatchFences = bytesOfSize(192 * 1024)

		usage, err := validateShardingScaleInEnvelopeSize(input)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(usage.DecodedActionEnvelopeBytes).Should(Equal(
			shardingScaleInDecodedEnvelopeLimitBytes))
		Expect(usage.ReceiptCandidateBytes).Should(Equal(
			shardingScaleInDecodedEnvelopeLimitBytes))
		Expect(usage.Base64ChunkPayloadBytes).Should(Equal(
			shardingScaleInBase64PayloadLimitBytes))
		Expect(usage.SerializedActionRequestBytes).Should(Equal(
			shardingScaleInSerializedActionRequestLimitBytes))
		Expect(usage.WorstCaseStatusBytes).Should(Equal(
			shardingScaleInWorstCaseStatusLimitBytes))
	})

	It("rejects aggregate overflow even when each durable part is individually below the limit", func() {
		input := validInput()
		input.PlanMaterial = bytesOfSize(100 * 1024)
		input.CurrentReceipt = bytesOfSize(20 * 1024)
		input.CompletedHolderTombstones = [][]byte{bytesOfSize(10 * 1024)}
		input.MutationBasis = bytesOfSize(10 * 1024)
		input.LargestLaunchRequest = bytesOfSize(41 * 1024)

		usage, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(err).Should(MatchError(ContainSubstring("decoded action envelope")))
		Expect(usage.DecodedActionEnvelopeBytes).Should(Equal(uint64(181 * 1024)))
	})

	It("counts request-authority sources and every executor base through plan material", func() {
		input := validInput()
		planPrefix := bytesOfSize(40 * 1024)
		requestAuthoritySources := bytesOfSize(60 * 1024)
		executorBaseRecords := bytesOfSize(40 * 1024)
		input.PlanMaterial = append(planPrefix, requestAuthoritySources...)
		input.PlanMaterial = append(input.PlanMaterial, executorBaseRecords...)
		input.LargestLaunchRequest = bytesOfSize(41 * 1024)

		usage, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(usage.DecodedActionEnvelopeBytes).Should(Equal(uint64(181 * 1024)))
	})

	It("rejects an oversized receipt candidate independently", func() {
		input := validInput()
		input.ReceiptCandidate = bytesOfSize(shardingScaleInDecodedEnvelopeLimitBytes + 1)

		usage, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(err).Should(MatchError(ContainSubstring("receipt candidate")))
		Expect(usage.ReceiptCandidateBytes).Should(Equal(
			shardingScaleInDecodedEnvelopeLimitBytes + 1))
	})

	It("rejects an oversized base64 chunk", func() {
		input := validInput()
		input.Base64ChunkPayloads = [][]byte{
			bytesOfSize(shardingScaleInMaxEnvelopeChunkPayloadBytes + 1),
		}

		_, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(err).Should(MatchError(ContainSubstring("base64 chunk 0")))
	})

	It("rejects too many base64 chunks", func() {
		input := validInput()
		input.Base64ChunkPayloads =
			make([][]byte, shardingScaleInMaxEnvelopeChunkCount+1)
		for index := range input.Base64ChunkPayloads {
			input.Base64ChunkPayloads[index] = bytesOfSize(1)
		}

		_, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(err).Should(MatchError(ContainSubstring("base64 chunk count")))
	})

	It("rejects an oversized serialized action request", func() {
		input := validInput()
		input.SerializedActionRequest =
			bytesOfSize(shardingScaleInSerializedActionRequestLimitBytes + 1)

		usage, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(err).Should(MatchError(ContainSubstring("serialized action request")))
		Expect(usage.SerializedActionRequestBytes).Should(Equal(
			shardingScaleInSerializedActionRequestLimitBytes + 1))
	})

	It("rejects aggregate status overflow when each status part is individually below the limit", func() {
		input := validInput()
		input.WorstCaseScaleInStatus = bytesOfSize(320 * 1024)
		input.TypedLifecycleDispatchFences = bytesOfSize(192*1024 + 1)

		usage, err := validateShardingScaleInEnvelopeSize(input)
		Expect(errors.Is(err, errShardingScaleInPlanEnvelopeTooLarge)).Should(BeTrue())
		Expect(err).Should(MatchError(ContainSubstring("worst-case status")))
		Expect(usage.WorstCaseStatusBytes).Should(Equal(
			shardingScaleInWorstCaseStatusLimitBytes + 1))
	})

	DescribeTable("rejects an incomplete preflight measurement",
		func(mutate func(*shardingScaleInEnvelopeSizeInput), message string) {
			input := validInput()
			mutate(&input)

			_, err := validateShardingScaleInEnvelopeSize(input)
			Expect(errors.Is(err, errInvalidShardingScaleInEnvelopeSize)).Should(BeTrue())
			Expect(err).Should(MatchError(ContainSubstring(message)))
		},
		Entry("missing plan material",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.PlanMaterial = nil
			},
			"plan material"),
		Entry("missing largest launch request",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.LargestLaunchRequest = nil
			},
			"largest launch request"),
		Entry("missing base64 chunks",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.Base64ChunkPayloads = nil
			},
			"base64 chunk payloads"),
		Entry("empty base64 chunk",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.Base64ChunkPayloads = [][]byte{nil}
			},
			"base64 chunk 0"),
		Entry("missing serialized request",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.SerializedActionRequest = nil
			},
			"serialized action request"),
		Entry("missing worst-case status",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.WorstCaseScaleInStatus = nil
			},
			"worst-case scale-in status"),
		Entry("missing dispatch fences",
			func(input *shardingScaleInEnvelopeSizeInput) {
				input.TypedLifecycleDispatchFences = nil
			},
			"typed lifecycle dispatch fences"),
	)
})
