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
		svcNamespace  = "default"
		node1IP       = "172.31.1.2"
		node2IP       = "172.31.1.1"

		eniId1  = "eni-01"
		eniIp11 = "172.31.1.10"
		eniIp12 = "172.31.1.11"

		eniId2  = "eni-02"
		eniIp21 = "172.31.2.10"
		eniIp22 = "172.31.2.11"
	)

	setupController := func() (*gomock.Controller, *mockcloud.MockProvider, *mock_protocol.MockNodeCache) {
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

	Context("Init nodes", func() {
		It("", func() {
			ctrl := gomock.NewController(GinkgoT())
			mockCloud := mockcloud.NewMockProvider(ctrl)
			mockNodeCache := mock_protocol.NewMockNodeCache(ctrl)

			sc := &ServiceController{
				Client: k8sClient,
				logger: logger,
				cp:     mockCloud,
				nc:     mockNodeCache,
				cache:  make(map[string]*FloatingIP),
			}

			mockNode := mock_protocol.NewMockNodeClient(ctrl)
			mockNodeCache.EXPECT().GetNode(node1IP).Return(mockNode, nil).AnyTimes()

			getENIResponse := &protocol.GetManagedENIsResponse{
				Enis: map[string]*protocol.ENIMetadata{
					eniId1: {
						EniId: eniId1,
						Ipv4Addresses: []*protocol.IPv4Address{
							{
								Primary: true,
								Address: eniIp11,
							},
						},
					},
					eniId2: {
						EniId: eniId2,
						Ipv4Addresses: []*protocol.IPv4Address{
							{
								Primary: true,
								Address: eniIp21,
							},
							{
								Primary: false,
								Address: eniIp22,
							},
						},
					},
				},
			}
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{
						Status: corev1.NodeStatus{
							Addresses: []corev1.NodeAddress{
								{
									Type:    corev1.NodeInternalIP,
									Address: node1IP,
								},
							},
						},
					},
				},
			}
			mockNode.EXPECT().GetManagedENIs(gomock.Any(), gomock.Any()).Return(getENIResponse, nil).AnyTimes()
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			Expect(sc.initNodes(nodeList)).Should(Succeed())
		})
	})

	Context("Create and delete service", func() {
		It("", func() {
			ctrl, mockCloud, mockNodeCache := setupController()

			var (
				floatingIP        = eniIp12
				oldNodeIP         = node1IP
				newNodeIP         = node2IP
				oldENIId          = eniId1
				newENIId          = eniId2
				chooseENIResponse *protocol.ChooseBusiestENIResponse
			)

			By("By creating service")
			mockOldNode := mock_protocol.NewMockNodeClient(ctrl)
			mockNodeCache.EXPECT().GetNode(oldNodeIP).Return(mockOldNode, nil).AnyTimes()

			chooseENIResponse = &protocol.ChooseBusiestENIResponse{
				Eni: &protocol.ENIMetadata{
					EniId: oldENIId,
				},
			}
			mockOldNode.EXPECT().ChooseBusiestENI(gomock.Any(), gomock.Any()).Return(chooseENIResponse, nil)
			mockCloud.EXPECT().AllocIPAddresses(oldENIId).Return(floatingIP, nil)
			mockOldNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil)

			svc, key := newSvcObj(true, oldNodeIP)
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), *key, svc)
				return svc.Annotations[AnnotationKeyFloatingIP] == floatingIP
			}, timeout, interval).Should(BeTrue())

			By("By migrating service")
			mockCloud.EXPECT().DeallocIPAddresses(eniId1, []string{floatingIP}).Return(nil)
			mockNewNode := mock_protocol.NewMockNodeClient(ctrl)
			chooseENIResponse = &protocol.ChooseBusiestENIResponse{
				Eni: &protocol.ENIMetadata{
					EniId: newENIId,
				},
			}
			mockNewNode.EXPECT().ChooseBusiestENI(gomock.Any(), gomock.Any()).Return(chooseENIResponse, nil).AnyTimes()
			mockNewNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockNodeCache.EXPECT().GetNode(newNodeIP).Return(mockNewNode, nil).AnyTimes()
			mockCloud.EXPECT().AssignPrivateIpAddresses(newENIId, floatingIP).Return(nil).AnyTimes()
			mockOldNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

			svc.GetAnnotations()[AnnotationKeyMasterNodeIP] = newNodeIP
			Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(context.Background(), *key, svc)).Should(Succeed())
				return svc.Annotations[AnnotationKeyENIId] == newENIId
			}, timeout, interval).Should(BeTrue())

			By("By deleting service")
			mockCloud.EXPECT().DeallocIPAddresses(newENIId, []string{floatingIP}).Return(nil)
			mockNewNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			Expect(k8sClient.Delete(context.Background(), svc)).Should(Succeed())
		})
	})
})
