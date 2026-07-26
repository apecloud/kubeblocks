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
	"encoding/base64"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("sharding scale-in holder request material", func() {
	newCanonicalPlan := func() (*appsv1.ShardingScaleInPlanMaterial, string) {
		material, planID, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		return material, planID
	}
	newInput := func() shardingScaleInPreparedRequestInput {
		return shardingScaleInPreparedRequestInput{
			HolderIndex:              0,
			TopologyFenceToken:       shardingScaleInTestDigestB,
			Phase:                    appsv1.ShardingScaleInPhaseDraining,
			ResultRevision:           7,
			StepKey:                  "drain/7",
			ClaimClass:               shardingScaleInClaimClassMayMutate,
			ExecutorPodUID:           "pod-0",
			ExecutorDispatchSequence: 1,
		}
	}
	receipt := func(id string) *string {
		return &id
	}
	setResetTuple := func(input *shardingScaleInPreparedRequestInput, receiptID string,
		cursor string, podUID string,
	) {
		input.Phase = appsv1.ShardingScaleInPhaseResetting
		input.ReceiptID = receipt(receiptID)
		input.StepKey = "reset/" + cursor + "/" + podUID +
			"/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		input.ClaimClass = shardingScaleInClaimClassMayMutate
		input.ExecutorPodUID = types.UID(podUID)
	}

	It("derives a deterministic target, envelope, payload, and execution identity", func() {
		material, planID := newCanonicalPlan()
		input := newInput()

		first, err := buildShardingScaleInPreparedRequest(material, planID, input)
		Expect(err).ShouldNot(HaveOccurred())
		for range 100 {
			next, err := buildShardingScaleInPreparedRequest(material, planID, input)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(next).Should(Equal(first))
		}

		Expect(first.HolderTarget.ParameterName).Should(Equal(shardingRemoveShardNameVar))
		Expect(first.HolderTarget.HolderIndex).Should(Equal(int32(0)))
		Expect(first.HolderTarget.ComponentName).Should(Equal("demo-redis-2"))
		Expect(first.HolderTarget.ComponentUID).Should(Equal(material.Leaving[0].ComponentUID))
		Expect(first.HolderTarget.ComponentShortName).Should(Equal(
			material.Leaving[0].ComponentShortName))
		value, err := base64.StdEncoding.Strict().DecodeString(first.HolderTarget.ValueB64)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(value)).Should(Equal(material.Leaving[0].ComponentName))
		Expect(first.HolderTarget.SourceDigest).Should(Satisfy(isShardingScaleInSHA256))
		Expect(first.HolderTarget.ValueDigest).Should(Satisfy(isShardingScaleInSHA256))
		Expect(first.EnvelopeDigest).Should(Satisfy(isShardingScaleInSHA256))
		Expect(first.RequestPayloadDigest).Should(Satisfy(isShardingScaleInSHA256))
		Expect(first.ExecutionID).Should(Satisfy(isShardingScaleInSHA256))
		Expect(first.Envelope.EffectClass).Should(Equal(shardingScaleInEffectClassMayMutate))
		Expect(first.RequestPayload.EffectClass).Should(Equal(shardingScaleInEffectClassMayMutate))
		Expect(first.Envelope.TopologyFenceToken).Should(Equal(input.TopologyFenceToken))
		Expect(first.RequestPayload.Envelope.TopologyFenceToken).Should(Equal(
			input.TopologyFenceToken))

		envelopeDigest, err := digestShardingScaleInCanonicalJSON(first.Envelope)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(first.EnvelopeDigest).Should(Equal(envelopeDigest))
		payloadDigest, err := digestShardingScaleInCanonicalJSON(first.RequestPayload)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(first.RequestPayloadDigest).Should(Equal(payloadDigest))
		executionID, err := digestShardingScaleInCanonicalJSON(shardingScaleInExecutionIDMaterial{
			Version:                  shardingScaleInExecutionIDVersionV2,
			PlanID:                   planID,
			HolderIndex:              input.HolderIndex,
			Phase:                    input.Phase,
			ResultRevision:           input.ResultRevision,
			ReceiptID:                first.RequestPayload.ReceiptID,
			StepKey:                  input.StepKey,
			ExecutorPodUID:           first.RequestPayload.Executor.PodUID,
			ExecutorAgentProcessUID:  first.RequestPayload.Executor.AgentProcessUID,
			ExecutorDispatchSequence: input.ExecutorDispatchSequence,
			RequestPayloadDigest:     first.RequestPayloadDigest,
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(first.ExecutionID).Should(Equal(executionID))

		decodedBase, err := base64.StdEncoding.Strict().DecodeString(
			first.RequestPayload.BaseParameterRecordB64)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(decodedBase)).ShouldNot(ContainSubstring(shardingRemoveShardNameVar))
		serialized, err := json.Marshal(first.RequestPayload)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(serialized)).Should(ContainSubstring(shardingRemoveShardNameVar))
		for _, forbiddenField := range []string{
			"requestPayloadDigest",
			"executionID",
			"launchRequestDigest",
			"operation",
			"authorization",
			"diagnostics",
		} {
			Expect(string(serialized)).ShouldNot(ContainSubstring(`"` + forbiddenField + `"`))
		}
	})

	It("keeps one executor base stable while separating two holders", func() {
		raw := newShardingScaleInPlanMaterialFixture()
		raw.Source.DesiredShards = 1
		raw.Leaving = append(raw.Leaving, raw.Staying[0])
		raw.Staying = raw.Staying[1:]
		material, planID, err := buildShardingScaleInPlanMaterial(raw)
		Expect(err).ShouldNot(HaveOccurred())

		firstInput := newInput()
		secondInput := firstInput
		secondInput.HolderIndex = 1
		secondInput.StepKey = "drain/8"
		secondInput.ResultRevision = 8
		secondInput.ExecutorDispatchSequence = 2

		first, err := buildShardingScaleInPreparedRequest(material, planID, firstInput)
		Expect(err).ShouldNot(HaveOccurred())
		second, err := buildShardingScaleInPreparedRequest(material, planID, secondInput)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(second.RequestPayload.BaseParameterRecordB64).Should(Equal(
			first.RequestPayload.BaseParameterRecordB64))
		Expect(second.RequestPayload.BaseParameterDigest).Should(Equal(
			first.RequestPayload.BaseParameterDigest))
		Expect(second.HolderTarget.ComponentUID).ShouldNot(Equal(first.HolderTarget.ComponentUID))
		Expect(second.HolderTarget.SourceDigest).ShouldNot(Equal(first.HolderTarget.SourceDigest))
		Expect(second.EnvelopeDigest).ShouldNot(Equal(first.EnvelopeDigest))
		Expect(second.RequestPayloadDigest).ShouldNot(Equal(first.RequestPayloadDigest))
		Expect(second.ExecutionID).ShouldNot(Equal(first.ExecutionID))
	})

	DescribeTable("binds every execution input into the request and execution digests",
		func(mutate func(*shardingScaleInPreparedRequestInput,
			*shardingScaleInPreparedRequestInput),
		) {
			material, planID := newCanonicalPlan()
			baselineInput := newInput()
			changedInput := newInput()
			mutate(&baselineInput, &changedInput)
			baseline, err := buildShardingScaleInPreparedRequest(
				material, planID, baselineInput)
			Expect(err).ShouldNot(HaveOccurred())
			changed, err := buildShardingScaleInPreparedRequest(
				material, planID, changedInput)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(changed.RequestPayloadDigest).ShouldNot(Equal(baseline.RequestPayloadDigest))
			Expect(changed.ExecutionID).ShouldNot(Equal(baseline.ExecutionID))
		},
		Entry("phase and its typed tuple", func(_ *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			setResetTuple(changed, shardingScaleInTestDigestA, "0", "pod-0")
		}),
		Entry("topology fence token", func(_ *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			changed.TopologyFenceToken = shardingScaleInTestDigestA
		}),
		Entry("result revision and its typed step", func(_ *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			changed.ResultRevision++
			changed.StepKey = "drain/8"
		}),
		Entry("receipt", func(baseline *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			setResetTuple(baseline, shardingScaleInTestDigestA, "0", "pod-0")
			setResetTuple(changed, shardingScaleInTestDigestB, "0", "pod-0")
		}),
		Entry("step key", func(baseline *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			setResetTuple(baseline, shardingScaleInTestDigestA, "0", "pod-0")
			setResetTuple(changed, shardingScaleInTestDigestA, "1", "pod-0")
		}),
		Entry("claim class and its typed step", func(baseline *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			baseline.Phase = appsv1.ShardingScaleInPhaseForgetting
			baseline.ReceiptID = receipt(shardingScaleInTestDigestA)
			baseline.StepKey = "final-proof/" + shardingScaleInTestDigestA
			baseline.ClaimClass = shardingScaleInClaimClassProofReadOnly
			changed.Phase = appsv1.ShardingScaleInPhaseForgetting
			changed.ReceiptID = receipt(shardingScaleInTestDigestA)
			changed.StepKey = "forget-epoch/" + shardingScaleInTestDigestB + "/pod-0"
			changed.ClaimClass = shardingScaleInClaimClassForgetEntryMayMutate
		}),
		Entry("executor", func(_ *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			changed.ExecutorPodUID = "pod-1"
		}),
		Entry("executor dispatch sequence", func(_ *shardingScaleInPreparedRequestInput,
			changed *shardingScaleInPreparedRequestInput,
		) {
			changed.ExecutorDispatchSequence++
		}),
	)

	DescribeTable("accepts only canonical phase, step, receipt, and claim combinations",
		func(mutate func(*shardingScaleInPreparedRequestInput)) {
			material, planID := newCanonicalPlan()
			input := newInput()
			mutate(&input)
			_, err := buildShardingScaleInPreparedRequest(material, planID, input)
			Expect(err).ShouldNot(HaveOccurred())
		},
		Entry("drain mutation", func(_ *shardingScaleInPreparedRequestInput) {}),
		Entry("purge proof", func(input *shardingScaleInPreparedRequestInput) {
			input.Phase = appsv1.ShardingScaleInPhasePurgePrepared
			input.ReceiptID = receipt(shardingScaleInTestDigestA)
			input.StepKey = "purge-proof/" + shardingScaleInTestDigestA
			input.ClaimClass = shardingScaleInClaimClassProofReadOnly
		}),
		Entry("reset mutation", func(input *shardingScaleInPreparedRequestInput) {
			input.Phase = appsv1.ShardingScaleInPhaseResetting
			input.ReceiptID = receipt(shardingScaleInTestDigestA)
			input.StepKey = "reset/0/pod-0/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}),
		Entry("forget mutation", func(input *shardingScaleInPreparedRequestInput) {
			input.Phase = appsv1.ShardingScaleInPhaseForgetting
			input.ReceiptID = receipt(shardingScaleInTestDigestA)
			input.StepKey = "forget-epoch/" + shardingScaleInTestDigestB + "/pod-0"
			input.ClaimClass = shardingScaleInClaimClassForgetEntryMayMutate
		}),
		Entry("final proof", func(input *shardingScaleInPreparedRequestInput) {
			input.Phase = appsv1.ShardingScaleInPhaseForgetting
			input.ReceiptID = receipt(shardingScaleInTestDigestA)
			input.StepKey = "final-proof/" + shardingScaleInTestDigestA
			input.ClaimClass = shardingScaleInClaimClassProofReadOnly
		}),
		Entry("delete proof", func(input *shardingScaleInPreparedRequestInput) {
			input.Phase = appsv1.ShardingScaleInPhaseVerified
			input.ReceiptID = receipt(shardingScaleInTestDigestA)
			input.StepKey = "delete-proof/" + shardingScaleInTestDigestB
			input.ClaimClass = shardingScaleInClaimClassProofReadOnly
		}),
	)

	DescribeTable("derives the unknown-effect class from the narrower claim class",
		func(claimClass shardingScaleInClaimClass, effectClass shardingScaleInEffectClass) {
			actual, err := shardingScaleInEffectClassForClaimClass(claimClass)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actual).Should(Equal(effectClass))
		},
		Entry("ordinary read", shardingScaleInClaimClassReadOnly,
			shardingScaleInEffectClassReadOnly),
		Entry("ordinary mutation", shardingScaleInClaimClassMayMutate,
			shardingScaleInEffectClassMayMutate),
		Entry("recovery read", shardingScaleInClaimClassRecoveryReadOnly,
			shardingScaleInEffectClassReadOnly),
		Entry("proof read", shardingScaleInClaimClassProofReadOnly,
			shardingScaleInEffectClassReadOnly),
		Entry("forget mutation", shardingScaleInClaimClassForgetEntryMayMutate,
			shardingScaleInEffectClassMayMutate),
	)

	It("keeps canonical request material stable across raw input ordering", func() {
		firstRaw := newShardingScaleInPlanMaterialFixture()
		firstRaw.Source.DesiredShards = 1
		firstRaw.Leaving = append(firstRaw.Leaving, firstRaw.Staying[0])
		firstRaw.Staying = firstRaw.Staying[1:]
		secondRaw := firstRaw.DeepCopy()
		secondRaw.Leaving[0], secondRaw.Leaving[1] =
			secondRaw.Leaving[1], secondRaw.Leaving[0]
		secondRaw.RequestAuthority.ExecutorTemplates[0],
			secondRaw.RequestAuthority.ExecutorTemplates[2] =
			secondRaw.RequestAuthority.ExecutorTemplates[2],
			secondRaw.RequestAuthority.ExecutorTemplates[0]
		secondRaw.RequestAuthority.VarSources[0],
			secondRaw.RequestAuthority.VarSources[1] =
			secondRaw.RequestAuthority.VarSources[1],
			secondRaw.RequestAuthority.VarSources[0]

		firstPlan, firstPlanID, err := buildShardingScaleInPlanMaterial(firstRaw)
		Expect(err).ShouldNot(HaveOccurred())
		secondPlan, secondPlanID, err := buildShardingScaleInPlanMaterial(secondRaw)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secondPlanID).Should(Equal(firstPlanID))
		Expect(secondPlan).Should(Equal(firstPlan))

		first, err := buildShardingScaleInPreparedRequest(firstPlan, firstPlanID, newInput())
		Expect(err).ShouldNot(HaveOccurred())
		second, err := buildShardingScaleInPreparedRequest(secondPlan, secondPlanID, newInput())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(second).Should(Equal(first))
	})

	It("does not alias the persisted plan or its two returned envelope views", func() {
		material, planID := newCanonicalPlan()
		before := material.DeepCopy()
		prepared, err := buildShardingScaleInPreparedRequest(material, planID, newInput())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(material).Should(Equal(before))
		Expect(prepared.RequestPayload.Envelope).Should(Equal(prepared.Envelope))

		prepared.Envelope.Leaving[0].Pods[0].Name = "changed-output-envelope"
		Expect(prepared.RequestPayload.Envelope.Leaving[0].Pods[0].Name).
			ShouldNot(Equal("changed-output-envelope"))
		Expect(material).Should(Equal(before))

		prepared.RequestPayload.Envelope.Staying[0].Pods[0].Name = "changed-payload-envelope"
		Expect(prepared.Envelope.Staying[0].Pods[0].Name).
			ShouldNot(Equal("changed-payload-envelope"))
		Expect(material).Should(Equal(before))
	})

	DescribeTable("rejects incomplete or mismatched prepared-request inputs",
		func(mutate func(*appsv1.ShardingScaleInPlanMaterial, *string,
			*shardingScaleInPreparedRequestInput), message string,
		) {
			material, planID := newCanonicalPlan()
			input := newInput()
			mutate(material, &planID, &input)
			_, err := buildShardingScaleInPreparedRequest(material, planID, input)
			Expect(err).Should(MatchError(ContainSubstring(message)))
		},
		Entry("noncanonical plan", func(material *appsv1.ShardingScaleInPlanMaterial,
			_ *string, _ *shardingScaleInPreparedRequestInput,
		) {
			material.Staying[0], material.Staying[1] = material.Staying[1], material.Staying[0]
		}, "plan material is not canonical"),
		Entry("wrong plan ID", func(_ *appsv1.ShardingScaleInPlanMaterial,
			planID *string, _ *shardingScaleInPreparedRequestInput,
		) {
			*planID = shardingScaleInTestDigestA
		}, "plan ID does not match"),
		Entry("holder out of range", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.HolderIndex = 1
		}, "holder index is outside leaving"),
		Entry("unsupported phase", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.Phase = appsv1.ShardingScaleInPhasePlanned
		}, "phase is not dispatchable"),
		Entry("delete-committed is not a dispatch phase",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhaseDeleteCommitted
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "delete/" + shardingScaleInTestDigestA
			}, "phase is not dispatchable"),
		Entry("drain claim cannot be downgraded",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.ClaimClass = shardingScaleInClaimClassReadOnly
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("drain receipt must be explicitly absent",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("bound receipt must not be empty",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhasePurgePrepared
				input.ReceiptID = receipt("")
				input.StepKey = "purge-proof/"
				input.ClaimClass = shardingScaleInClaimClassProofReadOnly
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("purge proof requires a receipt",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhasePurgePrepared
				input.StepKey = "purge-proof/" + shardingScaleInTestDigestA
				input.ClaimClass = shardingScaleInClaimClassProofReadOnly
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("purge proof step must bind the receipt",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhasePurgePrepared
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "purge-proof/" + shardingScaleInTestDigestB
				input.ClaimClass = shardingScaleInClaimClassProofReadOnly
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("proof claim must use the frozen proof executor",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhasePurgePrepared
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "purge-proof/" + shardingScaleInTestDigestA
				input.ClaimClass = shardingScaleInClaimClassProofReadOnly
				input.ExecutorPodUID = "pod-1"
			}, "proof claim executor must match the frozen proof executor"),
		Entry("reset step must bind the executor",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				setResetTuple(input, shardingScaleInTestDigestA, "0", "pod-1")
				input.ExecutorPodUID = "pod-0"
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("reset cursor must be canonical",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				setResetTuple(input, shardingScaleInTestDigestA, "00", "pod-0")
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("reset old node ID must be exact lowercase hex",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				setResetTuple(input, shardingScaleInTestDigestA, "0", "pod-0")
				input.StepKey = "reset/0/pod-0/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("forget step must bind the executor",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhaseForgetting
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "forget-epoch/" + shardingScaleInTestDigestB + "/pod-1"
				input.ClaimClass = shardingScaleInClaimClassForgetEntryMayMutate
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("forget epoch must be a SHA256 digest",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhaseForgetting
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "forget-epoch/not-a-digest/pod-0"
				input.ClaimClass = shardingScaleInClaimClassForgetEntryMayMutate
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("final proof step must bind the receipt",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhaseForgetting
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "final-proof/" + shardingScaleInTestDigestB
				input.ClaimClass = shardingScaleInClaimClassProofReadOnly
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("delete proof digest must be exact",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInPreparedRequestInput,
			) {
				input.Phase = appsv1.ShardingScaleInPhaseVerified
				input.ReceiptID = receipt(shardingScaleInTestDigestA)
				input.StepKey = "delete-proof/not-a-digest"
				input.ClaimClass = shardingScaleInClaimClassProofReadOnly
			}, "phase, step, receipt, and claim combination is not supported"),
		Entry("missing topology fence token", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.TopologyFenceToken = ""
		}, "topology fence token must be a SHA256 digest"),
		Entry("missing step", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.StepKey = ""
		}, "step key must not be empty"),
		Entry("invalid revision", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.ResultRevision = 0
		}, "result revision must be positive"),
		Entry("unknown executor", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.ExecutorPodUID = "unknown"
		}, "executor must bind one exact plan Pod"),
		Entry("invalid sequence", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.ExecutorDispatchSequence = 0
		}, "dispatch sequence must be positive"),
		Entry("unsupported claim class", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInPreparedRequestInput,
		) {
			input.ClaimClass = "AmbientFallback"
		}, "claim class is not supported"),
	)

	It("rejects a holder target smuggled into the executor base", func() {
		material, planID := newCanonicalPlan()
		template := &material.RequestAuthority.ExecutorTemplates[0]
		record := shardingScaleInBaseParameterRecord{
			Version: shardingScaleInBaseParameterRecordVersionV1,
			Parameters: []shardingScaleInBaseParameter{
				{
					Name: shardingScaleInBaseParameterProtocolVersion,
					ValueB64: base64.StdEncoding.EncodeToString(
						[]byte(appsv1.ShardingScaleInResultProtocolV2)),
				},
				{
					Name:     shardingScaleInBaseParameterServicePort,
					ValueB64: base64.StdEncoding.EncodeToString([]byte("6379")),
				},
				{
					Name:     shardingRemoveShardNameVar,
					ValueB64: base64.StdEncoding.EncodeToString([]byte("demo-redis-2")),
				},
			},
		}
		decoded, err := json.Marshal(record)
		Expect(err).ShouldNot(HaveOccurred())
		template.BaseParameterRecordB64 = base64.StdEncoding.EncodeToString(decoded)
		template.BaseParameterDigest, err = digestShardingScaleInCanonicalJSON(record)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = buildShardingScaleInPreparedRequest(material, planID, newInput())
		Expect(err).Should(MatchError(ContainSubstring(
			`base parameter "KB_REMOVE_SHARD_NAME" is not allowed`)))
	})
})
