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
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"

	mockcloud "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/mocks"
	mock_protocol "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol/mocks"
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
			Expect(eni.EniId).Should(Equal(eniID2))
		})
	})
})
