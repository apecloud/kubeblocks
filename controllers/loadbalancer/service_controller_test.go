package loadbalancer

import (
	"context"
	"fmt"
	"time"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
	mock_protocol "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol/mocks"

	"github.com/sethvargo/go-password/password"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	mockcloud "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/mocks"
)

var _ = Describe("ServiceController", func() {
	const (
		timeout       = 10 * time.Second
		interval      = 1 * time.Second
		svcPort       = 12345
		svcTargetPort = 80
		rpcPort       = 19200
		svcNamespace  = "default"
		node1IP       = "172.31.1.2"
		node2IP       = "172.31.1.1"
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

	resetController := func() (*gomock.Controller, *mockcloud.MockProvider, *mock_protocol.MockNodeCache) {
		ctrl := gomock.NewController(GinkgoT())

		mockProvider := mockcloud.NewMockProvider(ctrl)
		mockNodeCache := mock_protocol.NewMockNodeCache(ctrl)
		serviceController.cp = mockProvider
		serviceController.nc = mockNodeCache

		return ctrl, mockProvider, mockNodeCache
	}

	newSvcObj := func(managed bool, masterIP string) (*corev1.Service, *types.NamespacedName) {
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
		if masterIP != "" {
			annotations[AnnotationKeyMasterNodeIP] = masterIP
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

	Context("Get master node ip", func() {
		It("", func() {
			//var (
			//	role                            string
			//	err                             error
			//	svc, _                          = newSvcObj(false, false)
			//	ctrl_, mockCloud, mockNodeCache = resetController()
			//)
			//
			//role, err = serviceController.checkRoleForService(context.Background(), svc)
			//Expect(err).ToNot(HaveOccurred())
			//Expect(role == RoleOthers).Should(BeTrue())
			//
			//newMasterAnnotations := make(map[string]string)
			//newMasterAnnotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerType
			//newMasterAnnotations[AnnotationKeyMasterNodeIP] = node1IP
			//serviceController.hostIP = node1IP
			//svc.SetAnnotations(newMasterAnnotations)
			//role, err = serviceController.checkRoleForService(context.Background(), svc)
			//Expect(err).ToNot(HaveOccurred())
			//Expect(role == RoleNewMaster).Should(BeTrue())
			//
			//oldMasterAnnotations := make(map[string]string)
			//oldMasterAnnotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerType
			//oldMasterAnnotations[AnnotationKeyFloatingIP] = svcVIP
			//svc.SetAnnotations(oldMasterAnnotations)
			//serviceController.SetFloatingIP(svcVIP, &cloud.ENIMetadata{})
			//role, err = serviceController.checkRoleForService(context.Background(), svc)
			//Expect(err).ToNot(HaveOccurred())
			//Expect(role == RoleOldMaster).Should(BeTrue())
		})
	})

	Context("Create and delete service", func() {
		It("", func() {
			ctrl, mockCloud, mockNodeCache := resetController()

			By("By creating service")
			nodeClient := mock_protocol.NewMockNodeClient(ctrl)
			mockNodeCache.EXPECT().GetNode(node1IP).Return(nodeClient, nil).AnyTimes()

			chooseENIResponse := &protocol.ChooseBusiestENIResponse{
				Eni: &protocol.ENIMetadata{
					EniId: eniId1,
				},
			}
			nodeClient.EXPECT().ChooseBusiestENI(gomock.Any(), gomock.Any()).Return(chooseENIResponse, nil).AnyTimes()
			mockCloud.EXPECT().AllocIPAddresses(eniId1).Return(eniIp13, nil).AnyTimes()
			nodeClient.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

			svc, key := newSvcObj(true, node1IP)
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

			fetchedSvc := corev1.Service{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), *key, &fetchedSvc)
				return fetchedSvc.Annotations[AnnotationKeyFloatingIP] == eniIp13
			}, timeout, interval).Should(BeTrue())

			By("By deleting service")
			mockCloud.EXPECT().DeallocIPAddresses(eniId1, []string{eniIp13}).Return(nil).AnyTimes()
			nodeClient.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			Expect(k8sClient.Delete(context.Background(), svc)).Should(Succeed())
		})
	})

	Context("When update service, on old master", func() {
		It("Should success without error", func() {
			ctrl, mockCloud, mockNodeCache := resetController()
			fip := &FloatingIP{
				ip:     eniIp12,
				nodeIP: node1IP,
				eni: &protocol.ENIMetadata{
					EniId:          eniId1,
					Mac:            eniMac1,
					DeviceNumber:   2,
					SubnetIpv4Cidr: subnet,
					Ipv4Addresses: []*protocol.IPv4Address{
						{
							Primary: true,
							Address: eniIp11,
						},
						{
							Primary: false,
							Address: eniIp12,
						},
					},
				},
			}

			By("do works on new master")
			serviceController.SetFloatingIP(fip.ip, fip)
			mockCloud.EXPECT().DeallocIPAddresses(gomock.Any(), []string{fip.ip}).Return(nil).AnyTimes()
			mockNewNode := mock_protocol.NewMockNodeClient(ctrl)
			chooseENIResponse := &protocol.ChooseBusiestENIResponse{
				Eni: &protocol.ENIMetadata{
					EniId: eniId2,
				},
			}
			mockNewNode.EXPECT().ChooseBusiestENI(gomock.Any(), gomock.Any()).Return(chooseENIResponse, nil).AnyTimes()
			mockNodeCache.EXPECT().GetNode(node2IP).Return(mockNewNode, nil).AnyTimes()
			mockCloud.EXPECT().AssignPrivateIpAddresses(eniId2, fip.ip).Return(nil).AnyTimes()
			mockNewNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

			By("do works on old master")
			mockOldNode := mock_protocol.NewMockNodeClient(ctrl)
			mockOldNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockNodeCache.EXPECT().GetNode(node1IP).Return(mockOldNode, nil).AnyTimes()

			svc, key := newSvcObj(true, node2IP)
			svc.GetAnnotations()[AnnotationKeyFloatingIP] = eniIp12
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(context.Background(), *key, svc)).Should(Succeed())
				return svc.Annotations[AnnotationKeyENIId] == eniId2
			}, timeout, interval).Should(BeTrue())
		})
	})
})
