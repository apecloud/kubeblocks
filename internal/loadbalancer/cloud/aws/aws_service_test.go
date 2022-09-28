package aws

import (
	"context"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	mock_aws "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/aws/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"strings"
	"time"
)

var _ = Describe("AwsService", func() {
	const (
		metadataMACPath      = "network/interfaces/macs/"
		metadataMAC          = "mac"
		metadataAZ           = "placement/availability-zone"
		metadataLocalIP      = "local-ipv4"
		metadataInstanceID   = "instance-id"
		metadataInstanceType = "instance-type"
		metadataSGs          = "/security-group-ids"
		metadataSubnetID     = "/subnet-id"
		metadataVPCcidrs     = "/vpc-ipv4-cidr-blocks"
		metadataDeviceNum    = "/device-number"
		metadataInterface    = "/interface-id"
		metadataSubnetCIDR   = "/subnet-ipv4-cidr-block"
		metadataIPv4s        = "/local-ipv4s"

		maxENI          = 3
		maxIPsPerENI    = 16
		az              = "local"
		localIP         = "172.31.0.1"
		instanceId      = "i-0000000000000"
		instanceType    = "t3.medium"
		primaryMAC      = "00:00:00:00:00:01"
		securityGroupId = "sg-00000000"
		subnet          = "172.31.0.0/24"
		subnetId        = "subnet-00000000"
		attachmentId    = "eni-attach-00000000"

		eniId1          = "eni-01"
		eniDeviceIndex1 = "0"
		eniMac1         = "00:00:00:00:00:01"
		eniIp11         = "172.31.1.10"
		eniIp12         = "172.31.1.11"
		eniIp13         = "172.31.1.12"

		eniId2          = "eni-02"
		eniMac2         = "00:00:00:00:00:02"
		eniDeviceIndex2 = "1"
		eniIp21         = "172.31.2.10"
		eniIp22         = "172.31.2.11"
		eniIp23         = "172.31.2.12"
		eniIp24         = "172.31.2.14"

		eniId3          = "eni-03"
		eniMac3         = "00:00:00:00:00:03"
		eniDeviceIndex3 = "2"
		eniIp31         = "172.31.3.10"

		eniId4 = "eni-04"
	)

	getImdsService := func(overrides map[string]interface{}) imdsService {
		data := map[string]interface{}{
			metadataAZ:           az,
			metadataLocalIP:      localIP,
			metadataInstanceID:   instanceId,
			metadataInstanceType: instanceType,
			metadataMAC:          primaryMAC,
			metadataMACPath:      primaryMAC,
			metadataMACPath + primaryMAC + metadataDeviceNum:  eniDeviceIndex1,
			metadataMACPath + primaryMAC + metadataInterface:  eniId1,
			metadataMACPath + primaryMAC + metadataSGs:        securityGroupId,
			metadataMACPath + primaryMAC + metadataIPv4s:      strings.Join([]string{eniIp11, eniIp12, eniIp13}, " "),
			metadataMACPath + primaryMAC + metadataSubnetID:   subnetId,
			metadataMACPath + primaryMAC + metadataSubnetCIDR: subnet,
			metadataMACPath + primaryMAC + metadataVPCcidrs:   subnet,
		}

		if overrides != nil {
			for k, v := range overrides {
				data[k] = v
			}
		}
		return imdsService{fakeIMDS(data)}
	}

	getTags := func(t time.Time) map[string]string {
		tags := map[string]string{
			cloud.TagENINode:              instanceId,
			cloud.TagENIKubeBlocksManaged: "true",
		}
		if !t.IsZero() {
			tags[cloud.TagENICreatedAt] = t.Format(time.RFC3339)
		}
		return tags
	}

	setup := func(overrides map[string]interface{}) (*gomock.Controller, *awsService, *mock_aws.MockEC2) {
		ctrl := gomock.NewController(GinkgoT())
		mockEC2 := mock_aws.NewMockEC2(ctrl)
		service := &awsService{
			instanceId: instanceId,
			logger:     logger,
			ec2Svc:     mockEC2,
			imdsSvc:    getImdsService(overrides),
		}
		return ctrl, service, mockEC2
	}

	Context("initWithEC2Metadata", func() {
		It("", func() {
			_, service, _ := setup(nil)

			Expect(service.initWithEC2Metadata(context.Background())).Should(Succeed())
			Expect(service.securityGroups).Should(Equal([]string{securityGroupId}))
			Expect(service.instanceId).Should(Equal(instanceId))
			Expect(service.primaryENImac).Should(Equal(primaryMAC))
		})
	})

	Context("initInstanceTypeLimits", func() {
		It("", func() {
			_, service, mockEC2 := setup(nil)
			Expect(service.initWithEC2Metadata(context.Background())).Should(Succeed())
			Expect(service.instanceType).Should(Equal(instanceType))

			describeInstanceTypesInput := &ec2.DescribeInstanceTypesInput{
				InstanceTypes: []*string{
					aws.String(service.instanceType),
				},
			}
			describeInstanceTypeOutput := &ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []*ec2.InstanceTypeInfo{
					{
						InstanceType: aws.String(instanceType),
						NetworkInfo: &ec2.NetworkInfo{
							MaximumNetworkInterfaces:  aws.Int64(maxENI),
							Ipv4AddressesPerInterface: aws.Int64(maxIPsPerENI),
						},
					},
				},
			}
			mockEC2.EXPECT().DescribeInstanceTypesWithContext(gomock.Any(), describeInstanceTypesInput).Return(describeInstanceTypeOutput, nil)
			Expect(service.initInstanceTypeLimits()).Should(Succeed())
			Expect(service.eniLimit).Should(Equal(maxENI))
			Expect(service.eniIPv4Limit).Should(Equal(maxIPsPerENI))
		})
	})

	Context("DescribeAllENIs", func() {
		It("", func() {
			overrides := map[string]interface{}{
				metadataMACPath: strings.Join([]string{primaryMAC, eniMac2}, " "),
				metadataMACPath + eniMac2 + metadataDeviceNum:  eniDeviceIndex2,
				metadataMACPath + eniMac2 + metadataInterface:  eniId2,
				metadataMACPath + eniMac2 + metadataSubnetCIDR: subnet,
				metadataMACPath + eniMac2 + metadataIPv4s:      strings.Join([]string{eniIp21, eniIp22, eniIp23}, " "),
			}
			_, service, mockEC2 := setup(overrides)

			enis := &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(eniId1),
						Attachment: &ec2.NetworkInterfaceAttachment{
							NetworkCardIndex: aws.Int64(0),
							DeviceIndex:      aws.Int64(0),
						},
						PrivateIpAddresses: []*ec2.NetworkInterfacePrivateIpAddress{
							{
								Primary:          aws.Bool(true),
								PrivateIpAddress: aws.String(eniIp11),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIp12),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIp13),
							},
						},
						TagSet: convertTagsToSDKTags(getTags(time.Now())),
					},
					{
						NetworkInterfaceId: aws.String(eniId2),
						Attachment: &ec2.NetworkInterfaceAttachment{
							NetworkCardIndex: aws.Int64(0),
							DeviceIndex:      aws.Int64(1),
						},
						PrivateIpAddresses: []*ec2.NetworkInterfacePrivateIpAddress{
							{
								Primary:          aws.Bool(true),
								PrivateIpAddress: aws.String(eniIp21),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIp22),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIp23),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIp24),
							},
						},
						TagSet: convertTagsToSDKTags(getTags(time.Now())),
					},
				},
			}
			mockEC2.EXPECT().DescribeNetworkInterfacesWithContext(gomock.Any(), gomock.Any()).Return(enis, nil).AnyTimes()

			result, err := service.DescribeAllENIs()
			Expect(err).Should(BeNil())
			Expect(len(result)).Should(Equal(2))
			Expect(result[eniId2].PrimaryIPv4Address()).Should(Equal(eniIp21))

			var eni2 cloud.ENIMetadata
			imdsENIs, err := service.GetAttachedENIs()
			for _, item := range imdsENIs {
				if item.ENIId == eniId2 {
					eni2 = item
					break
				}
			}
			Expect(eni2).ShouldNot(BeNil())
			Expect(service.checkOutOfSyncState(eniId2, eni2.IPv4Addresses, enis.NetworkInterfaces[1].PrivateIpAddresses)).Should(BeFalse())
		})
	})

	Context("AllocENI", func() {
		It("", func() {
			_, service, mockEC2 := setup(nil)

			createInterfaceOutput := &ec2.CreateNetworkInterfaceOutput{
				NetworkInterface: &ec2.NetworkInterface{
					NetworkInterfaceId: aws.String(eniId3),
				},
			}
			mockEC2.EXPECT().CreateNetworkInterfaceWithContext(gomock.Any(), gomock.Any()).Return(createInterfaceOutput, nil)

			describeInstanceInput := &ec2.DescribeInstancesInput{
				InstanceIds: []*string{aws.String(instanceId)},
			}
			describeInstanceOutput := &ec2.DescribeInstancesOutput{
				Reservations: []*ec2.Reservation{
					{
						Instances: []*ec2.Instance{
							{
								InstanceId: aws.String(instanceId),
								NetworkInterfaces: []*ec2.InstanceNetworkInterface{
									{
										Attachment: &ec2.InstanceNetworkInterfaceAttachment{
											DeviceIndex: aws.Int64(0),
										},
									},
									{
										Attachment: &ec2.InstanceNetworkInterfaceAttachment{
											DeviceIndex: aws.Int64(1),
										},
									},
									{
										Attachment: &ec2.InstanceNetworkInterfaceAttachment{
											DeviceIndex: aws.Int64(2),
										},
									},
									{
										Attachment: &ec2.InstanceNetworkInterfaceAttachment{
											DeviceIndex: aws.Int64(4),
										},
									},
								},
							},
						},
					},
				},
			}
			mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), describeInstanceInput).Return(describeInstanceOutput, nil).AnyTimes()
			number, err := service.awsGetFreeDeviceNumber()
			Expect(err).Should(BeNil())
			Expect(number).Should(Equal(3))

			attachInput := &ec2.AttachNetworkInterfaceInput{
				DeviceIndex:        aws.Int64(int64(3)),
				InstanceId:         aws.String(instanceId),
				NetworkInterfaceId: aws.String(eniId3),
			}
			attachOutput := &ec2.AttachNetworkInterfaceOutput{
				AttachmentId: aws.String(attachmentId),
			}
			mockEC2.EXPECT().AttachNetworkInterfaceWithContext(gomock.Any(), attachInput).Return(attachOutput, nil)

			modifyAttributeInput := &ec2.ModifyNetworkInterfaceAttributeInput{
				Attachment: &ec2.NetworkInterfaceAttachmentChanges{
					AttachmentId:        aws.String(attachmentId),
					DeleteOnTermination: aws.Bool(true),
				},
				NetworkInterfaceId: aws.String(eniId3),
			}
			mockEC2.EXPECT().ModifyNetworkInterfaceAttributeWithContext(context.Background(), modifyAttributeInput).Return(nil, nil)
			eniId, err := service.AllocENI()
			Expect(err).Should(BeNil())
			Expect(eniId).Should(Equal(eniId3))
		})
	})

	Context("Wait for ENI attached", func() {
		It("", func() {
			_, service, _ := setup(nil)

			var err error
			_, err = service.WaitForENIAttached(eniId2)
			Expect(err).ShouldNot(BeNil())

			_, err = service.WaitForENIAttached(eniId1)
			Expect(err).Should(BeNil())
		})
	})

	Context("Free ENI", func() {
		It("", func() {
			_, service, mockEC2 := setup(nil)

			describeInterfaceInput := &ec2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: []*string{aws.String(eniId2)},
			}
			describeInterfaceOutput := &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(eniId2),
						TagSet:             convertTagsToSDKTags(getTags(time.Now())),
						Attachment: &ec2.NetworkInterfaceAttachment{
							AttachmentId: aws.String(attachmentId),
						},
					},
				},
			}
			mockEC2.EXPECT().DescribeNetworkInterfacesWithContext(gomock.Any(), describeInterfaceInput).Return(describeInterfaceOutput, nil)

			detachInput := &ec2.DetachNetworkInterfaceInput{
				AttachmentId: aws.String(attachmentId),
			}
			mockEC2.EXPECT().DetachNetworkInterfaceWithContext(gomock.Any(), detachInput).Return(nil, errors.New("mock detach failed"))
			mockEC2.EXPECT().DetachNetworkInterfaceWithContext(gomock.Any(), detachInput).Return(nil, nil)

			deleteInput := &ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: aws.String(eniId2),
			}
			mockEC2.EXPECT().DeleteNetworkInterfaceWithContext(gomock.Any(), deleteInput).Return(nil, errors.New("mock delete failed"))
			mockEC2.EXPECT().DeleteNetworkInterfaceWithContext(gomock.Any(), deleteInput).Return(nil, nil)
			Expect(service.FreeENI(eniId2)).Should(Succeed())
		})
	})

	Context("Clean leaked ENIs", func() {
		It("should be success without error", func() {
			_, service, mockEC2 := setup(nil)

			store := make(map[string]string)
			enis := &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(eniId1),
						Description:        aws.String("just created eni, should not be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Now())),
					},
					{
						NetworkInterfaceId: aws.String(eniId2),
						Description:        aws.String("expired leaked eni, should be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Now().Add(-1 * time.Hour))),
					},
					{
						Attachment: &ec2.NetworkInterfaceAttachment{
							AttachmentId: aws.String("test"),
						},
						NetworkInterfaceId: aws.String(eniId3),
						Description:        aws.String("eni attached to ec2, should not be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Now())),
					},
					{
						NetworkInterfaceId: aws.String(eniId4),
						Description:        aws.String("eni without created at tag, should not be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Time{})),
					},
				},
			}
			describeHookFn := func(ctx aws.Context, input *ec2.DescribeNetworkInterfacesInput, fn func(*ec2.DescribeNetworkInterfacesOutput, bool) bool, opts ...request.Option) error {
				fn(enis, true)
				return nil
			}
			mockEC2.EXPECT().DescribeNetworkInterfacesPagesWithContext(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(describeHookFn).Return(nil).AnyTimes()

			recordCreatedTagsRequest := func(ctx aws.Context, input *ec2.CreateTagsInput, opts ...request.Option) {
				for k, v := range convertSDKTagsToTags(input.Tags) {
					store[k] = v
				}
			}
			mockEC2.EXPECT().CreateTagsWithContext(gomock.Any(), gomock.Any()).DoAndReturn(recordCreatedTagsRequest).Return(nil, nil).AnyTimes()
			leakedENIs, err := service.findLeakedENis()
			Expect(err).Should(BeNil())
			Expect(len(leakedENIs)).Should(Equal(1))
			Expect(leakedENIs[0].NetworkInterfaceId).Should(Equal(aws.String(eniId2)))
			_, ok := store[cloud.TagENICreatedAt]
			Expect(ok).Should(BeTrue())

			deleted := make(map[string]bool)
			deleteHookFn := func(ctx aws.Context, input *ec2.DeleteNetworkInterfaceInput, opts ...request.Option) (*ec2.DeleteNetworkInterfaceOutput, error) {
				deleted[aws.StringValue(input.NetworkInterfaceId)] = true
				return nil, nil
			}
			mockEC2.EXPECT().DeleteNetworkInterfaceWithContext(gomock.Any(), gomock.Any()).DoAndReturn(deleteHookFn).Return(nil, nil)
			service.cleanLeakedENIs()
			Expect(deleted[eniId2]).ShouldNot(BeNil())
		})
	})

	Context("Alloc and dealloc private IP Addresses", func() {
		It("", func() {

			var err error
			_, service, mockEC2 := setup(nil)

			mockEC2.EXPECT().AssignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.AssignPrivateIpAddressesOutput{}, nil)
			_, err = service.AllocIPAddresses(eniId2)
			Expect(err).Should(BeNil())

			mockEC2.EXPECT().AssignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock assign failed"))
			_, err = service.AllocIPAddresses(eniId2)
			Expect(err).ShouldNot(BeNil())

			mockEC2.EXPECT().UnassignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.UnassignPrivateIpAddressesOutput{}, nil)
			Expect(service.DeallocIPAddresses(eniId2, []string{eniIp22})).Should(Succeed())

			mockEC2.EXPECT().UnassignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock unassign failed"))
			Expect(service.DeallocIPAddresses(eniId2, []string{eniIp22})).ShouldNot(Succeed())
		})
	})
})
