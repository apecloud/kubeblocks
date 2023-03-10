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

package loadbalancer

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent"
	mockagent "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent/mocks"
	mockcloud "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/mocks"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

const (
	timeout       = 10 * time.Second
	interval      = 1 * time.Second
	svcPort       = 12345
	svcTargetPort = 80
	namespace     = "default"
	node1IP       = "172.31.1.2"
	node2IP       = "172.31.1.1"

	eniID1    = "eni-01"
	subnetID1 = "subnet-00000001"
	subnetID2 = "subnet-00000002"
	eniIP11   = "172.31.1.10"
	eniIP12   = "172.31.1.11"

	eniID2  = "eni-02"
	eniIP21 = "172.31.2.10"
	eniIP22 = "172.31.2.11"
)

var newSvcObj = func(managed bool, masterIP string) (*corev1.Service, types.NamespacedName) {
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
		annotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerTypePrivateIP
	} else {
		annotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerTypeNone
	}
	if masterIP != "" {
		annotations[AnnotationKeyMasterNodeIP] = masterIP
	}
	svc.SetAnnotations(annotations)
	return svc, types.NamespacedName{
		Name:      svc.GetName(),
		Namespace: svc.GetNamespace(),
	}
}

var _ = Describe("ServiceController", Ordered, func() {

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ServiceSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.EndpointsSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

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
			mockNode.EXPECT().GetIP().Return(node1IP).AnyTimes()

			nodes := []agent.Node{mockNode}
			mockNodeManager.EXPECT().GetNodes().Return(nodes, nil).AnyTimes()
			mockNodeManager.EXPECT().GetNode(node1IP).Return(mockNode, nil).AnyTimes()

			getENIResponse := []*protocol.ENIMetadata{
				{
					EniId:    eniID1,
					SubnetId: subnetID1,
					Ipv4Addresses: []*protocol.IPv4Address{
						{
							Primary: true,
							Address: eniIP11,
						},
					},
				},
				{
					EniId:    eniID2,
					SubnetId: subnetID1,
					Ipv4Addresses: []*protocol.IPv4Address{
						{
							Primary: true,
							Address: eniIP21,
						},
						{
							Primary: false,
							Address: eniIP22,
						},
					},
				},
			}
			mockNode.EXPECT().GetManagedENIs().Return(getENIResponse, nil).AnyTimes()
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			Expect(sc.initNodes(nodes)).Should(Succeed())
		})
	})

	Context("Create and delete service", func() {
		It("", func() {
			ctrl, mockCloud, mockNodeManager := setupController()

			var (
				floatingIP = eniIP12
				oldNodeIP  = node1IP
				newNodeIP  = node2IP
				oldENIId   = eniID1
				newENIId   = eniID2
			)

			By("By creating service")
			mockOldNode := mockagent.NewMockNode(ctrl)
			mockNodeManager.EXPECT().GetNode(oldNodeIP).Return(mockOldNode, nil).AnyTimes()

			oldENI := &protocol.ENIMetadata{
				EniId:    oldENIId,
				SubnetId: subnetID1,
			}
			mockOldNode.EXPECT().ChooseENI().Return(oldENI, nil).AnyTimes()
			mockCloud.EXPECT().AllocIPAddresses(oldENIId).Return(floatingIP, nil).AnyTimes()
			mockOldNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			svc, key := newSvcObj(true, oldNodeIP)
			Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), key, svc); err != nil {
					return false
				}
				return svc.Annotations[AnnotationKeyFloatingIP] == floatingIP
			}, timeout, interval).Should(BeTrue())

			By("By migrating service")
			mockCloud.EXPECT().DeallocIPAddresses(oldENIId, gomock.Any()).Return(nil).AnyTimes()
			mockNewNode := mockagent.NewMockNode(ctrl)
			newENI := &protocol.ENIMetadata{
				EniId:    newENIId,
				SubnetId: subnetID1,
			}
			mockNewNode.EXPECT().ChooseENI().Return(newENI, nil).AnyTimes()
			mockNewNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockNodeManager.EXPECT().GetNode(newNodeIP).Return(mockNewNode, nil).AnyTimes()
			mockCloud.EXPECT().AssignPrivateIPAddresses(newENIId, floatingIP).Return(nil).AnyTimes()
			mockOldNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			svc.GetAnnotations()[AnnotationKeyMasterNodeIP] = newNodeIP
			Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(context.Background(), key, svc)).Should(Succeed())
				return svc.Annotations[AnnotationKeyENIId] == newENIId
			}, timeout, interval).Should(BeTrue())

			By("By deleting service")
			mockCloud.EXPECT().DeallocIPAddresses(newENIId, gomock.Any()).Return(nil).AnyTimes()
			mockNewNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		})
	})

	Context("Creating service using cluster traffic policy", func() {
		It("", func() {
			ctrl, mockProvider, mockNodeManager := setupController()
			mockNode := mockagent.NewMockNode(ctrl)
			mockNode.EXPECT().GetIP().Return(node1IP)
			mockNodeManager.EXPECT().ChooseSpareNode(gomock.Any()).Return(mockNode, nil)
			mockNodeManager.EXPECT().GetNode(node1IP).Return(mockNode, nil).AnyTimes()

			eni := &protocol.ENIMetadata{
				EniId:    eniID1,
				SubnetId: subnetID1,
			}
			mockNode.EXPECT().ChooseENI().Return(eni, nil)
			mockProvider.EXPECT().AllocIPAddresses(eni.EniId).Return(eniIP12, nil)
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil)

			svc, key := newSvcObj(true, "")
			svc.GetAnnotations()[AnnotationKeyTrafficPolicy] = AnnotationValueClusterTrafficPolicy
			Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), key, svc); err != nil {
					return false
				}
				return svc.Annotations[AnnotationKeyFloatingIP] == eniIP12
			}, timeout, interval).Should(BeTrue())

			mockProvider.EXPECT().DeallocIPAddresses(eniID1, gomock.Any()).Return(nil).AnyTimes()
			mockNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		})
	})

	Context("Creating service using local traffic policy", func() {
		It("", func() {
			ctrl, mockProvider, mockNodeManager := setupController()
			mockNode := mockagent.NewMockNode(ctrl)
			info := &protocol.InstanceInfo{
				SubnetId: subnetID1,
			}
			mockNode.EXPECT().GetIP().Return(node1IP).AnyTimes()
			mockNode.EXPECT().GetNodeInfo().Return(info).AnyTimes()
			mockNodeManager.EXPECT().GetNode(gomock.Any()).Return(mockNode, nil).AnyTimes()
			mockNodeManager.EXPECT().ChooseSpareNode(gomock.Any()).Return(mockNode, nil).AnyTimes()

			mockProvider.EXPECT().DeallocIPAddresses(eniID1, gomock.Any()).Return(nil).AnyTimes()
			mockNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			eni := &protocol.ENIMetadata{
				EniId:    eniID1,
				SubnetId: subnetID1,
			}
			mockNode.EXPECT().ChooseENI().Return(eni, nil).AnyTimes()
			mockProvider.EXPECT().AllocIPAddresses(eni.EniId).Return(eniIP12, nil).AnyTimes()
			mockProvider.EXPECT().AssignPrivateIPAddresses(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			svc, svcKey := newSvcObj(true, "")
			svc.GetAnnotations()[AnnotationKeyTrafficPolicy] = AnnotationValueBestEffortLocalTrafficPolicy
			Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())
			pod, podKey := newPodObj(svc.GetName())
			Expect(testCtx.CreateObj(context.Background(), pod)).Should(Succeed())

			patch := client.MergeFrom(pod.DeepCopy())
			pod.Status.HostIP = node1IP
			Expect(k8sClient.Status().Patch(context.Background(), pod, patch)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), *podKey, pod); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
					return false
				}
				return pod.Status.HostIP != "" && svc.GetAnnotations()[AnnotationKeyENINodeIP] == pod.Status.HostIP
			}, timeout, interval).Should(BeTrue())

			svc.GetAnnotations()[AnnotationKeySubnetID] = subnetID2
			Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), *podKey, pod); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
					return false
				}
				return pod.Status.HostIP != "" && svc.GetAnnotations()[AnnotationKeyENINodeIP] == pod.Status.HostIP
			}, timeout, interval).Should(BeTrue())

			By("release resources")
		})
	})

	Context("Choose pod", func() {
		It("", func() {
			t := time.Now()
			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.NewTime(t.Add(-20 * time.Second)),
						},
						Status: corev1.PodStatus{
							HostIP: node2IP,
							Phase:  corev1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.NewTime(t.Add(-30 * time.Second)),
						},
						Status: corev1.PodStatus{
							HostIP: node1IP,
							Phase:  corev1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.NewTime(t.Add(-10 * time.Second)),
						},
						Status: corev1.PodStatus{
							HostIP: node1IP,
							Phase:  corev1.PodPending,
						},
					},
				},
			}
			pod := LocalTrafficPolicy{}.choosePod(pods)
			Expect(pod.Status.HostIP).Should(Equal(node1IP))
		})
	})
})
