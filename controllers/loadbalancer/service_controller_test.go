package loadbalancer

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-password/password"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	mock_cloud "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/mocks"
	mock_network "github.com/apecloud/kubeblocks/internal/loadbalancer/network/mocks"
)

var _ = Describe("ServiceController", func() {
	const (
		timeout       = 10 * time.Second
		interval      = 1 * time.Second
		svcPort       = 12345
		svcTargetPort = 80
		svcNamespace  = "default"
		masterHostIP  = "172.31.1.2"
		localHostIP   = "172.31.1.1"
		svcVIP        = "172.31.1.100"

		subnet = "172.31.0.0/16"

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
	)

	resetController := func() (*mock_cloud.MockProvider, *mock_network.MockClient) {
		ctrl := gomock.NewController(GinkgoT())

		mockService := mock_cloud.NewMockProvider(ctrl)
		mockNetwork := mock_network.NewMockClient(ctrl)
		serviceController.maxIPsPerENI = 5
		serviceController.maxENI = 2
		serviceController.minPrivateIP = 2
		serviceController.hostIP = localHostIP
		serviceController.cloud = mockService
		serviceController.nc = mockNetwork
		serviceController.cachedENIs = make(map[string]*cloud.ENIMetadata)

		return mockService, mockNetwork
	}

	getMockENIs := func() map[string]*cloud.ENIMetadata {
		return map[string]*cloud.ENIMetadata{
			eniId1: {
				ENIId:          eniId1,
				MAC:            eniMac1,
				DeviceNumber:   0,
				SubnetIPv4CIDR: subnet,
				IPv4Addresses: []*ec2.NetworkInterfacePrivateIpAddress{
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp11),
					},
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp12),
					},
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp13),
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
				IPv4Addresses: []*ec2.NetworkInterfacePrivateIpAddress{
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp21),
					},
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp22),
					},
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp23),
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
				IPv4Addresses: []*ec2.NetworkInterfacePrivateIpAddress{
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp31),
					},
					{
						Primary:          aws.Bool(true),
						PrivateIpAddress: aws.String(eniIp32),
					},
				},
			},
		}
	}

	newSvcObj := func(managed bool, master bool) (*corev1.Service, *types.NamespacedName) {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		svcName := fmt.Sprintf("nginx-%s", randomStr)
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: svcNamespace,
				Labels: map[string]string{
					"app": svcName,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       svcPort,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(svcTargetPort),
					},
				},
				Selector: map[string]string{
					"app": svcName,
				},
			},
		}
		annotations := make(map[string]string)
		if managed {
			annotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerType
		}
		if master {
			annotations[AnnotationKeyMasterHost] = localHostIP
		}
		svc.SetAnnotations(annotations)
		return svc, &types.NamespacedName{
			Name:      svc.GetName(),
			Namespace: svc.GetNamespace(),
		}
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("Check local role for service", func() {
		It("Should success with no error", func() {
			var (
				role   string
				err    error
				svc, _ = newSvcObj(false, false)
			)

			role, err = serviceController.checkRoleForService(context.Background(), svc)
			Expect(err).ToNot(HaveOccurred())
			Expect(role == RoleOthers).Should(BeTrue())

			newMasterAnnotations := make(map[string]string)
			newMasterAnnotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerType
			newMasterAnnotations[AnnotationKeyMasterHost] = masterHostIP
			serviceController.hostIP = masterHostIP
			svc.SetAnnotations(newMasterAnnotations)
			role, err = serviceController.checkRoleForService(context.Background(), svc)
			Expect(err).ToNot(HaveOccurred())
			Expect(role == RoleNewMaster).Should(BeTrue())

			oldMasterAnnotations := make(map[string]string)
			oldMasterAnnotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerType
			oldMasterAnnotations[AnnotationKeyPrivateIP] = svcVIP
			svc.SetAnnotations(oldMasterAnnotations)
			serviceController.putCachedENI(svcVIP, &cloud.ENIMetadata{})
			role, err = serviceController.checkRoleForService(context.Background(), svc)
			Expect(err).ToNot(HaveOccurred())
			Expect(role == RoleOldMaster).Should(BeTrue())
		})
	})

	Context("When create service", func() {
		It("Should success with no error", func() {
			mockCloud, mockNetwork := resetController()

			By("By creating service")
			enis := getMockENIs()
			mockCloud.EXPECT().DescribeAllENIs().Return(enis, nil)
			response := &ec2.AssignPrivateIpAddressesOutput{
				AssignedPrivateIpAddresses: []*ec2.AssignedPrivateIpAddress{
					{
						PrivateIpAddress: aws.String(eniIp24),
					},
				},
			}
			mockCloud.EXPECT().AllocIPAddresses(eniId2).Return(response, nil)
			mockNetwork.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil)
			svc, key := newSvcObj(true, true)
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(context.Background(), *key, svc)).Should(Succeed())
				return svc.Annotations[AnnotationKeyPrivateIP] == eniIp24
			}, timeout, interval).Should(BeTrue())

			By("By deleting service")
			mockCloud.EXPECT().DeallocIPAddresses(eniId2, []string{eniIp24}).Return(nil)
			mockNetwork.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil)
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), svc)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When update service, on new master", func() {
		It("Should success without error", func() {
		})
	})

	Context("When update service, on old master", func() {
		It("Should success without error", func() {
			_, mockNetwork := resetController()
			eni := &cloud.ENIMetadata{ENIId: eniId2}
			mockNetwork.EXPECT().CleanNetworkForService(eniIp24, eni).Return(nil)
			mockNetwork.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil)

			By("By creating service")
			svc, _ := newSvcObj(true, false)
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

			svc.GetAnnotations()[AnnotationKeyENIId] = eniId2
			svc.GetAnnotations()[AnnotationKeyPrivateIP] = eniIp24
			svc.GetAnnotations()[AnnotationKeyMasterHost] = masterHostIP
			serviceController.putCachedENI(eniIp24, eni)
			Expect(serviceController.getCachedENI(eniIp24) != nil).Should(BeTrue())
			Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())

			Eventually(func() bool {
				return serviceController.getCachedENI(eniIp24) == nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
