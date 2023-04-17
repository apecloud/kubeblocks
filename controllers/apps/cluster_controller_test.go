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

package apps

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

const backupPolicyTPLName = "test-backup-policy-template-mysql"

var _ = Describe("Cluster Controller", func() {
	const (
		clusterDefName         = "test-clusterdef"
		clusterVersionName     = "test-clusterversion"
		clusterNamePrefix      = "test-cluster"
		statelessCompName      = "stateless"
		statelessCompDefName   = "stateless"
		statefulCompName       = "stateful"
		statefulCompDefName    = "stateful"
		consensusCompName      = "consensus"
		consensusCompDefName   = "consensus"
		replicationCompName    = "replication"
		replicationCompDefName = "replication"
		leader                 = "leader"
		follower               = "follower"
	)

	var (
		randomStr              = testCtx.GetRandomStr()
		clusterNameRand        = "mysql-" + randomStr
		clusterDefNameRand     = "mysql-definition-" + randomStr
		clusterVersionNameRand = "mysql-cluster-version-" + randomStr
	)

	// Cleanups
	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PodSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	// Test cases

	waitForCreatingResourceCompletely := func(clusterKey client.ObjectKey, compNames ...string) {
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		for _, compName := range compNames {
			Eventually(testapps.GetClusterComponentPhase(testCtx, clusterKey.Name, compName)).Should(Equal(appsv1alpha1.CreatingClusterCompPhase))
		}
	}

	checkAllResourcesCreated := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(replicationCompName, replicationCompDefName).SetReplicas(3).
			AddComponent(statelessCompName, statelessCompDefName).SetReplicas(3).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		By("Check deployment workload has been created")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.DeploymentSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey: clusterKey.Name,
			}, client.InNamespace(clusterKey.Namespace))).ShouldNot(BeEquivalentTo(0))

		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)

		By("Check statefulset pod's volumes")
		for _, sts := range stsList.Items {
			podSpec := sts.Spec.Template
			volumeNames := map[string]struct{}{}
			for _, v := range podSpec.Spec.Volumes {
				volumeNames[v.Name] = struct{}{}
			}

			for _, cc := range [][]corev1.Container{
				podSpec.Spec.Containers,
				podSpec.Spec.InitContainers,
			} {
				for _, c := range cc {
					for _, vm := range c.VolumeMounts {
						_, ok := volumeNames[vm.Name]
						Expect(ok).Should(BeTrue())
					}
				}
			}
		}

		By("Check associated PDB has been created")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.PodDisruptionBudgetSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey: clusterKey.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(0))

		podSpec := stsList.Items[0].Spec.Template.Spec
		By("Checking created sts pods template with built-in toleration")
		Expect(podSpec.Tolerations).Should(HaveLen(1))
		Expect(podSpec.Tolerations[0].Key).To(Equal(constant.KubeBlocksDataNodeTolerationKey))

		By("Checking created sts pods template with built-in Affinity")
		Expect(podSpec.Affinity.PodAntiAffinity == nil && podSpec.Affinity.PodAffinity == nil).Should(BeTrue())
		Expect(podSpec.Affinity.NodeAffinity).ShouldNot(BeNil())
		Expect(podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key).To(
			Equal(constant.KubeBlocksDataNodeLabelKey))

		By("Checking created sts pods template without TopologySpreadConstraints")
		Expect(podSpec.TopologySpreadConstraints).Should(BeEmpty())

		By("Check should create env configmap")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.ConfigMapSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey:   clusterKey.Name,
				constant.AppConfigTypeLabelKey: "kubeblocks-env",
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(2))

		By("Make sure the cluster controller has set the cluster status to Running")
		for i, comp := range clusterObj.Spec.ComponentSpecs {
			if comp.ComponentDefRef != replicationCompDefName || comp.Name != replicationCompName {
				continue
			}
			stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj), clusterObj.Spec.ComponentSpecs[i].Name)
			for _, v := range stsList.Items {
				Expect(testapps.ChangeObjStatus(&testCtx, &v, func() {
					testk8s.MockStatefulSetReady(&v)
				})).ShouldNot(HaveOccurred())
			}
		}
	}

	type ExpectService struct {
		headless bool
		svcType  corev1.ServiceType
	}

	getHeadlessSvcPorts := func(g Gomega, compDefName string) []corev1.ServicePort {
		comp, err := util.GetComponentDefByCluster(testCtx.Ctx, k8sClient, *clusterObj, compDefName)
		g.Expect(err).ShouldNot(HaveOccurred())
		var headlessSvcPorts []corev1.ServicePort
		for _, container := range comp.PodSpec.Containers {
			for _, port := range container.Ports {
				// be consistent with headless_service_template.cue
				headlessSvcPorts = append(headlessSvcPorts, corev1.ServicePort{
					Name:       port.Name,
					Protocol:   port.Protocol,
					Port:       port.ContainerPort,
					TargetPort: intstr.FromString(port.Name),
				})
			}
		}
		return headlessSvcPorts
	}

	validateCompSvcList := func(g Gomega, compName string, compType string, expectServices map[string]ExpectService) {
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		svcList := &corev1.ServiceList{}
		g.Expect(k8sClient.List(testCtx.Ctx, svcList, client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: compName,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())

		for svcName, svcSpec := range expectServices {
			idx := slices.IndexFunc(svcList.Items, func(e corev1.Service) bool {
				return strings.HasSuffix(e.Name, svcName)
			})
			g.Expect(idx >= 0).To(BeTrue())
			svc := svcList.Items[idx]
			g.Expect(svc.Spec.Type).Should(Equal(svcSpec.svcType))
			switch {
			case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
				g.Expect(svc.Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && !svcSpec.headless:
				g.Expect(svc.Spec.ClusterIP).ShouldNot(Equal(corev1.ClusterIPNone))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && svcSpec.headless:
				g.Expect(svc.Spec.ClusterIP).Should(Equal(corev1.ClusterIPNone))
				g.Expect(reflect.DeepEqual(svc.Spec.Ports, getHeadlessSvcPorts(g, compType))).Should(BeTrue())
			}
		}
		g.Expect(len(expectServices)).Should(Equal(len(svcList.Items)))
	}

	testServiceAddAndDelete := func() {
		By("Creating a cluster with two LoadBalancer services")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(replicationCompName, replicationCompDefName).SetReplicas(1).
			AddService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		expectServices := map[string]ExpectService{
			testapps.ServiceHeadlessName: {svcType: corev1.ServiceTypeClusterIP, headless: true},
			testapps.ServiceDefaultName:  {svcType: corev1.ServiceTypeClusterIP, headless: false},
			testapps.ServiceVPCName:      {svcType: corev1.ServiceTypeLoadBalancer, headless: false},
			testapps.ServiceInternetName: {svcType: corev1.ServiceTypeLoadBalancer, headless: false},
		}
		Eventually(func(g Gomega) { validateCompSvcList(g, replicationCompName, replicationCompDefName, expectServices) }).Should(Succeed())

		By("Delete a LoadBalancer service")
		deleteService := testapps.ServiceVPCName
		delete(expectServices, deleteService)
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			for idx, comp := range cluster.Spec.ComponentSpecs {
				if comp.ComponentDefRef != replicationCompDefName || comp.Name != replicationCompName {
					continue
				}
				var services []appsv1alpha1.ClusterComponentService
				for _, item := range comp.Services {
					if item.Name == deleteService {
						continue
					}
					services = append(services, item)
				}
				cluster.Spec.ComponentSpecs[idx].Services = services
				return
			}
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateCompSvcList(g, replicationCompName, replicationCompDefName, expectServices) }).Should(Succeed())

		By("Add the deleted LoadBalancer service back")
		expectServices[deleteService] = ExpectService{svcType: corev1.ServiceTypeLoadBalancer, headless: false}
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			for idx, comp := range cluster.Spec.ComponentSpecs {
				if comp.ComponentDefRef != replicationCompDefName || comp.Name != replicationCompName {
					continue
				}
				comp.Services = append(comp.Services, appsv1alpha1.ClusterComponentService{
					Name:        deleteService,
					ServiceType: corev1.ServiceTypeLoadBalancer,
				})
				cluster.Spec.ComponentSpecs[idx] = comp
				return
			}
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateCompSvcList(g, replicationCompName, replicationCompDefName, expectServices) }).Should(Succeed())
	}

	checkAllServicesCreate := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(replicationCompName, replicationCompDefName).SetReplicas(1).
			AddComponent(statelessCompName, statelessCompDefName).SetReplicas(3).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName, statelessCompName)

		By("Checking proxy services")
		nginxExpectServices := map[string]ExpectService{
			// TODO: fix me later, proxy should not have internal headless service
			testapps.ServiceHeadlessName: {svcType: corev1.ServiceTypeClusterIP, headless: true},
			testapps.ServiceDefaultName:  {svcType: corev1.ServiceTypeClusterIP, headless: false},
		}
		Eventually(func(g Gomega) { validateCompSvcList(g, statelessCompName, statelessCompDefName, nginxExpectServices) }).Should(Succeed())

		By("Checking mysql services")
		mysqlExpectServices := map[string]ExpectService{
			testapps.ServiceHeadlessName: {svcType: corev1.ServiceTypeClusterIP, headless: true},
			testapps.ServiceDefaultName:  {svcType: corev1.ServiceTypeClusterIP, headless: false},
		}
		Eventually(func(g Gomega) {
			validateCompSvcList(g, replicationCompName, replicationCompDefName, mysqlExpectServices)
		}).Should(Succeed())

		By("Make sure the cluster controller has set the cluster status to Running")
		for i, comp := range clusterObj.Spec.ComponentSpecs {
			if comp.ComponentDefRef != replicationCompDefName || comp.Name != replicationCompName {
				continue
			}
			stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj), clusterObj.Spec.ComponentSpecs[i].Name)
			for _, v := range stsList.Items {
				Expect(testapps.ChangeObjStatus(&testCtx, &v, func() {
					testk8s.MockStatefulSetReady(&v)
				})).ShouldNot(HaveOccurred())
			}
		}
	}

	testWipeOut := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		// REVIEW: this test flow

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	testDoNotTermintate := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		// REVIEW: this test flow

		// REVIEW: why not set termination upon creation?
		By("Update the cluster's termination policy to DoNotTerminate")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.DoNotTerminate
		})()).ShouldNot(HaveOccurred())

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, true)).Should(Succeed())

		By("Update the cluster's termination policy to WipeOut")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.WipeOut
		})()).ShouldNot(HaveOccurred())

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	changeCompReplicas := func(clusterName types.NamespacedName, replicas int32, comp *appsv1alpha1.ClusterComponentSpec) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			for i, clusterComp := range cluster.Spec.ComponentSpecs {
				if clusterComp.Name == comp.Name {
					cluster.Spec.ComponentSpecs[i].Replicas = replicas
				}
			}
		})()).ShouldNot(HaveOccurred())
	}

	changeStatefulSetReplicas := func(clusterName types.NamespacedName, replicas int32) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			if len(cluster.Spec.ComponentSpecs) == 0 {
				cluster.Spec.ComponentSpecs = []appsv1alpha1.ClusterComponentSpec{
					{
						Name:            replicationCompName,
						ComponentDefRef: replicationCompDefName,
						Replicas:        replicas,
					}}
			} else {
				cluster.Spec.ComponentSpecs[0].Replicas = replicas
			}
		})()).ShouldNot(HaveOccurred())
	}

	testChangeReplicas := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		replicasSeq := []int32{5, 3, 1, 0, 2, 4}
		expectedOG := int64(1)
		for _, replicas := range replicasSeq {
			By(fmt.Sprintf("Change replicas to %d", replicas))
			changeStatefulSetReplicas(clusterKey, replicas)
			expectedOG++

			By("Checking cluster status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				g.Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))
			})).Should(Succeed())
			Eventually(func(g Gomega) {
				stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
				g.Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(replicas))
			}).Should(Succeed())
		}
	}

	getPVCName := func(compName string, i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, compName, i)
	}

	createPVC := func(clusterName, pvcName, compName string) {
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, "data").SetStorage("1Gi").CheckedCreate(&testCtx)
	}

	mockPodsForConsensusTest := func(cluster *appsv1alpha1.Cluster, number int) []corev1.Pod {
		componentName := cluster.Spec.ComponentSpecs[0].Name
		clusterName := cluster.Name
		stsName := cluster.Name + "-" + componentName
		pods := make([]corev1.Pod, 0)
		for i := 0; i < number; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName + "-" + strconv.Itoa(i),
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey:          clusterName,
						constant.KBAppComponentLabelKey:       componentName,
						appsv1.ControllerRevisionHashLabelKey: "mock-version",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "mock-container",
						Image: "mock-container",
					}},
				},
			}
			pods = append(pods, *pod)
		}
		return pods
	}

	horizontalScaleComp := func(updatedReplicas int, comp *appsv1alpha1.ClusterComponentSpec, policy *appsv1alpha1.HorizontalScalePolicy) {
		By("Mocking components' PVCs to bound")
		for i := 0; i < int(comp.Replicas); i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			createPVC(clusterKey.Name, pvcKey.Name, comp.Name)
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})()).ShouldNot(HaveOccurred())
		}

		By("Checking sts replicas right")
		stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Replicas))

		By("Creating mock pods in StatefulSet")
		pods := mockPodsForConsensusTest(clusterObj, int(comp.Replicas))
		for _, pod := range pods {
			Expect(testCtx.CheckedCreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			// mock the status to pass the isReady(pod) check in consensus_set
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
		}

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, int32(updatedReplicas), comp)

		checkUpdatedStsReplicas := func() {
			By("Checking updated sts replicas")
			Eventually(func() int32 {
				stsList = testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
				return *stsList.Items[0].Spec.Replicas
			}).Should(BeEquivalentTo(updatedReplicas))
		}

		if policy == nil {
			checkUpdatedStsReplicas()
			return
		}

		By("Checking Backup created")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.BackupSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: comp.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(1))

		By("Mocking VolumeSnapshot and set it as ReadyToUse")
		snapshotKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
			clusterKey.Name, comp.Name),
			Namespace: testCtx.DefaultNamespace}
		pvcName := getPVCName(comp.Name, 0)
		volumeSnapshot := &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotKey.Name,
				Namespace: snapshotKey.Namespace,
				Labels: map[string]string{
					constant.KBManagedByKey:         "cluster",
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				}},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
			},
		}
		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		Expect(controllerruntime.SetControllerReference(clusterObj, volumeSnapshot, scheme)).Should(Succeed())
		Expect(testCtx.CreateObj(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
		readyToUse := true
		volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
		volumeSnapshot.Status = &volumeSnapshotStatus
		Expect(k8sClient.Status().Update(testCtx.Ctx, volumeSnapshot)).Should(Succeed())

		By("Mock PVCs status to bound")
		for i := 0; i < updatedReplicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})()).ShouldNot(HaveOccurred())
		}

		By("Check backup job cleanup")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.BackupSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: comp.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(0))
		Eventually(testapps.CheckObjExists(&testCtx, snapshotKey, &snapshotv1.VolumeSnapshot{}, false)).Should(Succeed())

		checkUpdatedStsReplicas()
	}

	// @argument componentDefsWithHScalePolicy assign ClusterDefinition.spec.componentDefs[].horizontalScalePolicy for
	// the matching names. If not provided, will set 1st ClusterDefinition.spec.componentDefs[0].horizontalScalePolicy.
	horizontalScale := func(updatedReplicas int, componentDefsWithHScalePolicy ...string) {
		viper.Set("VOLUMESNAPSHOT", true)
		cluster := &appsv1alpha1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		// REVIEW/TODO: (chantu)
		// ought to have HorizontalScalePolicy setup during ClusterDefinition object creation,
		// following implementation is rather hack-ish.

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				// assign 1st component
				if len(componentDefsWithHScalePolicy) == 0 && len(clusterDef.Spec.ComponentDefs) > 0 {
					componentDefsWithHScalePolicy = []string{
						clusterDef.Spec.ComponentDefs[0].Name,
					}
				}
				for i, compDef := range clusterDef.Spec.ComponentDefs {
					if !slices.Contains(componentDefsWithHScalePolicy, compDef.Name) {
						continue
					}

					By("Checking backup policy created from backup policy template")
					policyName := lifecycle.DeriveBackupPolicyName(clusterKey.Name, compDef.Name)
					// REVIEW/TODO: (chantu)
					//  caught following error, it appears that BackupPolicy is statically setup or only work with 1st
					//  componentDefs?
					//
					//       Unexpected error:
					//          <*errors.StatusError | 0x140023b5b80>: {
					//              ErrStatus: {
					//                  TypeMeta: {Kind: "", APIVersion: ""},
					//                  ListMeta: {
					//                      SelfLink: "",
					//                      ResourceVersion: "",
					//                      Continue: "",
					//                      RemainingItemCount: nil,
					//                  },
					//                  Status: "Failure",
					//                  Message: "backuppolicies.dataprotection.kubeblocks.io \"test-clusterstqcba-consensus-backup-policy\" not found",
					//                  Reason: "NotFound",
					//                  Details: {
					//                      Name: "test-clusterstqcba-consensus-backup-policy",
					//                      Group: "dataprotection.kubeblocks.io",
					//                      Kind: "backuppolicies",
					//                      UID: "",
					//                      Causes: nil,
					//                      RetryAfterSeconds: 0,
					//                  },
					//                  Code: 404,
					//              },
					//          }
					//          backuppolicies.dataprotection.kubeblocks.io "test-clusterstqcba-consensus-backup-policy" not found
					//      occurred
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy =
						&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
							BackupPolicyTemplateName: backupPolicyTPLName}

					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: policyName, Namespace: clusterKey.Namespace},
						&dataprotectionv1alpha1.BackupPolicy{}, true)).Should(Succeed())

				}
			})()).ShouldNot(HaveOccurred())
		//

		By("Mocking all components' PVCs to bound")
		for _, comp := range clusterObj.Spec.ComponentSpecs {
			for i := 0; i < int(comp.Replicas); i++ {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(comp.Name, i),
				}
				createPVC(clusterKey.Name, pvcKey.Name, comp.Name)
				Eventually(testapps.CheckObjExists(&testCtx, pvcKey,
					&corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
				// REVIEW/TODO: (chantu)
				//  why using Eventually for change object status?
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})).Should(Succeed())
			}
		}

		By("Get the latest cluster def")
		Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(clusterDefObj), clusterDefObj)).Should(Succeed())
		for i, comp := range clusterObj.Spec.ComponentSpecs {
			var policy *appsv1alpha1.HorizontalScalePolicy
			for _, componentDef := range clusterDefObj.Spec.ComponentDefs {
				if componentDef.Name == comp.ComponentDefRef {
					policy = componentDef.HorizontalScalePolicy
				}
			}
			horizontalScaleComp(updatedReplicas, &clusterObj.Spec.ComponentSpecs[i], policy)
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).
			Should(BeEquivalentTo(initialGeneration + len(clusterObj.Spec.ComponentSpecs)))
		for i := range clusterObj.Spec.ComponentSpecs {
			stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj),
				clusterObj.Spec.ComponentSpecs[i].Name)
			for _, v := range stsList.Items {
				Expect(testapps.ChangeObjStatus(&testCtx, &v, func() {
					testk8s.MockStatefulSetReady(&v)
				})).ShouldNot(HaveOccurred())
			}
		}
	}

	testHorizontalScale := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		By("Creating a single component cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(replicationCompName, replicationCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		// REVIEW: this test flow, wait for running phase?
		horizontalScale(int(updatedReplicas))
	}

	testMultiCompHScale := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		By("Creating a multi components cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(statefulCompName, statefulCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			AddComponent(consensusCompName, consensusCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			AddComponent(replicationCompName, replicationCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, statefulCompName, consensusCompName, replicationCompName)

		// statefulCompDefName not in componentDefsWithHScalePolicy, for nil backup policy test
		// REVIEW: (chantu)
		//  1. this test flow, wait for running phase?
		//  2. following horizontalScale only work with statefulCompDefName?
		horizontalScale(int(updatedReplicas), consensusCompDefName, replicationCompDefName)
	}

	testVerticalScale := func() {
		const storageClassName = "sc-mock"
		const replicas = 3

		By("Mock a StorageClass which allows resize")
		allowVolumeExpansion := true
		storageClass := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          "kubernetes.io/no-provisioner",
			AllowVolumeExpansion: &allowVolumeExpansion,
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, storageClass)).Should(Succeed())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(replicationCompName, replicationCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(replicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		By("Checking the replicas")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]
		Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(replicationCompName, i),
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: pvcSpec.ToV1PersistentVolumeClaimSpec(),
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pvc)).Should(Succeed())
			pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
			Expect(k8sClient.Status().Update(testCtx.Ctx, pvc)).Should(Succeed())
		}

		By("Updating the PVC storage size")
		newStorageValue := resource.MustParse("2Gi")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
		})()).ShouldNot(HaveOccurred())

		By("mock pods/sts of component are available")
		testapps.MockConsensusComponentPods(testCtx, sts, clusterObj.Name, replicationCompName)
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(testCtx, clusterKey.Name, replicationCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Checking PVCs are resized")
		stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts = &stsList.Items[0]
		for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(replicationCompName, int(i)),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
		}
	}

	testClusterAffinity := func() {
		const topologyKey = "testTopologyKey"
		const lableKey = "testNodeLabelKey"
		const labelValue = "testLabelValue"

		By("Creating a cluster with Affinity")
		affinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Required,
			TopologyKeys:    []string{topologyKey},
			NodeLabels: map[string]string{
				lableKey: labelValue,
			},
			Tenancy: appsv1alpha1.SharedNode,
		}

		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(replicationCompName, replicationCompDefName).SetReplicas(3).
			WithRandomName().SetClusterAffinity(affinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		By("Checking the Affinity and TopologySpreadConstraints")
		Eventually(func(g Gomega) {
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			g.Expect(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(lableKey))
			g.Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.DoNotSchedule))
			g.Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(topologyKey))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(HaveLen(1))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(topologyKey))
		}).Should(Succeed())

	}

	testComponentAffinity := func() {
		const clusterTopologyKey = "testClusterTopologyKey"
		const compTopologyKey = "testComponentTopologyKey"

		By("Creating a cluster with Affinity")
		affinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Required,
			TopologyKeys:    []string{clusterTopologyKey},
			Tenancy:         appsv1alpha1.SharedNode,
		}
		compAffinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Preferred,
			TopologyKeys:    []string{compTopologyKey},
			Tenancy:         appsv1alpha1.DedicatedNode,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().SetClusterAffinity(affinity).
			AddComponent(replicationCompName, replicationCompDefName).SetComponentAffinity(compAffinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		By("Checking the Affinity and the TopologySpreadConstraints")
		Eventually(func(g Gomega) {
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			g.Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.ScheduleAnyway))
			g.Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(compTopologyKey))
			g.Expect(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(HaveLen(1))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(corev1.LabelHostname))
		}).Should(Succeed())
	}

	testClusterToleration := func() {
		const tolerationKey = "testClusterTolerationKey"
		const tolerationValue = "testClusterTolerationValue"
		By("Creating a cluster with Toleration")
		toleration := corev1.Toleration{
			Key:      tolerationKey,
			Value:    tolerationValue,
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(replicationCompName, replicationCompDefName).SetReplicas(1).
			AddClusterToleration(toleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		By("Checking the tolerations")
		Eventually(func(g Gomega) {
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			g.Expect(podSpec.Tolerations).Should(HaveLen(2))
			toleration = podSpec.Tolerations[0]
			g.Expect(toleration.Key).Should(BeEquivalentTo(tolerationKey))
			g.Expect(toleration.Value).Should(BeEquivalentTo(tolerationValue))
			g.Expect(toleration.Operator).Should(BeEquivalentTo(corev1.TolerationOpEqual))
			g.Expect(toleration.Effect).Should(BeEquivalentTo(corev1.TaintEffectNoSchedule))
		}).Should(Succeed())
	}

	testComponentToleration := func() {
		clusterTolerationKey := "testClusterTolerationKey"
		compTolerationKey := "testcompTolerationKey"
		compTolerationValue := "testcompTolerationValue"

		By("Creating a cluster with Toleration")
		toleration := corev1.Toleration{
			Key:      clusterTolerationKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		}
		compToleration := corev1.Toleration{
			Key:      compTolerationKey,
			Value:    compTolerationValue,
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().AddClusterToleration(toleration).
			AddComponent(replicationCompName, replicationCompDefName).AddComponentToleration(compToleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		By("Checking the tolerations")
		Eventually(func(g Gomega) {
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.Tolerations).Should(HaveLen(2))
			toleration = podSpec.Tolerations[0]
			g.Expect(toleration.Key).Should(BeEquivalentTo(compTolerationKey))
			g.Expect(toleration.Value).Should(BeEquivalentTo(compTolerationValue))
			g.Expect(toleration.Operator).Should(BeEquivalentTo(corev1.TolerationOpEqual))
			g.Expect(toleration.Effect).Should(BeEquivalentTo(corev1.TaintEffectNoSchedule))
		}).Should(Succeed())
	}

	mockRoleChangedEvent := func(key types.NamespacedName, sts *appsv1.StatefulSet) []corev1.Event {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		events := make([]corev1.Event, 0)
		for _, pod := range pods {
			event := corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name + "-event",
					Namespace: testCtx.DefaultNamespace,
				},
				Reason:  "Unhealthy",
				Message: `Readiness probe failed: {"event":"Success","originalRole":"Leader","role":"Follower"}`,
				InvolvedObject: corev1.ObjectReference{
					Name:      pod.Name,
					Namespace: testCtx.DefaultNamespace,
					UID:       pod.UID,
					FieldPath: constant.ProbeCheckRolePath,
				},
			}
			events = append(events, event)
		}
		events[0].Message = `Readiness probe failed: {"event":"Success","originalRole":"Leader","role":"Leader"}`
		return events
	}

	getStsPodsName := func(sts *appsv1.StatefulSet) []string {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		names := make([]string, 0)
		for _, pod := range pods {
			names = append(names, pod.Name)
		}
		return names
	}

	testThreeReplicas := func() {
		const replicas = 3

		By("Mock a cluster obj")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(replicationCompName, replicationCompDefName).
			SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		var stsList *appsv1.StatefulSetList
		var sts *appsv1.StatefulSet
		Eventually(func(g Gomega) {
			stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			g.Expect(stsList.Items).ShouldNot(BeEmpty())
			sts = &stsList.Items[0]
		}).Should(Succeed())

		By("Creating mock pods in StatefulSet")
		pods := mockPodsForConsensusTest(clusterObj, replicas)
		for _, pod := range pods {
			Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			// mock the status to pass the isReady(pod) check in consensus_set
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
		}

		By("Creating mock role changed events")
		// pod.Labels[intctrlutil.RoleLabelKey] will be filled with the role
		events := mockRoleChangedEvent(clusterKey, sts)
		for _, event := range events {
			Expect(testCtx.CreateObj(ctx, &event)).Should(Succeed())
		}

		By("Checking pods' role are changed accordingly")
		Eventually(func(g Gomega) {
			pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
			g.Expect(err).ShouldNot(HaveOccurred())
			// should have 3 pods
			g.Expect(pods).Should(HaveLen(3))
			// 1 leader
			// 2 followers
			leaderCount, followerCount := 0, 0
			for _, pod := range pods {
				switch pod.Labels[constant.RoleLabelKey] {
				case leader:
					leaderCount++
				case follower:
					followerCount++
				}
			}
			g.Expect(leaderCount).Should(Equal(1))
			g.Expect(followerCount).Should(Equal(2))
		}).Should(Succeed())

		By("Updating StatefulSet's status")
		sts.Status.UpdateRevision = "mock-version"
		sts.Status.Replicas = int32(replicas)
		sts.Status.AvailableReplicas = int32(replicas)
		sts.Status.CurrentReplicas = int32(replicas)
		sts.Status.ReadyReplicas = int32(replicas)
		sts.Status.ObservedGeneration = sts.Generation
		Expect(k8sClient.Status().Update(ctx, sts)).Should(Succeed())

		By("Checking consensus set pods' role are updated in cluster status")
		Eventually(func(g Gomega) {
			fetched := &appsv1alpha1.Cluster{}
			g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
			compName := fetched.Spec.ComponentSpecs[0].Name
			g.Expect(fetched.Status.Components != nil).To(BeTrue())
			g.Expect(fetched.Status.Components).To(HaveKey(compName))
			compStatus, ok := fetched.Status.Components[compName]
			g.Expect(ok).Should(BeTrue())
			consensusStatus := compStatus.ConsensusSetStatus
			g.Expect(consensusStatus != nil).To(BeTrue())
			g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
			g.Expect(consensusStatus.Followers).Should(HaveLen(2))
			g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
			g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
		}).Should(Succeed())

		By("Waiting the component be running")
		Eventually(testapps.GetClusterComponentPhase(testCtx, clusterObj.Name, replicationCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
	}

	testBackupError := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(replicationCompName, replicationCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, replicationCompName)

		// REVIEW: this test flow, should wait/fake still Running phase?
		By("Creating backup")
		backupKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      "test-backup",
		}
		backup := dataprotectionv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupKey.Name,
				Namespace: backupKey.Namespace,
				Labels: map[string]string{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: replicationCompName,
					constant.KBManagedByKey:         "cluster",
				},
			},
			Spec: dataprotectionv1alpha1.BackupSpec{
				BackupPolicyName: lifecycle.DeriveBackupPolicyName(clusterKey.Name, replicationCompDefName),
				BackupType:       "snapshot",
			},
		}
		Expect(testCtx.Create(ctx, &backup)).Should(Succeed())

		By("Checking backup status to failed, because VolumeSnapshot disabled")
		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, backup *dataprotectionv1alpha1.Backup) {
			g.Expect(backup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupFailed))
		})).Should(Succeed())

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
					&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
						BackupPolicyTemplateName: backupPolicyTPLName}
			})()).ShouldNot(HaveOccurred())

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, updatedReplicas, &clusterObj.Spec.ComponentSpecs[0])

		By("Checking cluster status failed with backup error")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			hasBackupError := false
			for _, cond := range cluster.Status.Conditions {
				if strings.Contains(cond.Message, "backup error") {
					hasBackupError = true
					break
				}
			}
			g.Expect(hasBackupError).Should(BeTrue())

		})).Should(Succeed())
	}

	updateClusterAnnotation := func(cluster *appsv1alpha1.Cluster) {
		Expect(testapps.ChangeObj(&testCtx, cluster, func(lcluster *appsv1alpha1.Cluster) {
			lcluster.Annotations = map[string]string{
				"time": time.Now().Format(time.RFC3339),
			}
		})).ShouldNot(HaveOccurred())
	}

	// Scenarios

	Context("when creating cluster without clusterversion", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, replicationCompDefName).
				Create(&testCtx).GetObject()
		})

		It("should reconcile to create cluster with no error", func() {
			By("Creating a cluster")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, "").
				AddComponent(replicationCompName, replicationCompDefName).SetReplicas(3).
				WithRandomName().Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, replicationCompName)
		})
	})

	Context("when creating cluster with multiple kinds of components", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, consensusCompDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, replicationCompDefName).
				AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(replicationCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(statelessCompDefName).AddContainerShort("nginx", testapps.NginxImage).
				Create(&testCtx).GetObject()

			By("Creating a BackupPolicyTemplate")
			createBackupPolicyTpl(clusterDefObj)
		})

		It("should create all sub-resources successfully", func() {
			checkAllResourcesCreated()
		})

		It("should create corresponding services correctly", func() {
			checkAllServicesCreate()
		})

		It("should add and delete service correctly", func() {
			testServiceAddAndDelete()
		})

		It("should successfully h-scale with multiple components", func() {
			testMultiCompHScale()
		})
	})

	When("when creating cluster with workloadType=stateful component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, replicationCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(replicationCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			By("Creating a BackupPolicyTemplate")
			createBackupPolicyTpl(clusterDefObj)
		})

		It("should delete cluster resources immediately if deleting cluster with WipeOut termination policy", func() {
			testWipeOut()
		})

		It("should not terminate immediately if deleting cluster with DoNotTerminate termination policy", func() {
			testDoNotTermintate()
		})

		It("should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", func() {
			testChangeReplicas()
		})

		Context("and with cluster affinity set", func() {
			It("should create pod with cluster affinity", func() {
				testClusterAffinity()
			})
		})

		Context("and with both cluster affinity and component affinity set", func() {
			It("Should observe the component affinity will override the cluster affinity", func() {
				testComponentAffinity()
			})
		})

		Context("and with cluster tolerations set", func() {
			It("Should create pods with cluster tolerations", func() {
				testClusterToleration()
			})
		})

		Context("and with both cluster tolerations and component tolerations set", func() {
			It("Should observe the component tolerations will override the cluster tolerations", func() {
				testComponentToleration()
			})
		})

		Context("with pvc", func() {
			It("should trigger a backup process(snapshot) and create pvcs from backup for newly created replicas when horizontal scale the cluster from 1 to 3", func() {
				testHorizontalScale()
			})
		})

		Context("with pvc and dynamic-provisioning storage class", func() {
			It("should update PVC request storage size accordingly when vertical scale the cluster", func() {
				testVerticalScale()
			})
		})
	})

	When("when creating cluster with workloadType=consensus component", func() {
		BeforeEach(func() {
			By("Create a clusterDef obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, replicationCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(replicationCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			By("Creating a BackupPolicyTemplate")
			createBackupPolicyTpl(clusterDefObj)
		})

		It("Should success with one leader pod and two follower pods", func() {
			testThreeReplicas()
		})

		It("should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", func() {
			testChangeReplicas()
		})

		Context("with pvc", func() {
			It("should trigger a backup process(snapshot) and create pvcs from backup for newly created replicas when horizontal scale the cluster from 1 to 3", func() {
				testHorizontalScale()
			})
		})

		Context("with pvc and dynamic-provisioning storage class", func() {
			It("should update PVC request storage size accordingly when vertical scale the cluster", func() {
				testVerticalScale()
			})
		})

		Context("with horizontalScale after verticalScale", func() {
			It("should succeed", func() {
				testVerticalScale()
				horizontalScale(5)
			})
		})

		It("should report error if backup error during h-scale", func() {
			testBackupError()
		})

		It("test restore cluster from backup", func() {
			By("mock backup")
			backupPolicyName := "test-backup-policy"
			backupName := "test-backup"
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName())
			By("creating backup")
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(backupPolicyName).
				SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
				Create(&testCtx).GetObject()

			By("waiting for backup failed, because no backup policy exists")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup),
				func(g Gomega, tmpBackup *dataprotectionv1alpha1.Backup) {
					g.Expect(tmpBackup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			By("mocking backup status completed, we don't need backup reconcile here")
			Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
				backup.Status.BackupToolName = backupTool.Name
				backup.Status.PersistentVolumeClaimName = "backup-pvc"
				backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
			})).ShouldNot(HaveOccurred())
			By("checking backup status completed")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup),
				func(g Gomega, tmpBackup *dataprotectionv1alpha1.Backup) {
					g.Expect(tmpBackup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			By("creating cluster with backup")
			restoreFromBackup := fmt.Sprintf(`{"%s":"%s"}`, replicationCompName, backupName)
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(replicationCompName, replicationCompDefName).
				SetReplicas(3).
				AddAnnotations(constant.RestoreFromBackUpAnnotationKey, restoreFromBackup).Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, replicationCompName)

			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			sts := stsList.Items[0]
			Expect(sts.Spec.Template.Spec.InitContainers).Should(HaveLen(1))

			By("mock pod/sts are available and wait for component enter running phase")
			testapps.MockConsensusComponentPods(testCtx, &sts, clusterObj.Name, replicationCompName)
			Expect(testapps.ChangeObjStatus(&testCtx, &sts, func() {
				testk8s.MockStatefulSetReady(&sts)
			})).ShouldNot(HaveOccurred())
			Eventually(testapps.GetClusterComponentPhase(testCtx, clusterObj.Name, replicationCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

			By("remove init container after all components are Running")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, client.ObjectKeyFromObject(clusterObj))).Should(BeEquivalentTo(1))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObj), clusterObj)).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					replicationCompName: {Phase: appsv1alpha1.RunningClusterCompPhase},
				}
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(&sts), func(g Gomega, tmpSts *appsv1.StatefulSet) {
				g.Expect(tmpSts.Spec.Template.Spec.InitContainers).Should(BeEmpty())
			})).Should(Succeed())

			By("clean up annotations after cluster running")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Phase = appsv1alpha1.RunningClusterPhase
			})).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Annotations[constant.RestoreFromBackUpAnnotationKey]).Should(BeEmpty())
			})).Should(Succeed())
		})
	})

	When("when creating cluster with workloadType=replication component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication componentDefRef.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication componentDefRef.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompDefName).
				AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()
		})

		// REVIEW/TODO: following test always failed at cluster.phase.observerGeneration=1
		//     with cluster.phase.phase=creating
		It("Should success with primary pod and secondary pod", func() {
			By("Mock a cluster obj with replication componentDefRef.")
			pvcSpec := testapps.NewPVCSpec("1Gi")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).
				SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
				SetReplicas(testapps.DefaultReplicationReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, testapps.DefaultRedisCompName)

			By("Checking statefulSet number")
			stsList := testk8s.ListAndCheckStatefulSetCount(&testCtx, clusterKey, 1)
			sts := &stsList.Items[0]

			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			for i := int32(0); i < *sts.Spec.Replicas; i++ {
				podName := fmt.Sprintf("%s-%d", sts.Name, i)
				testapps.MockReplicationComponentStsPod(nil, testCtx, sts, clusterObj.Name,
					testapps.DefaultRedisCompName, podName, replication.DefaultRole(i))
			}
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
		})
	})

	Context("test cluster Failed/Abnormal phase", func() {
		It("test cluster conditions", func() {
			By("init cluster")
			cluster := testapps.CreateConsensusMysqlCluster(testCtx, clusterDefNameRand,
				clusterVersionNameRand, clusterNameRand, consensusCompDefName, consensusCompName)
			clusterKey := client.ObjectKeyFromObject(cluster)

			By("mock pvc created")
			for i := 0; i < 3; i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, consensusCompName, i)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterKey.Name,
					consensusCompName, "data").SetStorage("2Gi").Create(&testCtx).GetObject()
				// mock pvc bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pvc), func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).ShouldNot(HaveOccurred())
			}

			By("test when clusterDefinition not found")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(BeEquivalentTo(lifecycle.ReasonPreCheckFailed))
			})).Should(Succeed())

			// TODO: removed conditionsError phase need to review correct-ness of following commented off block:
			// By("test conditionsError phase")
			// Expect(testapps.GetAndChangeObjStatus(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
			// 	condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, ConditionTypeProvisioningStarted)
			// 	condition.LastTransitionTime = metav1.Time{Time: time.Now().Add(-(time.Millisecond*time.Duration(viper.GetInt(constant.CfgKeyCtrlrReconcileRetryDurationMS)) + time.Second))}
			// 	meta.SetStatusCondition(&tmpCluster.Status.Conditions, *condition)
			// })()).ShouldNot(HaveOccurred())

			// Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
			// 	g.Expect(tmpCluster.Status.Phase == appsv1alpha1.ConditionsErrorPhase).Should(BeTrue())
			// })).Should(Succeed())

			By("test when clusterVersion not Available")
			_ = testapps.CreateConsensusMysqlClusterDef(testCtx, clusterDefNameRand, consensusCompDefName)
			clusterVersion := testapps.CreateConsensusMysqlClusterVersion(testCtx, clusterDefNameRand, clusterVersionNameRand, consensusCompDefName)
			clusterVersionKey := client.ObjectKeyFromObject(clusterVersion)
			// mock clusterVersion unavailable
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "test-n"
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase == appsv1alpha1.UnavailablePhase).Should(BeTrue())
			})).Should(Succeed())

			// trigger reconcile
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error1"}
			})()).ShouldNot(HaveOccurred())

			Eventually(func(g Gomega) {
				updateClusterAnnotation(cluster)
				g.Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
					condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
					g.Expect(condition).ShouldNot(BeNil())
					g.Expect(condition.Reason).Should(BeEquivalentTo(lifecycle.ReasonPreCheckFailed))
				})).Should(Succeed())
			}).Should(Succeed())

			By("reset clusterVersion to Available")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "consensus"
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase == appsv1alpha1.AvailablePhase).Should(BeTrue())
			})).Should(Succeed())

			// trigger reconcile
			updateClusterAnnotation(cluster)
			By("test preCheckFailed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition != nil && condition.Reason == lifecycle.ReasonPreCheckFailed).Should(BeTrue())
			})).Should(Succeed())

			By("reset and waiting cluster to Creating")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error"}
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.CreatingClusterPhase))
				g.Expect(tmpCluster.Status.ObservedGeneration).ShouldNot(BeZero())
			})).Should(Succeed())

			By("test apply resources failed")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, appsv1alpha1.ConditionTypeApplyResources)
					g.Expect(condition != nil && condition.Reason == lifecycle.ReasonApplyResourcesFailed).Should(BeTrue())
				})).Should(Succeed())
		})
	})
})

func createBackupPolicyTpl(clusterDefObj *appsv1alpha1.ClusterDefinition) {
	By("Creating a BackupPolicyTemplate")
	bpt := testapps.NewBackupPolicyTemplateFactory(backupPolicyTPLName).
		AddLabels(clusterDefLabelKey, clusterDefObj.Name).
		SetClusterDefRef(clusterDefObj.Name)
	for _, v := range clusterDefObj.Spec.ComponentDefs {
		bpt = bpt.AddBackupPolicy(v.Name).AddSnapshotPolicy()
		switch v.WorkloadType {
		case appsv1alpha1.Consensus:
			bpt.SetTargetRole("leader")
		case appsv1alpha1.Replication:
			bpt.SetTargetRole("primary")
		}
	}
	bpt.Create(&testCtx)
}
