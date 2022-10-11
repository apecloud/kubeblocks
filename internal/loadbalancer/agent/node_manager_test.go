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
