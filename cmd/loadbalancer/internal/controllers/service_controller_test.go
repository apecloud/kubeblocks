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
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/golang/mock/gomock"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent"
	mockagent "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent/mocks"
	mockcloud "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/mocks"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

const (
	namespace = "default"
	appName   = "service-controller-test"
	imageName = "nginx"

	node1IP   = "172.31.1.2"
	eniID1    = "eni-01"
	subnetID1 = "subnet-00000001"
	subnetID2 = "subnet-00000002"
	eniIP11   = "172.31.1.10"
	eniIP12   = "172.31.1.11"

	node2IP       = "172.31.1.1"
	eniID2        = "eni-02"
	eniIP21       = "172.31.2.10"
	eniIP22       = "172.31.2.11"
	svcPort       = 12345
	svcTargetPort = 80
)

var newSvcObj = func(managed bool, masterIP string) *corev1.Service {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	svcName := fmt.Sprintf("%s-%s", appName, randomStr)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      svcName,
			Labels: map[string]string{
				"app": svcName,
			},
			Annotations: map[string]string{},
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
				constant.AppNameLabelKey: appName,
			},
		},
	}
	if managed {
		svc.Annotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerTypePrivateIP
	} else {
		svc.Annotations[AnnotationKeyLoadBalancerType] = AnnotationValueLoadBalancerTypeNone
	}

	if masterIP != "" {
		svc.Annotations[AnnotationKeyMasterNodeIP] = masterIP
	}
	return svc
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

	var (
		ctrl            *gomock.Controller
		mockProvider    *mockcloud.MockProvider
		mockNodeManager *mockagent.MockNodeManager
		node1           agent.Node
		node2           agent.Node
	)

	buildENI := func(eniID string, subnetID string, ips []string) *protocol.ENIMetadata {
		if len(ips) == 0 {
			panic("can build ENI without private IPs")
		}
		result := protocol.ENIMetadata{
			EniId:    eniID,
			SubnetId: subnetID,
			Ipv4Addresses: []*protocol.IPv4Address{
				{
					Primary: true,
					Address: ips[0],
				},
			},
		}
		for _, ip := range ips {
			result.Ipv4Addresses = append(result.Ipv4Addresses, &protocol.IPv4Address{Primary: false, Address: ip})
		}
		return &result
	}
	setupController := func() {
		ctrl = gomock.NewController(GinkgoT())
		mockProvider = mockcloud.NewMockProvider(ctrl)
		mockNodeManager = mockagent.NewMockNodeManager(ctrl)

		setupNode := func(primaryIP string, subnetID string, eni *protocol.ENIMetadata) agent.Node {
			mockNode := mockagent.NewMockNode(ctrl)
			mockNode.EXPECT().GetIP().Return(primaryIP).AnyTimes()
			mockNode.EXPECT().GetNodeInfo().Return(&protocol.InstanceInfo{SubnetId: subnetID}).AnyTimes()
			mockNode.EXPECT().SetupNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockNode.EXPECT().CleanNetworkForService(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockNode.EXPECT().ChooseENI().Return(&protocol.ENIMetadata{EniId: eni.EniId, SubnetId: subnetID}, nil).AnyTimes()
			mockNode.EXPECT().GetManagedENIs().Return([]*protocol.ENIMetadata{eni}, nil).AnyTimes()
			mockNodeManager.EXPECT().GetNode(primaryIP).Return(mockNode, nil).AnyTimes()
			return mockNode
		}
		node1 = setupNode(node1IP, subnetID1, buildENI(eniID1, subnetID1, []string{eniIP11}))
		node2 = setupNode(node2IP, subnetID1, buildENI(eniID2, subnetID1, []string{eniIP21, eniIP22}))
		serviceController.cp = mockProvider
		serviceController.nm = mockNodeManager
		serviceController.initTrafficPolicies()

	}

	setupSpareNode := func(ip string) {
		var node agent.Node
		switch ip {
		case node1.GetIP():
			node = node1
		case node2.GetIP():
			node = node2
		default:
			panic("unknown node " + ip)
		}
		mockNodeManager.EXPECT().ChooseSpareNode(gomock.Any()).Return(node, nil).AnyTimes()
	}

	BeforeEach(func() {
		cleanEnv()

		setupController()
	})

	AfterEach(cleanEnv)

	It("Init nodes", func() {
		sc := &ServiceController{
			Client: k8sClient,
			logger: logger,
			cp:     mockProvider,
			nm:     mockNodeManager,
			cache:  make(map[string]*FloatingIP),
		}
		nodes := []agent.Node{node1, node2}
		mockNodeManager.EXPECT().GetNodes().Return(nodes, nil).AnyTimes()
		Expect(sc.initNodes(nodes)).Should(Succeed())
	})

	It("Create and migrate service", func() {
		setupSpareNode(node1IP)

		var (
			floatingIP = eniIP12
			oldENIId   = eniID1
			newENIId   = eniID2
		)

		By("By creating service")
		mockProvider.EXPECT().AllocIPAddresses(oldENIId).Return(floatingIP, nil).AnyTimes()

		svc := newSvcObj(true, node1.GetIP())
		svcKey := client.ObjectKey{Namespace: svc.GetNamespace(), Name: svc.GetName()}
		Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())

		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
				return false
			}
			return svc.Annotations[AnnotationKeyFloatingIP] == floatingIP
		}).Should(BeTrue())

		By("By migrating service")
		mockProvider.EXPECT().DeallocIPAddresses(oldENIId, gomock.Any()).Return(nil).AnyTimes()
		mockProvider.EXPECT().AssignPrivateIPAddresses(newENIId, floatingIP).Return(nil).AnyTimes()

		Expect(testapps.ChangeObj(&testCtx, svc, func() {
			svc.GetAnnotations()[AnnotationKeyMasterNodeIP] = node2.GetIP()
		})).Should(Succeed())
		Eventually(func() bool {
			Expect(k8sClient.Get(context.Background(), svcKey, svc)).Should(Succeed())
			return svc.Annotations[AnnotationKeyENIId] == newENIId
		}).Should(BeTrue())
		mockProvider.EXPECT().DeallocIPAddresses(newENIId, gomock.Any()).Return(nil).AnyTimes()
	})

	It("Creating service using cluster traffic policy", func() {
		setupSpareNode(node1IP)

		eni := &protocol.ENIMetadata{
			EniId:    eniID1,
			SubnetId: subnetID1,
		}
		mockProvider.EXPECT().AllocIPAddresses(eni.EniId).Return(eniIP12, nil)
		mockProvider.EXPECT().DeallocIPAddresses(eni.EniId, gomock.Any()).Return(nil).AnyTimes()

		svc := newSvcObj(true, "")
		svcKey := client.ObjectKey{Namespace: svc.GetNamespace(), Name: svc.GetName()}
		svc.GetAnnotations()[AnnotationKeyTrafficPolicy] = AnnotationValueClusterTrafficPolicy
		Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())

		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
				return false
			}
			return svc.Annotations[AnnotationKeyFloatingIP] == eniIP12
		}).Should(BeTrue())
	})

	It("Creating service using local traffic policy", func() {
		setupSpareNode(node1IP)

		mockProvider.EXPECT().DeallocIPAddresses(eniID1, gomock.Any()).Return(nil).AnyTimes()

		eni := &protocol.ENIMetadata{
			EniId:    eniID1,
			SubnetId: subnetID1,
		}
		mockProvider.EXPECT().AllocIPAddresses(eni.EniId).Return(eniIP12, nil).AnyTimes()
		mockProvider.EXPECT().AssignPrivateIPAddresses(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		// mock pods must be created before service, as pod.Status.hostIP is used as traffic node with policy BestEffortLocal
		pod := testapps.NewPodFactory(namespace, appName).
			AddLabels(constant.AppNameLabelKey, appName).
			AddContainer(corev1.Container{Name: appName, Image: imageName}).
			Create(&testCtx).
			GetObject()
		podKey := client.ObjectKey{Namespace: namespace, Name: pod.GetName()}
		Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
			pod.Status.HostIP = node1IP
		})).Should(Succeed())

		svc := newSvcObj(true, "")
		svcKey := client.ObjectKey{Namespace: namespace, Name: svc.GetName()}
		svc.GetAnnotations()[AnnotationKeyTrafficPolicy] = AnnotationValueBestEffortLocalTrafficPolicy
		Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())

		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), podKey, pod); err != nil {
				return false
			}
			if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
				return false
			}
			return pod.Status.HostIP != "" && svc.GetAnnotations()[AnnotationKeyENINodeIP] == pod.Status.HostIP
		}).Should(BeTrue())

		Expect(testapps.ChangeObj(&testCtx, svc, func() {
			svc.GetAnnotations()[AnnotationKeySubnetID] = subnetID2
		}))
		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), podKey, pod); err != nil {
				return false
			}
			if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
				return false
			}
			return pod.Status.HostIP != "" && svc.GetAnnotations()[AnnotationKeyENINodeIP] == pod.Status.HostIP
		}).Should(BeTrue())
	})

	It("Choose pod", func() {
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
