package agent

import (
	"math"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mockcloud "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/mocks"
	mock_protocol "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol/mocks"
)

var _ = Describe("Node", func() {
	setup := func() (*node, *eniManager, *mock_protocol.MockNodeClient, *mockcloud.MockProvider) {
		ctrl := gomock.NewController(GinkgoT())
		mockNodeClient := mock_protocol.NewMockNodeClient(ctrl)
		mockProvider := mockcloud.NewMockProvider(ctrl)
		em := &eniManager{
			maxIPsPerENI: math.MaxInt,
			cp:           mockProvider,
			nc:           mockNodeClient,
		}
		node := &node{
			em:     em,
			nc:     mockNodeClient,
			cp:     mockProvider,
			logger: logger,
		}
		return node, em, mockNodeClient, mockProvider
	}

	Context("Choose ENI", func() {
		It("", func() {

			node, _, mockNodeClient, _ := setup()
			mockNodeClient.EXPECT().DescribeAllENIs(gomock.Any(), gomock.Any()).Return(getDescribeAllENIResponse(), nil)
			eni, err := node.ChooseENI()
			Expect(err).Should(BeNil())
			Expect(eni.EniId).Should(Equal(eniId2))
		})
	})
})
