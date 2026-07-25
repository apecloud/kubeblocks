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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("sharding scale-in envelope v2 adapter", func() {
	newInput := func(
		material *appsv1.ShardingScaleInPlanMaterial,
		planID string,
	) shardingScaleInEnvelopeV2Input {
		target, err := buildShardingScaleInHolderTarget(planID, 0, material.Leaving[0])
		Expect(err).ShouldNot(HaveOccurred())
		return shardingScaleInEnvelopeV2Input{
			TopologyFenceToken: shardingScaleInTestDigestB,
			BaseParameterDigest: material.RequestAuthority.
				ExecutorTemplates[0].BaseParameterDigest,
			Phase:        appsv1.ShardingScaleInPhaseDraining,
			HolderTarget: target,
		}
	}

	It("renders the persisted PlanMaterial and typed holder into the shared envelope", func() {
		material, planID, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		input := newInput(material, planID)
		before := material.DeepCopy()

		first, err := renderShardingScaleInEnvelopeV2(material, planID, input)
		Expect(err).ShouldNot(HaveOccurred())
		for range 100 {
			next, err := renderShardingScaleInEnvelopeV2(material, planID, input)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(next).Should(Equal(first))
		}

		decoded, err := kbagentproto.DecodeShardingScaleInEnvelopeV2(first)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(decoded.PlanID).Should(Equal(planID))
		Expect(decoded.TopologyFenceToken).Should(Equal(input.TopologyFenceToken))
		Expect(decoded.RequestAuthorityDigest).Should(Equal(
			material.RequestAuthority.RequestAuthorityDigest))
		Expect(decoded.BaseParameterDigest).Should(Equal(input.BaseParameterDigest))
		Expect(decoded.Phase).Should(Equal(string(input.Phase)))
		Expect(decoded.Holder.ParameterName).Should(Equal(shardingRemoveShardNameVar))
		Expect(decoded.Holder.ComponentUID).Should(Equal(
			string(material.Leaving[0].ComponentUID)))
		Expect(decoded.ReceiptID).Should(BeEmpty())

		got, err := shardingScaleInPlanMembersFromProtocol(decoded.Members)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(reflect.DeepEqual(got.Leaving, material.Leaving)).Should(BeTrue())
		Expect(reflect.DeepEqual(got.Staying, material.Staying)).Should(BeTrue())
		Expect(reflect.DeepEqual(material, before)).Should(BeTrue())
	})

	It("binds a nonempty receipt into the exact envelope", func() {
		material, planID, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		input := newInput(material, planID)
		input.Phase = appsv1.ShardingScaleInPhaseResetting
		input.ReceiptID = shardingScaleInTestDigestA

		rendered, err := renderShardingScaleInEnvelopeV2(material, planID, input)
		Expect(err).ShouldNot(HaveOccurred())
		decoded, err := kbagentproto.DecodeShardingScaleInEnvelopeV2(rendered)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(decoded.Phase).Should(Equal(string(appsv1.ShardingScaleInPhaseResetting)))
		Expect(decoded.ReceiptID).Should(Equal(shardingScaleInTestDigestA))
	})

	It("rejects noncanonical persisted material rather than sorting it", func() {
		material, planID, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		input := newInput(material, planID)
		material.Staying[0], material.Staying[1] = material.Staying[1], material.Staying[0]

		_, err = renderShardingScaleInEnvelopeV2(material, planID, input)
		Expect(err).Should(MatchError(ContainSubstring("not canonical")))
	})

	It("rejects a nil persisted PlanMaterial", func() {
		_, err := renderShardingScaleInEnvelopeV2(
			nil, shardingScaleInTestDigestA, shardingScaleInEnvelopeV2Input{})
		Expect(err).Should(MatchError(ContainSubstring("must not be nil")))
	})

	DescribeTable("rejects material that is not exactly bound to the persisted plan",
		func(mutate func(*appsv1.ShardingScaleInPlanMaterial, *string,
			*shardingScaleInEnvelopeV2Input),
		) {
			material, planID, err := buildShardingScaleInPlanMaterial(
				newShardingScaleInPlanMaterialFixture())
			Expect(err).ShouldNot(HaveOccurred())
			input := newInput(material, planID)
			mutate(material, &planID, &input)

			_, err = renderShardingScaleInEnvelopeV2(material, planID, input)
			Expect(err).Should(HaveOccurred())
			Expect(errors.Is(err, errInvalidShardingScaleInEnvelopeV2)).Should(BeTrue())
		},
		Entry("plan ID", func(_ *appsv1.ShardingScaleInPlanMaterial,
			planID *string, _ *shardingScaleInEnvelopeV2Input,
		) {
			*planID = shardingScaleInTestDigestA
		}),
		Entry("topology fence", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.TopologyFenceToken = "not-a-digest"
		}),
		Entry("request authority digest", func(material *appsv1.ShardingScaleInPlanMaterial,
			_ *string, _ *shardingScaleInEnvelopeV2Input,
		) {
			material.RequestAuthority.RequestAuthorityDigest = shardingScaleInTestDigestA
		}),
		Entry("base parameter digest", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.BaseParameterDigest = "not-a-digest"
		}),
		Entry("unbound base parameter digest",
			func(_ *appsv1.ShardingScaleInPlanMaterial,
				_ *string, input *shardingScaleInEnvelopeV2Input,
			) {
				input.BaseParameterDigest = shardingScaleInTestDigestC
			}),
		Entry("phase", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.Phase = appsv1.ShardingScaleInPhasePlanned
		}),
		Entry("holder identity", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.HolderTarget.ComponentUID = "different"
		}),
		Entry("holder source digest", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.HolderTarget.SourceDigest = shardingScaleInTestDigestA
		}),
		Entry("holder value", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.HolderTarget.ValueB64 = "ZGlmZmVyZW50"
		}),
		Entry("holder index", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.HolderTarget.HolderIndex = 1
		}),
		Entry("receipt", func(_ *appsv1.ShardingScaleInPlanMaterial,
			_ *string, input *shardingScaleInEnvelopeV2Input,
		) {
			input.ReceiptID = "not-a-digest"
		}),
	)
})
