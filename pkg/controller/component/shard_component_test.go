/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("cluster shard component", func() {

	Context("cluster shard component test", func() {
		It("cluster shard component test", func() {
			shardSpec := &appsv1alpha1.ShardSpec{
				Template: appsv1alpha1.ClusterComponentSpec{
					Name:     "test",
					Replicas: 2,
				},
				Shards: 3,
			}

			compSpecList := GenShardCompSpecList(shardSpec)
			Expect(len(compSpecList)).Should(Equal(3))
			Expect(compSpecList[0].Name).Should(Equal("test-0"))
			Expect(compSpecList[0].Name).Should(Equal("test-1"))
			Expect(compSpecList[0].Name).Should(Equal("test-2"))

			compNameList := GenShardCompNameList(shardSpec)
			Expect(len(compNameList)).Should(Equal(3))
			Expect(compNameList[0]).Should(Equal("test-0"))
			Expect(compNameList[0]).Should(Equal("test-1"))
			Expect(compNameList[0]).Should(Equal("test-2"))
		})
	})
})
