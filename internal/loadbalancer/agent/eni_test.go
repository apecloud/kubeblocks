package agent

import (
	"math"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	mockcloud "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/mocks"
	mocknetwork "github.com/apecloud/kubeblocks/internal/loadbalancer/network/mocks"
)

var _ = Describe("Eni", func() {

	const (
		masterHostIP = "172.31.1.2"
		subnet       = "172.31.0.0/16"

		eniId1  = "eni-01"
		eniMac1 = "00:00:00:00:00:01"
		eniIp11 = "172.31.1.10"
		eniIp12 = "172.31.1.11"
		eniIp13 = "172.31.1.12"

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

	setup := func() (*eniManager, *mockcloud.MockProvider, *mocknetwork.MockClient) {
		ctrl := gomock.NewController(GinkgoT())
		mockProvider := mockcloud.NewMockProvider(ctrl)
		mockNetworkClient := mocknetwork.NewMockClient(ctrl)
		enis := map[string]*cloud.ENIMetadata{
			eniId1: {
				ENIId:        eniId1,
				DeviceNumber: 1,
				Tags: map[string]string{
					cloud.TagENIKubeBlocksManaged: "true",
				},
			},
		}
		mockProvider.EXPECT().GetENILimit().Return(math.MaxInt)
		mockProvider.EXPECT().GetENIIPv4Limit().Return(6)
		mockProvider.EXPECT().DescribeAllENIs().Return(enis, nil)
		mockNetworkClient.EXPECT().SetupNetworkForENI(gomock.Any()).Return(errors.New("mock setup failed"))
		mockNetworkClient.EXPECT().SetupNetworkForENI(gomock.Any()).Return(nil)
		manager, err := NewENIManager(logger, mockProvider, mockNetworkClient)
		Expect(err).Should(BeNil())
		return manager, mockProvider, mockNetworkClient
	}

	getMockENIs := func() map[string]*cloud.ENIMetadata {
		return map[string]*cloud.ENIMetadata{
			eniId1: {
				ENIId:          eniId1,
				MAC:            eniMac1,
				DeviceNumber:   0,
				SubnetIPv4CIDR: subnet,
				IPv4Addresses: []*cloud.IPv4Address{
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
				ENIId:          eniId2,
				MAC:            eniMac2,
				DeviceNumber:   1,
				SubnetIPv4CIDR: subnet,
				Tags: map[string]string{
					cloud.TagENIKubeBlocksManaged: "true",
					cloud.TagENINode:              masterHostIP,
					cloud.TagENICreatedAt:         time.Now().String(),
				},
				IPv4Addresses: []*cloud.IPv4Address{
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
				ENIId:          eniId3,
				MAC:            eniMac3,
				DeviceNumber:   3,
				SubnetIPv4CIDR: subnet,
				Tags: map[string]string{
					cloud.TagENIKubeBlocksManaged: "true",
					cloud.TagENINode:              masterHostIP,
					cloud.TagENICreatedAt:         time.Now().String(),
				},
				IPv4Addresses: []*cloud.IPv4Address{
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
				ENIId:        eniId4,
				DeviceNumber: 4,
				Tags: map[string]string{
					cloud.TagENIKubeBlocksManaged: "true",
					cloud.TagENINode:              masterHostIP,
					cloud.TagENICreatedAt:         time.Now().String(),
				},
				IPv4Addresses: []*cloud.IPv4Address{
					{
						Primary: true,
						Address: eniIp41,
					},
				},
			},
		}
	}

	Context("Test start", func() {
		It("", func() {
			manager, mockProvider, _ := setup()

			enis := map[string]*cloud.ENIMetadata{
				eniId1: {
					ENIId:        eniId1,
					DeviceNumber: 0,
				},
			}
			mockProvider.EXPECT().DescribeAllENIs().Return(enis, nil).AnyTimes()
			mockProvider.EXPECT().ModifySourceDestCheck(eniId1, gomock.Any()).Return(nil)
			// we close stop channel to prevent running ensureENI
			stop := make(chan struct{})
			close(stop)

			Expect(manager.Start(stop)).Should(Succeed())
		})
	})

	Context("ChooseBusiestENI", func() {
		It("", func() {
			manager, mockProvider, _ := setup()
			enis := getMockENIs()
			mockProvider.EXPECT().DescribeAllENIs().Return(enis, nil)
			eni, err := manager.ChooseBusiestENI()
			Expect(err).Should(BeNil())
			Expect(eni.ENIId).Should(Equal(eniId2))
		})
	})

	Context("Ensure ENI, alloc new ENI", func() {
		It("", func() {
			manager, mockProvider, mockNetwork := setup()
			enis := getMockENIs()
			mockProvider.EXPECT().DescribeAllENIs().Return(enis, nil)
			manager.minPrivateIP = math.MaxInt

			eni := cloud.ENIMetadata{ENIId: eniId5}
			mockProvider.EXPECT().AllocENI().Return(eni.ENIId, nil)
			mockProvider.EXPECT().WaitForENIAttached(eni.ENIId).Return(eni, nil)
			mockNetwork.EXPECT().SetupNetworkForENI(&eni).Return(nil)
			Expect(manager.ensureENI()).Should(Succeed())
		})
	})

	Context("Ensure ENI, delete spare ENI", func() {
		It("", func() {
			manager, mockProvider, mockNetwork := setup()
			enis := getMockENIs()
			mockProvider.EXPECT().DescribeAllENIs().Return(enis, nil)

			var ids []string
			recordDeletedENI := func(eni *cloud.ENIMetadata) error {
				ids = append(ids, eni.ENIId)
				return nil
			}
			mockNetwork.EXPECT().CleanNetworkForENI(gomock.Any()).DoAndReturn(recordDeletedENI).Return(nil).AnyTimes()
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
			mockProvider.EXPECT().FindLeakedENIs().Return(enis, nil)
			mockProvider.EXPECT().DeleteENI(gomock.Any()).Return(nil).AnyTimes()
			Expect(manager.cleanLeakedENIs()).Should(Succeed())
		})
	})
})
