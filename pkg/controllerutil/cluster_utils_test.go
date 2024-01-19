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

package controllerutil

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("cluster utils test", func() {

	Context("cluster utils test", func() {
		It("get original or generated cluster component spec test", func() {
			cluster := &appsv1alpha1.Cluster{
				Spec: appsv1alpha1.ClusterSpec{
					ShardingSpecs: []appsv1alpha1.ShardingSpec{
						{
							Template: appsv1alpha1.ClusterComponentSpec{
								Replicas: 2,
								Name:     "fakeCompName",
							},
							Name:   "shardingName",
							Shards: 3,
						},
					},
					ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
						{
							Name: "compName",
						},
					},
				},
			}

			compSpec := GetOriginalOrGeneratedComponentSpecByName(cluster, "compName")
			Expect(compSpec).ShouldNot(BeNil())
			Expect(compSpec.Name).Should(Equal("compName"))

			compSpec = GetOriginalOrGeneratedComponentSpecByName(cluster, "fakeCompName")
			Expect(compSpec).Should(BeNil())

			compSpec = GetOriginalOrGeneratedComponentSpecByName(cluster, "shardingName-0")
			Expect(compSpec).ShouldNot(BeNil())
			Expect(compSpec.Name).Should(Equal("shardingName-0"))
		})
	})
})
