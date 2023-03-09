/*
Copyright ApeCloud, Inc.

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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
)

const (
	node1IP   = "172.31.1.10"
	subnet1Id = "subnet-001"
	node2IP   = "172.31.1.11"
	subnet2Id = "subnet-002"
	node3IP   = "172.31.1.11"
)

var _ = Describe("NodeManager", func() {
	setup := func() (*gomock.Controller, *nodeManager) {
		ctrl := gomock.NewController(GinkgoT())
		nm := &nodeManager{
			logger: logger,
			Client: &mockK8sClient{},
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
		return ctrl, nm
	}

	Context("Refresh nodes", func() {
		It("", func() {
			_, nm := setup()
			newGRPCConn = func(addr string) (*grpc.ClientConn, error) {
				return nil, nil
			}
			newNode = func(logger logr.Logger, ip string, nc pb.NodeClient, cp cloud.Provider) (Node, error) {
				return &mockNode{node: &node{stop: make(chan struct{})}}, nil
			}
			Expect(nm.refreshNodes()).Should(Succeed())
		})
	})

	Context("Choose spare node", func() {
		It("", func() {
			_, nm := setup()
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

type mockNode struct {
	*node
}

func (m *mockNode) Start() error {
	return nil
}

type mockK8sClient struct {
	client.Client
}

func (m *mockK8sClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	result := list.(*corev1.NodeList)
	result.Items = []corev1.Node{
		{
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: node1IP,
					},
					{
						Type:    corev1.NodeInternalIP,
						Address: node2IP,
					},
				},
			},
		},
	}
	return nil
}
