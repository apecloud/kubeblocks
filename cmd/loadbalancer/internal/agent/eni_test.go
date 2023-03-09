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
	"math"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	mockcloud "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/mocks"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
	mockprotocol "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol/mocks"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	masterHostIP = "172.31.1.2"
	nodeIP       = "172.31.1.100"
	subnet       = "172.31.0.0/16"

	instanceID      = "i-0000000000000"
	securityGroupID = "sec-0000000000000"
	eniID1          = "eni-01"
	eniMac1         = "00:00:00:00:00:01"
	eniIP11         = "172.31.1.10"
	eniIP12         = "172.31.1.11"
	eniIP13         = "172.31.1.12"

	eniID2  = "eni-02"
	eniMac2 = "00:00:00:00:00:02"
	eniIP21 = "172.31.2.10"
	eniIP22 = "172.31.2.11"
	eniIP23 = "172.31.2.12"

	eniID3  = "eni-03"
	eniMac3 = "00:00:00:00:00:03"
	eniIP31 = "172.31.3.10"
	eniIP32 = "172.31.3.11"

	eniID4  = "eni-04"
	eniIP41 = "172.31.4.10"
	eniID5  = "eni-05"
)

var getDescribeAllENIResponse = func() *pb.DescribeAllENIsResponse {
	return &pb.DescribeAllENIsResponse{
		RequestId: util.GenRequestID(),
		Enis:      getMockENIs(),
	}
}

var getMockENIs = func() map[string]*pb.ENIMetadata {
	return map[string]*pb.ENIMetadata{
		eniID1: {
			EniId:          eniID1,
			Mac:            eniMac1,
			DeviceNumber:   0,
			SubnetIpv4Cidr: subnet,
			Ipv4Addresses: []*pb.IPv4Address{
				{
					Primary: true,
					Address: eniIP11,
				},
				{
					Primary: true,
					Address: eniIP12,
				},
				{
					Primary: true,
					Address: eniIP13,
				},
			},
		},
		// busiest ENI
		eniID2: {
			EniId:          eniID2,
			Mac:            eniMac2,
			DeviceNumber:   1,
			SubnetIpv4Cidr: subnet,
			Tags: map[string]string{
				cloud.TagENIKubeBlocksManaged: "true",
				cloud.TagENINode:              masterHostIP,
				cloud.TagENICreatedAt:         time.Now().String(),
			},
			Ipv4Addresses: []*pb.IPv4Address{
				{
					Primary: true,
					Address: eniIP21,
				},
				{
					Primary: false,
					Address: eniIP22,
				},
				{
					Primary: false,
					Address: eniIP23,
				},
			},
		},
		eniID3: {
			EniId:          eniID3,
			Mac:            eniMac3,
			DeviceNumber:   3,
			SubnetIpv4Cidr: subnet,
			Tags: map[string]string{
				cloud.TagENIKubeBlocksManaged: "true",
				cloud.TagENINode:              masterHostIP,
				cloud.TagENICreatedAt:         time.Now().String(),
			},
			Ipv4Addresses: []*pb.IPv4Address{
				{
					Primary: true,
					Address: eniIP31,
				},
				{
					Primary: false,
					Address: eniIP32,
				},
			},
		},
		eniID4: {
			EniId:        eniID4,
			DeviceNumber: 4,
			Tags: map[string]string{
				cloud.TagENIKubeBlocksManaged: "true",
				cloud.TagENINode:              masterHostIP,
				cloud.TagENICreatedAt:         time.Now().String(),
			},
			Ipv4Addresses: []*pb.IPv4Address{
				{
					Primary: true,
					Address: eniIP41,
				},
			},
		},
	}
}

var _ = Describe("Eni", func() {

	setup := func() (*eniManager, *mockcloud.MockProvider, *mockprotocol.MockNodeClient) {
		ctrl := gomock.NewController(GinkgoT())
		mockProvider := mockcloud.NewMockProvider(ctrl)
		mockNodeClient := mockprotocol.NewMockNodeClient(ctrl)
		mockProvider.EXPECT().GetENILimit().Return(math.MaxInt)
		mockProvider.EXPECT().GetENIIPv4Limit().Return(6)
		mockNodeClient.EXPECT().DescribeAllENIs(gomock.Any(), gomock.Any()).Return(getDescribeAllENIResponse(), nil).AnyTimes()
		mockNodeClient.EXPECT().SetupNetworkForENI(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		info := &pb.InstanceInfo{
			InstanceId:       instanceID,
			SubnetId:         subnet1Id,
			SecurityGroupIds: []string{securityGroupID},
		}
		manager, err := newENIManager(logger, nodeIP, info, mockNodeClient, mockProvider)
		Expect(err).Should(BeNil())
		return manager, mockProvider, mockNodeClient
	}

	Context("Test start", func() {
		It("", func() {
			manager, mockProvider, _ := setup()
			mockProvider.EXPECT().ModifySourceDestCheck(eniID1, gomock.Any()).Return(nil)
			// we close stop channel to prevent running ensureENI
			stop := make(chan struct{})
			close(stop)

			Expect(manager.start(stop, 10*time.Second, 1*time.Minute)).Should(Succeed())
		})
	})

	Context("Ensure ENI, alloc new ENI", func() {
		It("", func() {
			manager, mockProvider, mockNodeClient := setup()
			manager.minPrivateIP = math.MaxInt

			eni := cloud.ENIMetadata{ID: eniID5}
			mockProvider.EXPECT().CreateENI(gomock.Any(), gomock.Any(), gomock.Any()).Return(eni.ID, nil)
			mockProvider.EXPECT().AttachENI(gomock.Any(), gomock.Any()).Return(eni.ID, nil)
			mockNodeClient.EXPECT().WaitForENIAttached(gomock.Any(), gomock.Any()).Return(nil, nil)
			mockNodeClient.EXPECT().SetupNetworkForENI(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			Expect(manager.ensureENI()).Should(Succeed())
		})
	})

	Context("Ensure ENI, delete spare ENI", func() {
		It("", func() {
			manager, mockProvider, mockNodeClient := setup()

			var ids []string
			recordDeletedENI := func(ctx context.Context, request *pb.CleanNetworkForENIRequest, options ...grpc.CallOption) (*pb.CleanNetworkForENIResponse, error) {
				ids = append(ids, request.GetEni().EniId)
				return nil, nil
			}
			mockNodeClient.EXPECT().CleanNetworkForENI(gomock.Any(), gomock.Any()).DoAndReturn(recordDeletedENI).Return(nil, nil).AnyTimes()
			mockProvider.EXPECT().FreeENI(gomock.Any()).Return(nil).AnyTimes()
			Expect(manager.ensureENI()).Should(Succeed())
			Expect(len(ids)).Should(Equal(1))
			Expect(ids[0]).Should(Equal(eniID4))
		})
	})

	Context("Clean leaked ENI", func() {
		It("", func() {
			manager, mockProvider, _ := setup()
			enis := []*cloud.ENIMetadata{
				{
					ID:           eniID1,
					DeviceNumber: 0,
				},
			}
			mockProvider.EXPECT().FindLeakedENIs(gomock.Any()).Return(enis, nil)
			mockProvider.EXPECT().DeleteENI(gomock.Any()).Return(nil).AnyTimes()
			Expect(manager.cleanLeakedENIs()).Should(Succeed())
		})
	})
})
