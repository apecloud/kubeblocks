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

package aws

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	mockaws "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/aws/mocks"
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
		instanceID      = "i-0000000000000"
		instanceType    = "t3.medium"
		primaryMAC      = "00:00:00:00:00:01"
		securityGroupID = "sg-00000000"
		subnet          = "172.31.0.0/24"
		subnetID        = "subnet-00000000"
		attachmentID    = "eni-attach-00000000"

		eniID1          = "eni-01"
		eniDeviceIndex1 = "0"
		eniMac1         = "00:00:00:00:00:01"
		eniIP11         = "172.31.1.10"
		eniIP12         = "172.31.1.11"
		eniIP13         = "172.31.1.12"

		eniID2          = "eni-02"
		eniMac2         = "00:00:00:00:00:02"
		eniDeviceIndex2 = "1"
		eniIP21         = "172.31.2.10"
		eniIP22         = "172.31.2.11"
		eniIP23         = "172.31.2.12"
		eniIP24         = "172.31.2.14"

		eniID3          = "eni-03"
		eniMac3         = "00:00:00:00:00:03"
		eniDeviceIndex3 = "2"
		eniIP31         = "172.31.3.10"

		eniID4 = "eni-04"
	)

	getImdsService := func(overrides map[string]interface{}) imdsService {
		data := map[string]interface{}{
			metadataAZ:           az,
			metadataLocalIP:      localIP,
			metadataInstanceID:   instanceID,
			metadataInstanceType: instanceType,
			metadataMAC:          primaryMAC,
			metadataMACPath:      primaryMAC,
			metadataMACPath + primaryMAC + metadataDeviceNum:  eniDeviceIndex1,
			metadataMACPath + primaryMAC + metadataInterface:  eniID1,
			metadataMACPath + primaryMAC + metadataSGs:        securityGroupID,
			metadataMACPath + primaryMAC + metadataIPv4s:      strings.Join([]string{eniIP11, eniIP12, eniIP13}, " "),
			metadataMACPath + primaryMAC + metadataSubnetID:   subnetID,
			metadataMACPath + primaryMAC + metadataSubnetCIDR: subnet,
			metadataMACPath + primaryMAC + metadataVPCcidrs:   subnet,
		}

		for k, v := range overrides {
			data[k] = v
		}
		return imdsService{fakeIMDS(data)}
	}

	getTags := func(t time.Time) map[string]string {
		tags := map[string]string{
			cloud.TagENINode:              instanceID,
			cloud.TagENIKubeBlocksManaged: "true",
		}
		if !t.IsZero() {
			tags[cloud.TagENICreatedAt] = t.Format(time.RFC3339)
		}
		return tags
	}

	setup := func(overrides map[string]interface{}) (*gomock.Controller, *awsService, *mockaws.MockEC2) {
		ctrl := gomock.NewController(GinkgoT())
		mockEC2 := mockaws.NewMockEC2(ctrl)
		service := &awsService{
			instanceID: instanceID,
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
			Expect(service.securityGroups).Should(Equal([]string{securityGroupID}))
			Expect(service.instanceID).Should(Equal(instanceID))
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
				metadataMACPath + eniMac2 + metadataInterface:  eniID2,
				metadataMACPath + eniMac2 + metadataSubnetCIDR: subnet,
				metadataMACPath + eniMac2 + metadataIPv4s:      strings.Join([]string{eniIP21, eniIP22, eniIP23}, " "),
			}
			_, service, mockEC2 := setup(overrides)

			enis := &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(eniID1),
						Attachment: &ec2.NetworkInterfaceAttachment{
							NetworkCardIndex: aws.Int64(0),
							DeviceIndex:      aws.Int64(0),
						},
						PrivateIpAddresses: []*ec2.NetworkInterfacePrivateIpAddress{
							{
								Primary:          aws.Bool(true),
								PrivateIpAddress: aws.String(eniIP11),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIP12),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIP13),
							},
						},
						TagSet: convertTagsToSDKTags(getTags(time.Now())),
					},
					{
						NetworkInterfaceId: aws.String(eniID2),
						Attachment: &ec2.NetworkInterfaceAttachment{
							NetworkCardIndex: aws.Int64(0),
							DeviceIndex:      aws.Int64(1),
						},
						PrivateIpAddresses: []*ec2.NetworkInterfacePrivateIpAddress{
							{
								Primary:          aws.Bool(true),
								PrivateIpAddress: aws.String(eniIP21),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIP22),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIP23),
							},
							{
								Primary:          aws.Bool(false),
								PrivateIpAddress: aws.String(eniIP24),
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
			Expect(result[eniID2].PrimaryIPv4Address()).Should(Equal(eniIP21))

			var eni2 cloud.ENIMetadata
			imdsENIs, err := service.GetAttachedENIs()
			for _, item := range imdsENIs {
				if item.ID == eniID2 {
					eni2 = item
					break
				}
			}
			Expect(err).Should(BeNil())
			Expect(eni2).ShouldNot(BeNil())
			Expect(service.checkOutOfSyncState(eniID2, eni2.IPv4Addresses, enis.NetworkInterfaces[1].PrivateIpAddresses)).Should(BeFalse())
		})
	})

	Context("Create ENI", func() {
		It("", func() {
			_, service, mockEC2 := setup(nil)

			createInterfaceOutput := &ec2.CreateNetworkInterfaceOutput{
				NetworkInterface: &ec2.NetworkInterface{
					NetworkInterfaceId: aws.String(eniID3),
				},
			}
			mockEC2.EXPECT().CreateNetworkInterfaceWithContext(gomock.Any(), gomock.Any()).Return(createInterfaceOutput, nil)

			eniID, err := service.CreateENI(instanceID, subnetID, []string{securityGroupID})
			Expect(err).Should(BeNil())
			Expect(eniID).Should(Equal(eniID3))
		})
	})

	Context("Wait for ENI attached", func() {
		It("", func() {
			_, service, _ := setup(nil)

			var err error
			_, err = service.WaitForENIAttached(eniID2)
			Expect(err).ShouldNot(BeNil())

			_, err = service.WaitForENIAttached(eniID1)
			Expect(err).Should(BeNil())
		})
	})

	Context("Free ENI", func() {
		It("", func() {
			_, service, mockEC2 := setup(nil)

			describeInterfaceInput := &ec2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: []*string{aws.String(eniID2)},
			}
			describeInterfaceOutput := &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(eniID2),
						TagSet:             convertTagsToSDKTags(getTags(time.Now())),
						Attachment: &ec2.NetworkInterfaceAttachment{
							AttachmentId: aws.String(attachmentID),
						},
					},
				},
			}
			mockEC2.EXPECT().DescribeNetworkInterfacesWithContext(gomock.Any(), describeInterfaceInput).Return(describeInterfaceOutput, nil)

			detachInput := &ec2.DetachNetworkInterfaceInput{
				AttachmentId: aws.String(attachmentID),
			}
			mockEC2.EXPECT().DetachNetworkInterfaceWithContext(gomock.Any(), detachInput).Return(nil, errors.New("mock detach failed"))
			mockEC2.EXPECT().DetachNetworkInterfaceWithContext(gomock.Any(), detachInput).Return(nil, nil)

			deleteInput := &ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: aws.String(eniID2),
			}
			mockEC2.EXPECT().DeleteNetworkInterfaceWithContext(gomock.Any(), deleteInput).Return(nil, errors.New("mock delete failed"))
			mockEC2.EXPECT().DeleteNetworkInterfaceWithContext(gomock.Any(), deleteInput).Return(nil, nil)
			Expect(service.FreeENI(eniID2)).Should(Succeed())
		})
	})

	Context("Clean leaked ENIs", func() {
		It("should be success without error", func() {
			_, service, mockEC2 := setup(nil)

			store := make(map[string]string)
			enis := &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						NetworkInterfaceId: aws.String(eniID1),
						Description:        aws.String("just created eni, should not be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Now())),
					},
					{
						NetworkInterfaceId: aws.String(eniID2),
						Description:        aws.String("expired leaked eni, should be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Now().Add(-1 * time.Hour))),
					},
					{
						Attachment: &ec2.NetworkInterfaceAttachment{
							AttachmentId: aws.String("test"),
						},
						NetworkInterfaceId: aws.String(eniID3),
						Description:        aws.String("eni attached to ec2, should not be cleaned"),
						TagSet:             convertTagsToSDKTags(getTags(time.Now())),
					},
					{
						NetworkInterfaceId: aws.String(eniID4),
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
			leakedENIs, err := service.FindLeakedENIs(instanceID)
			Expect(err).Should(BeNil())
			Expect(len(leakedENIs)).Should(Equal(1))
			Expect(leakedENIs[0].ID).Should(Equal(eniID2))
			_, ok := store[cloud.TagENICreatedAt]
			Expect(ok).Should(BeTrue())
		})
	})

	Context("Alloc and dealloc private IP Addresses", func() {
		It("", func() {

			var err error
			_, service, mockEC2 := setup(nil)

			assignOutput := &ec2.AssignPrivateIpAddressesOutput{
				AssignedPrivateIpAddresses: []*ec2.AssignedPrivateIpAddress{
					{
						PrivateIpAddress: aws.String(eniIP22),
					},
				},
			}
			mockEC2.EXPECT().AssignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(assignOutput, nil)
			_, err = service.AllocIPAddresses(eniID2)
			Expect(err).Should(BeNil())

			mockEC2.EXPECT().AssignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock assign failed"))
			_, err = service.AllocIPAddresses(eniID2)
			Expect(err).ShouldNot(BeNil())

			mockEC2.EXPECT().UnassignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.UnassignPrivateIpAddressesOutput{}, nil)
			Expect(service.DeallocIPAddresses(eniID2, []string{eniIP22})).Should(Succeed())

			mockEC2.EXPECT().UnassignPrivateIpAddressesWithContext(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock unassign failed"))
			Expect(service.DeallocIPAddresses(eniID2, []string{eniIP22})).ShouldNot(Succeed())
		})
	})
})
