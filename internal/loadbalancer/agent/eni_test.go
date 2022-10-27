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
	"context"
	"math"
	"time"

	"google.golang.org/grpc"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	mockcloud "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/mocks"
	mockprotocol "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol/mocks"
)

const (
	masterHostIP = "172.31.1.2"
	nodeIP       = "172.31.1.100"
	subnet       = "172.31.0.0/16"

	instanceId      = "i-0000000000000"
	securityGroupId = "sec-0000000000000"
	eniId1          = "eni-01"
	eniMac1         = "00:00:00:00:00:01"
	eniIp11         = "172.31.1.10"
	eniIp12         = "172.31.1.11"
	eniIp13         = "172.31.1.12"

	eniId2  = "eni-02"
	eniMac2 = "00:00:00:00:00:02"
	eniIp21 = "172.31.2.10"
	eniIp22 = "172.31.2.11"
	eniIp23 = "172.31.2.12"
	eniIp24 = "172.31.2.14"

	eniId3  = "eni-03"
	eniMac3 = "00:00:00:00:00:03"
	eniIp31 = "172.31.3.10"
	eniIp32 = "172.31.3.11"

	eniId4  = "eni-04"
	eniIp41 = "172.31.4.10"
	eniId5  = "eni-05"
)

var getDescribeAllENIResponse = func() *pb.DescribeAllENIsResponse {
	return &pb.DescribeAllENIsResponse{
		RequestId: util.GenRequestId(),
		Enis:      getMockENIs(),
	}
}

var getDescribeNodeInfoResponse = func() *pb.DescribeNodeInfoResponse {
	return &pb.DescribeNodeInfoResponse{
		RequestId: util.GenRequestId(),
		Info: &pb.InstanceInfo{
			InstanceId:       instanceId,
			SubnetId:         subnet1Id,
			SecurityGroupIds: []string{securityGroupId},
		},
	}
}

var getMockENIs = func() map[string]*pb.ENIMetadata {
	return map[string]*pb.ENIMetadata{
		eniId1: {
			EniId:          eniId1,
			Mac:            eniMac1,
			DeviceNumber:   0,
			SubnetIpv4Cidr: subnet,
			Ipv4Addresses: []*pb.IPv4Address{
				{
					Primary: true,
					Address: eniIp11,
				},
				{
					Primary: true,
					Address: eniIp12,
				},
				{
					Primary: true,
					Address: eniIp13,
				},
			},
		},
		// busiest ENI
		eniId2: {
			EniId:          eniId2,
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
					Address: eniIp21,
				},
				{
					Primary: false,
					Address: eniIp22,
				},
				{
					Primary: false,
					Address: eniIp23,
				},
			},
		},
		eniId3: {
			EniId:          eniId3,
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
					Address: eniIp31,
				},
				{
					Primary: false,
					Address: eniIp32,
				},
			},
		},
		eniId4: {
			EniId:        eniId4,
			DeviceNumber: 4,
			Tags: map[string]string{
				cloud.TagENIKubeBlocksManaged: "true",
				cloud.TagENINode:              masterHostIP,
				cloud.TagENICreatedAt:         time.Now().String(),
			},
			Ipv4Addresses: []*pb.IPv4Address{
				{
					Primary: true,
					Address: eniIp41,
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
		mockNodeClient.EXPECT().DescribeNodeInfo(gomock.Any(), gomock.Any()).Return(getDescribeNodeInfoResponse(), nil)
		manager, err := newENIManager(logger, nodeIP, mockNodeClient, mockProvider)
		Expect(err).Should(BeNil())
		return manager, mockProvider, mockNodeClient
	}

	Context("Test start", func() {
		It("", func() {
			manager, mockProvider, _ := setup()
			mockProvider.EXPECT().ModifySourceDestCheck(eniId1, gomock.Any()).Return(nil)
			// we close stop channel to prevent running ensureENI
			stop := make(chan struct{})
			close(stop)

			Expect(manager.start(stop, 10*time.Second, 1*time.Minute)).Should(Succeed())
		})
	})

	Context("Ensure ENI, alloc new ENI", func() {
		It("", func() {
			manager, mockProvider, mockNodeClient := setup()
			mockNodeClient.EXPECT().DescribeAllENIs(gomock.Any(), gomock.Any()).Return(getDescribeAllENIResponse(), nil)
			manager.minPrivateIP = math.MaxInt

			eni := cloud.ENIMetadata{ENIId: eniId5}
			mockProvider.EXPECT().CreateENI(gomock.Any(), gomock.Any(), gomock.Any()).Return(eni.ENIId, nil)
			mockProvider.EXPECT().AttachENI(gomock.Any(), gomock.Any()).Return(eni.ENIId, nil)
			mockNodeClient.EXPECT().WaitForENIAttached(gomock.Any(), gomock.Any()).Return(nil, nil)
			mockNodeClient.EXPECT().SetupNetworkForENI(gomock.Any(), &eni).Return(nil, nil)
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
			Expect(ids[0]).Should(Equal(eniId4))
		})
	})

	Context("Clean leaked ENI", func() {
		It("", func() {
			manager, mockProvider, _ := setup()
			enis := []*cloud.ENIMetadata{
				{
					ENIId:        eniId1,
					DeviceNumber: 0,
				},
			}
			mockProvider.EXPECT().FindLeakedENIs(gomock.Any()).Return(enis, nil)
			mockProvider.EXPECT().DeleteENI(gomock.Any()).Return(nil).AnyTimes()
			Expect(manager.cleanLeakedENIs()).Should(Succeed())
		})
	})
})
