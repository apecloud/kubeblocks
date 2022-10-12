/*
Copyright 2022 The KubeBlocks Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

var _ = Describe("NodeManager", func() {
	const (
		node1IP   = "172.31.1.10"
		subnet1Id = "subnet-001"
		node2IP   = "172.31.1.11"
		subnet2Id = "subnet-002"
		node3IP   = "172.31.1.11"
	)
	setup := func() *nodeManager {
		nm := &nodeManager{
			logger: logger,
			nodes: map[string]Node{
				node1IP: &node{
					ip: node1IP,
					em: &eniManager{resource: &NodeResource{
						TotalPrivateIPs: 6,
						UsedPrivateIPs:  1,
						SubnetIds: map[string]map[string]*pb.ENIMetadata{
							subnet1Id: {},
						},
					}},
				},
				node2IP: &node{
					ip: node2IP,
					em: &eniManager{resource: &NodeResource{
						TotalPrivateIPs: 6,
						UsedPrivateIPs:  4,
						SubnetIds: map[string]map[string]*pb.ENIMetadata{
							subnet1Id: {},
							subnet2Id: {},
						},
					}},
				},
			},
		}
		return nm
	}

	Context("Choose spare node", func() {
		It("", func() {
			nm := setup()
			node, err := nm.ChooseSpareNode(subnet2Id)
			Expect(err).Should(BeNil())
			Expect(node.GetIP()).Should(Equal(node2IP))

			node, err = nm.ChooseSpareNode(subnet1Id)
			Expect(err).Should(BeNil())
			Expect(node.GetIP()).Should(Equal(node1IP))

			node, err = nm.ChooseSpareNode("")
			Expect(err).Should(BeNil())
			Expect(node.GetIP()).Should(Equal(node1IP))
		})
	})
})
