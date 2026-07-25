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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("sharding scale-in launch request material", func() {
	newPrepared := func() (*appsv1.ShardingScaleInPlanMaterial, string,
		*shardingScaleInPreparedRequestMaterial,
	) {
		plan, planID, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		prepared, err := buildShardingScaleInPreparedRequest(plan, planID,
			shardingScaleInPreparedRequestInput{
				HolderIndex:              0,
				TopologyFenceToken:       shardingScaleInTestDigestB,
				Phase:                    appsv1.ShardingScaleInPhaseDraining,
				ResultRevision:           7,
				StepKey:                  "drain/7",
				ClaimClass:               shardingScaleInClaimClassMayMutate,
				ExecutorPodUID:           "pod-0",
				ExecutorDispatchSequence: 1,
			})
		Expect(err).ShouldNot(HaveOccurred())
		return plan, planID, prepared
	}

	It("derives one deterministic launch identity from the exact prepared request", func() {
		plan, planID, prepared := newPrepared()
		first, err := buildShardingScaleInPreparedLaunch(plan, planID, prepared)
		Expect(err).ShouldNot(HaveOccurred())
		for range 100 {
			next, err := buildShardingScaleInPreparedLaunch(plan, planID, prepared)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(next).Should(Equal(first))
		}

		Expect(first.Material.ActionName).Should(Equal(plan.RequestAuthority.ActionName))
		Expect(first.Material.ExecutionID).Should(Equal(prepared.ExecutionID))
		Expect(first.Material.RequestPayloadDigest).Should(Equal(
			prepared.RequestPayloadDigest))
		Expect(first.Material.ExpectedAgentProcessUID).Should(Equal(
			prepared.RequestPayload.Executor.AgentProcessUID))
		Expect(first.Material.ExecutorDispatchSequence).Should(Equal(
			prepared.RequestPayload.ExecutorDispatchSequence))
		Expect(first.LaunchRequestDigest).Should(Satisfy(isShardingScaleInSHA256))
		expectedDigest, err := digestShardingScaleInCanonicalJSON(first.Material)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(first.LaunchRequestDigest).Should(Equal(expectedDigest))

		serialized, err := json.Marshal(first.Material)
		Expect(err).ShouldNot(HaveOccurred())
		for _, forbiddenField := range []string{
			"launchRequestDigest",
			"operation",
			"authorization",
			"authorizationRequestNonce",
			"ensureLaunchCallCountDiagnostic",
			"diagnostics",
		} {
			Expect(string(serialized)).ShouldNot(ContainSubstring(`"` + forbiddenField + `"`))
		}
	})

	It("changes the launch identity when the topology fence token changes", func() {
		plan, planID, firstPrepared := newPrepared()
		first, err := buildShardingScaleInPreparedLaunch(plan, planID, firstPrepared)
		Expect(err).ShouldNot(HaveOccurred())

		secondPrepared, err := buildShardingScaleInPreparedRequest(plan, planID,
			shardingScaleInPreparedRequestInput{
				HolderIndex:              0,
				TopologyFenceToken:       shardingScaleInTestDigestA,
				Phase:                    appsv1.ShardingScaleInPhaseDraining,
				ResultRevision:           7,
				StepKey:                  "drain/7",
				ClaimClass:               shardingScaleInClaimClassMayMutate,
				ExecutorPodUID:           "pod-0",
				ExecutorDispatchSequence: 1,
			})
		Expect(err).ShouldNot(HaveOccurred())
		second, err := buildShardingScaleInPreparedLaunch(plan, planID, secondPrepared)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(second.Material.RequestPayloadDigest).ShouldNot(Equal(
			first.Material.RequestPayloadDigest))
		Expect(second.Material.ExecutionID).ShouldNot(Equal(first.Material.ExecutionID))
		Expect(second.LaunchRequestDigest).ShouldNot(Equal(first.LaunchRequestDigest))
	})

	DescribeTable("rejects material that is not the exact prepared request",
		func(mutate func(*appsv1.ShardingScaleInPlanMaterial, *string,
			*shardingScaleInPreparedRequestMaterial), message string,
		) {
			plan, planID, prepared := newPrepared()
			mutate(plan, &planID, prepared)
			_, err := buildShardingScaleInPreparedLaunch(plan, planID, prepared)
			Expect(err).Should(MatchError(ContainSubstring(message)))
		},
		Entry("wrong plan ID", func(_ *appsv1.ShardingScaleInPlanMaterial,
			planID *string, _ *shardingScaleInPreparedRequestMaterial,
		) {
			*planID = shardingScaleInTestDigestA
		}, "prepared request is invalid"),
		Entry("changed action", func(plan *appsv1.ShardingScaleInPlanMaterial,
			_ *string, _ *shardingScaleInPreparedRequestMaterial,
		) {
			plan.RequestAuthority.ActionName = "ambient-fallback"
		}, "prepared request is invalid"),
		Entry("changed request payload", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, prepared *shardingScaleInPreparedRequestMaterial,
		) {
			prepared.RequestPayload.TimeoutSeconds++
		}, "prepared request does not match"),
		Entry("changed topology fence token", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, prepared *shardingScaleInPreparedRequestMaterial,
		) {
			prepared.RequestPayload.Envelope.TopologyFenceToken =
				shardingScaleInTestDigestA
		}, "prepared request does not match"),
		Entry("changed request digest", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, prepared *shardingScaleInPreparedRequestMaterial,
		) {
			prepared.RequestPayloadDigest = shardingScaleInTestDigestA
		}, "prepared request does not match"),
		Entry("changed execution ID", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, prepared *shardingScaleInPreparedRequestMaterial,
		) {
			prepared.ExecutionID = shardingScaleInTestDigestA
		}, "prepared request does not match"),
		Entry("changed holder target", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, prepared *shardingScaleInPreparedRequestMaterial,
		) {
			prepared.HolderTarget.ComponentUID = "other-component"
		}, "prepared request does not match"),
	)

	It("rejects a missing prepared request", func() {
		plan, planID, _ := newPrepared()
		_, err := buildShardingScaleInPreparedLaunch(plan, planID, nil)
		Expect(err).Should(MatchError(ContainSubstring(
			"prepared request must not be nil")))
	})

	It("does not alias the prepared request", func() {
		plan, planID, prepared := newPrepared()
		launch, err := buildShardingScaleInPreparedLaunch(plan, planID, prepared)
		Expect(err).ShouldNot(HaveOccurred())
		expected := launch.Material.RequestPayload.Envelope.Leaving[0].Pods[0].Name

		prepared.RequestPayload.Envelope.Leaving[0].Pods[0].Name = "changed-prepared"
		Expect(launch.Material.RequestPayload.Envelope.Leaving[0].Pods[0].Name).
			Should(Equal(expected))

		launch.Material.RequestPayload.Envelope.Leaving[0].Pods[0].Name = "changed-launch"
		Expect(prepared.RequestPayload.Envelope.Leaving[0].Pods[0].Name).
			ShouldNot(Equal("changed-launch"))
	})
})
