/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("cluster validation transformer test", func() {
	Context("cluster api validation", func() {
		It("with cluster topology", func() {
			By("explicitly specify topology")
			cluster := &appsv1.Cluster{
				Spec: appsv1.ClusterSpec{
					ClusterDef: "clusterdef",
					Topology:   "topology",
					ComponentSpecs: []appsv1.ClusterComponentSpec{
						{
							ComponentDef: "compdef",
						},
						{
							ComponentDef: "compdef",
						},
					},
				},
			}
			Expect(withClusterTopology(cluster)).Should(BeTrue())

			By("use default topology")
			cluster.Spec.Topology = ""
			Expect(withClusterTopology(cluster)).Should(BeTrue())

			By("w/o topology")
			cluster.Spec.ClusterDef = ""
			cluster.Spec.Topology = ""
			Expect(withClusterTopology(cluster)).Should(BeFalse())
		})

		It("with cluster user defined", func() {
			By("specify componentDef only")
			cluster := &appsv1.Cluster{
				Spec: appsv1.ClusterSpec{
					ComponentSpecs: []appsv1.ClusterComponentSpec{
						{
							ComponentDef: "compdef",
						},
						{
							ComponentDef: "compdef",
						},
					},
				},
			}
			Expect(withClusterUserDefined(cluster)).Should(BeTrue())

			By("+clusterDef")
			cluster.Spec.ClusterDef = "clusterdef"
			Expect(withClusterUserDefined(cluster)).Should(BeFalse())
		})
	})
})
