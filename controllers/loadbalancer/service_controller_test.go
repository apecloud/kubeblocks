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

package loadbalancer

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	mockagent "github.com/apecloud/kubeblocks/internal/loadbalancer/agent/mocks"
	mockcloud "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/mocks"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

var _ = Describe("ServiceController", Ordered, func() {
	const (
		timeout       = 10 * time.Second
		interval      = 1 * time.Second
		svcPort       = 12345
		svcTargetPort = 80
		namespace     = "default"
		node1IP       = "172.31.1.2"
		node2IP       = "172.31.1.1"

		eniId1  = "eni-01"
		eniIp11 = "172.31.1.10"
		eniIp12 = "172.31.1.11"

		eniId2  = "eni-02"
		eniIp21 = "172.31.2.10"
		eniIp22 = "172.31.2.11"
	)

	setupController := func() (*gomock.Controller, *mockcloud.MockProvider, *mockagent.MockNodeManager) {
		ctrl := gomock.NewController(GinkgoT())

		mockProvider := mockcloud.NewMockProvider(ctrl)
		mockNodeManager := mockagent.NewMockNodeManager(ctrl)
		serviceController.cp = mockProvider
		serviceController.nm = mockNodeManager
		serviceController.initTrafficPolicies()

		return ctrl, mockProvider, mockNodeManager
	}

	newPodObj := func(name string) (*corev1.Pod, *types.NamespacedName) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"app": name,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  name,
						Image: "nginx",
					},
				},
			},
		}
		return pod, &types.NamespacedName{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
		}
	}

	newSvcObj := func(managed bool, masterIP string) (*corev1.Service, *types.NamespacedName) {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		svcName := fmt.Sprintf("nginx-%s", randomStr)
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: namespace,
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
			mockNodeManager := mockagent.NewMockNodeManager(ctrl)

			sc := &ServiceController{
				Client: k8sClient,
				logger: logger,
				cp:     mockCloud,
				nm:     mockNodeManager,
				cache:  make(map[string]*FloatingIP),
			}

			mockNode := mockagent.NewMockNode(ctrl)
			mockNodeManager.EXPECT().GetNode(node1IP).Return(mockNode, nil).AnyTimes()

			getENIResponse := []*protocol.ENIMetadata{
				{
					EniId: eniId1,
					Ipv4Addresses: []*protocol.IPv4Address{
						{
							Primary: true,
							Address: eniIp11,
						},
					},
				},
				{
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
			mockNode.EXPECT().GetManagedENIs().Return(getENIResponse, nil).AnyTimes()
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			Expect(sc.initNodes(nodeList)).Should(Succeed())
		})
	})

	Context("Create and delete service", func() {
		It("", func() {
			ctrl, mockCloud, mockNodeManager := setupController()

			var (
				floatingIP = eniIp12
				oldNodeIP  = node1IP
				newNodeIP  = node2IP
				oldENIId   = eniId1
				newENIId   = eniId2
			)

			By("By creating service")
			mockOldNode := mockagent.NewMockNode(ctrl)
			mockNodeManager.EXPECT().GetNode(oldNodeIP).Return(mockOldNode, nil).AnyTimes()

			oldENI := &protocol.ENIMetadata{
				EniId: oldENIId,
			}
			mockOldNode.EXPECT().ChooseENI().Return(oldENI, nil)
			mockCloud.EXPECT().AllocIPAddresses(oldENIId).Return(floatingIP, nil)
			mockOldNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil)

			svc, key := newSvcObj(true, oldNodeIP)
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), *key, svc)
				return svc.Annotations[AnnotationKeyFloatingIP] == floatingIP
			}, timeout, interval).Should(BeTrue())

			By("By migrating service")
			mockCloud.EXPECT().DeallocIPAddresses(oldENIId, gomock.Any()).Return(nil).AnyTimes()
			mockNewNode := mockagent.NewMockNode(ctrl)
			newENI := &protocol.ENIMetadata{
				EniId: newENIId,
			}
			mockNewNode.EXPECT().ChooseENI().Return(newENI, nil).AnyTimes()
			mockNewNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockNodeManager.EXPECT().GetNode(newNodeIP).Return(mockNewNode, nil).AnyTimes()
			mockCloud.EXPECT().AssignPrivateIpAddresses(newENIId, floatingIP).Return(nil).AnyTimes()
			mockOldNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			svc.GetAnnotations()[AnnotationKeyMasterNodeIP] = newNodeIP
			Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(context.Background(), *key, svc)).Should(Succeed())
				return svc.Annotations[AnnotationKeyENIId] == newENIId
			}, timeout, interval).Should(BeTrue())

			By("By deleting service")
			mockCloud.EXPECT().DeallocIPAddresses(newENIId, gomock.Any()).Return(nil).AnyTimes()
			mockNewNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			Expect(k8sClient.Delete(context.Background(), svc)).Should(Succeed())
		})
	})

	Context("Creating service using cluster traffic policy", func() {
		It("", func() {
			ctrl, mockCloud, mockNodeManager := setupController()
			mockNode := mockagent.NewMockNode(ctrl)
			mockNode.EXPECT().GetIP().Return(node1IP)
			mockNodeManager.EXPECT().ChooseSpareNode(gomock.Any()).Return(mockNode, nil)
			mockNodeManager.EXPECT().GetNode(node1IP).Return(mockNode, nil).AnyTimes()

			eni := &protocol.ENIMetadata{
				EniId: eniId1,
			}
			mockNode.EXPECT().ChooseENI().Return(eni, nil)
			mockCloud.EXPECT().AllocIPAddresses(eni.EniId).Return(eniIp12, nil)
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil)

			svc, key := newSvcObj(true, "")
			svc.GetAnnotations()[AnnotationKeyTrafficPolicy] = AnnotationValueClusterTrafficPolicy
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), *key, svc)
				return svc.Annotations[AnnotationKeyFloatingIP] == eniIp12
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Creating service using local traffic policy", func() {
		It("", func() {
			ctrl, mockCloud, mockNodeManager := setupController()
			mockNode := mockagent.NewMockNode(ctrl)
			mockNodeManager.EXPECT().GetNode(gomock.Any()).Return(mockNode, nil).AnyTimes()

			eni := &protocol.ENIMetadata{
				EniId: eniId1,
			}
			mockNode.EXPECT().ChooseENI().Return(eni, nil).AnyTimes()
			mockCloud.EXPECT().AllocIPAddresses(eni.EniId).Return(eniIp12, nil).AnyTimes()
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			svc, svcKey := newSvcObj(true, "")
			svc.GetAnnotations()[AnnotationKeyTrafficPolicy] = AnnotationValueLocalTrafficPolicy
			Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

			pod, podKey := newPodObj(svc.GetName())
			Expect(k8sClient.Create(context.Background(), pod)).Should(Succeed())
			Eventually(func() bool {
				return k8sClient.Get(context.Background(), *podKey, pod) == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), *svcKey, svc)
				return svc.Annotations[AnnotationKeyENINodeIP] == pod.Status.HostIP
			}, timeout, interval).Should(BeTrue())
		})
	})
})
