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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("sharding scale-in member record adapter", func() {
	It("renders and parses the persisted canonical PlanMaterial roster exactly", func() {
		material, _, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())

		lines, err := renderShardingScaleInMemberLines(material)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(lines).Should(HaveLen(1 + len(material.Leaving) + len(material.Staying)))

		decoded, err := kbagentproto.DecodeShardingScaleInMemberSet(lines)
		Expect(err).ShouldNot(HaveOccurred())
		got, err := shardingScaleInPlanMembersFromProtocol(decoded)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(reflect.DeepEqual(got.Leaving, material.Leaving)).Should(BeTrue())
		Expect(reflect.DeepEqual(got.Staying, material.Staying)).Should(BeTrue())
	})

	It("rejects noncanonical persisted order rather than sorting it into acceptance", func() {
		material, _, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(material.Staying).Should(HaveLen(2))
		material.Staying[0], material.Staying[1] = material.Staying[1], material.Staying[0]

		_, err = renderShardingScaleInMemberLines(material)
		Expect(err).Should(MatchError(ContainSubstring("not canonical")))
	})

	It("does not alias or mutate the persisted roster", func() {
		material, _, err := buildShardingScaleInPlanMaterial(
			newShardingScaleInPlanMaterialFixture())
		Expect(err).ShouldNot(HaveOccurred())
		before := material.DeepCopy()

		lines, err := renderShardingScaleInMemberLines(material)
		Expect(err).ShouldNot(HaveOccurred())
		lines[1] = "mutated"

		Expect(reflect.DeepEqual(material, before)).Should(BeTrue())
	})
})
