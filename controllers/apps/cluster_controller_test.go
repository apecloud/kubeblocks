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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Cluster Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const mysqlCompType = "replicasets"
	const mysqlCompName = "mysql"

	const nginxCompType = "proxy"
	const nginxCompName = "nginx"

	const leader = "leader"
	const follower = "follower"

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
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
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

	checkAllResourcesCreated := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(3).
			AddComponent(nginxCompName, nginxCompType).SetReplicas(3).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(BeEquivalentTo(appsv1alpha1.StartingClusterPhase))

		By("Check deployment workload has been created")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.DeploymentSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey: clusterKey.Name,
			}, client.InNamespace(clusterKey.Namespace))).ShouldNot(Equal(0))

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
	}

	testServiceAddAndDelete := func() {
		By("Creating a cluster with two LoadBalancer services")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			AddService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(BeEquivalentTo(appsv1alpha1.StartingClusterPhase))

		validateSvc := func(g Gomega, total int, svcName string, predicate func(Gomega, int) bool) bool {
			svcList := &corev1.ServiceList{}
			g.Expect(k8sClient.List(testCtx.Ctx, svcList, client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: mysqlCompName,
			}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
			g.Expect(svcList.Items).Should(HaveLen(total))
			idx := slices.IndexFunc(svcList.Items, func(e corev1.Service) bool {
				return strings.HasSuffix(e.Name, svcName)
			})
			if predicate != nil {
				return predicate(g, idx)
			}
			g.Expect(idx >= 0).Should(BeTrue())
			svc := svcList.Items[idx]
			return svc.Spec.Type != corev1.ServiceTypeLoadBalancer ||
				svc.Spec.ExternalTrafficPolicy == corev1.ServiceExternalTrafficPolicyTypeLocal
		}
		Consistently(func(g Gomega) bool {
			return validateSvc(g, 4, testapps.ServiceVPCName, nil)
		}).Should(BeTrue())
		Consistently(func(g Gomega) bool {
			return validateSvc(g, 4, testapps.ServiceInternetName, nil)
		}).Should(BeTrue())

		By("Delete a LoadBalancer service")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			for idx, comp := range cluster.Spec.ComponentSpecs {
				if comp.ComponentDefRef != mysqlCompType || comp.Name != mysqlCompName {
					continue
				}
				var services []appsv1alpha1.ClusterComponentService
				for _, item := range comp.Services {
					if item.Name == testapps.ServiceVPCName {
						continue
					}
					services = append(services, item)
				}
				cluster.Spec.ComponentSpecs[idx].Services = services
				return
			}
		})()).ShouldNot(HaveOccurred())

		// REVIEW: not so BDD as need to implement condition logics.
		Eventually(func(g Gomega) bool {
			return validateSvc(g, 3, testapps.ServiceVPCName,
				func(g Gomega, i int) bool {
					return i >= 0
				})
		}).Should(BeFalse())

		By("Add the deleted LoadBalancer service back")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			for idx, comp := range cluster.Spec.ComponentSpecs {
				if comp.ComponentDefRef != mysqlCompType || comp.Name != mysqlCompName {
					continue
				}
				comp.Services = append(comp.Services, appsv1alpha1.ClusterComponentService{
					Name:        testapps.ServiceVPCName,
					ServiceType: corev1.ServiceTypeLoadBalancer,
				})
				cluster.Spec.ComponentSpecs[idx] = comp
				return
			}
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) bool {
			return validateSvc(g, 4, testapps.ServiceVPCName, nil)
		}).Should(BeTrue())
	}

	checkAllServicesCreate := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			AddComponent(nginxCompName, nginxCompType).SetReplicas(3).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

		By("Checking proxy should have external ClusterIP service")
		svcList1 := &corev1.ServiceList{}
		Expect(k8sClient.List(testCtx.Ctx, svcList1, client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: nginxCompName,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
		// TODO: fix me later, proxy should not have internal headless service
		// Expect(svcList1.Items).Should(HaveLen(1))
		Expect(svcList1.Items).ShouldNot(BeEmpty())
		var existsExternalClusterIP bool
		for _, svc := range svcList1.Items {
			Expect(svc.Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
			if svc.Spec.ClusterIP == corev1.ClusterIPNone {
				continue
			}
			existsExternalClusterIP = true
		}
		Expect(existsExternalClusterIP).To(BeTrue())

		By("Checking mysql should have internal headless service")
		getHeadlessSvcPorts := func(compDefName string) []corev1.ServicePort {
			cluster := &appsv1alpha1.Cluster{}
			Expect(k8sClient.Get(testCtx.Ctx, clusterKey, cluster)).To(Succeed())
			comp, err := util.GetComponentDefByCluster(testCtx.Ctx, k8sClient, *cluster, compDefName)
			Expect(err).ShouldNot(HaveOccurred())
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

		svcList2 := &corev1.ServiceList{}
		Expect(k8sClient.List(testCtx.Ctx, svcList2, client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: mysqlCompName,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
		Expect(svcList2.Items).Should(HaveLen(2))

		idx := slices.IndexFunc(svcList2.Items, func(e corev1.Service) bool {
			if e.Spec.Type != corev1.ServiceTypeClusterIP {
				return false
			}
			if e.Spec.ClusterIP != corev1.ClusterIPNone {
				return false
			}
			return true
		})
		Expect(idx).Should(BeNumerically(">=", 0))
		Expect(reflect.DeepEqual(svcList2.Items[idx].Spec.Ports,
			getHeadlessSvcPorts(mysqlCompType))).Should(BeTrue())
	}

	testWipeOut := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

		// REVIEW: this test flow

		// REVIEW: why not set termination upon creation?
		By("Update the cluster's termination policy to DoNotTerminate")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.DoNotTerminate
		})()).ShouldNot(HaveOccurred())

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Check the cluster do not terminate immediately")
		checkClusterDoNotTerminate := testapps.CheckObj(&testCtx, clusterKey,
			func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.Message).To(ContainSubstring(
					fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", fetched.Spec.TerminationPolicy)))
				g.Expect(fetched.Finalizers).ShouldNot(BeEmpty())
			})
		Eventually(checkClusterDoNotTerminate).Should(Succeed())
		Consistently(checkClusterDoNotTerminate).Should(Succeed())

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
						Name:            mysqlCompName,
						ComponentDefRef: mysqlCompType,
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
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

		replicasSeq := []int32{5, 3, 1, 0, 2, 4}
		expectedOG := int64(1)
		for _, replicas := range replicasSeq {
			By(fmt.Sprintf("Change replicas to %d", replicas))
			changeStatefulSetReplicas(clusterKey, replicas)
			expectedOG++

			By("Checking cluster status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				g.Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
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

	horizontalScaleComp := func(updatedReplicas int, comp *appsv1alpha1.ClusterComponentSpec) {
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

		By("Checking updated sts replicas")
		stsList = testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(*stsList.Items[0].Spec.Replicas).To(BeEquivalentTo(updatedReplicas))
	}

	horizontalScale := func(updatedReplicas int) {

		viper.Set("VOLUMESNAPSHOT", true)

		cluster := &appsv1alpha1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
					&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot}
			})()).ShouldNot(HaveOccurred())

		By("Creating a BackupPolicyTemplate")
		createBackupPolicyTpl(clusterDefObj)

		for i := range clusterObj.Spec.ComponentSpecs {
			horizontalScaleComp(updatedReplicas, &clusterObj.Spec.ComponentSpecs[i])
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(initialGeneration + len(clusterObj.Spec.ComponentSpecs)))
	}

	testHorizontalScale := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		By("Creating a single component cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

		// REVIEW: this test flow, wait for running phase?
		horizontalScale(int(updatedReplicas))
	}

	testMultiCompHScale := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		secondMysqlCompName := mysqlCompName + "1"

		By("Creating a multi components cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(initialReplicas).
			AddComponent(secondMysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

		// REVIEW: this test flow, wait for running phase?
		horizontalScale(int(updatedReplicas))
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
		pvcSpec := testapps.NewPVC("1Gi")
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(replicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

		By("Checking the replicas")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]
		Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(mysqlCompName, i),
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: pvcSpec,
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

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Checking PVCs are resized")
		stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts = &stsList.Items[0]
		for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(mysqlCompName, int(i)),
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
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(3).
			WithRandomName().SetClusterAffinity(affinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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
			AddComponent(mysqlCompName, mysqlCompType).SetComponentAffinity(compAffinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			AddClusterToleration(toleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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
			AddComponent(mysqlCompName, mysqlCompType).AddComponentToleration(compToleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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

		By("Waiting the cluster be running")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
	}

	mockPodsForReplicationTest := func(cluster *appsv1alpha1.Cluster, stsList []appsv1.StatefulSet) []corev1.Pod {
		componentName := cluster.Spec.ComponentSpecs[0].Name
		clusterName := cluster.Name
		pods := make([]corev1.Pod, 0)
		for _, sts := range stsList {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        sts.Name + "-0",
					Namespace:   testCtx.DefaultNamespace,
					Annotations: map[string]string{},
					Labels: map[string]string{
						constant.RoleLabelKey:                 sts.Labels[constant.RoleLabelKey],
						constant.AppInstanceLabelKey:          clusterName,
						constant.KBAppComponentLabelKey:       componentName,
						appsv1.ControllerRevisionHashLabelKey: sts.Status.UpdateRevision,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "mock-container",
						Image: "mock-container",
					}},
				},
			}
			for k, v := range sts.Spec.Template.Labels {
				pod.ObjectMeta.Labels[k] = v
			}
			for k, v := range sts.Spec.Template.Annotations {
				pod.ObjectMeta.Annotations[k] = v
			}
			pods = append(pods, *pod)
		}
		return pods
	}

	testBackupError := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

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
					constant.KBAppComponentLabelKey: mysqlCompName,
					constant.KBManagedByKey:         "cluster",
				},
			},
			Spec: dataprotectionv1alpha1.BackupSpec{
				BackupPolicyName: "test-backup-policy",
				BackupType:       "snapshot",
			},
		}
		Expect(testCtx.Create(ctx, &backup)).Should(Succeed())

		By("Checking backup status to failed, because VolumeSnapshot disabled")
		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, backup *dataprotectionv1alpha1.Backup) {
			g.Expect(backup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupFailed))
		})).Should(Succeed())

		By("Creating a BackupPolicyTemplate")
		createBackupPolicyTpl(clusterDefObj)

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
					&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot, BackupTemplateSelector: map[string]string{
						clusterDefLabelKey: clusterDefObj.Name,
					}}
			})()).ShouldNot(HaveOccurred())

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, updatedReplicas, &clusterObj.Spec.ComponentSpecs[0])

		By("Checking cluster status failed with backup error")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.AbnormalClusterPhase))
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

	// Scenarios

	Context("when creating cluster without clusterversion", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				Create(&testCtx).GetObject()
		})

		It("should reconcile to create cluster with no error", func() {
			By("Creating a cluster")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, "").
				AddComponent(mysqlCompName, mysqlCompType).SetReplicas(3).
				WithRandomName().Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster enter running phase")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))
		})
	})

	Context("when creating cluster with multiple kinds of components", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(nginxCompType).AddContainerShort("nginx", testapps.NginxImage).
				Create(&testCtx).GetObject()
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

	Context("when creating cluster with workloadType=stateful component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
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

	Context("when creating cluster with workloadType=consensus component", func() {
		BeforeEach(func() {
			By("Create a clusterDef obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ConsensusMySQLComponent, mysqlCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
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
				backup.Status.RemoteVolume = &corev1.Volume{
					Name: "backup-pvc",
				}
				backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
			})).ShouldNot(HaveOccurred())
			By("checking backup status completed")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup),
				func(g Gomega, tmpBackup *dataprotectionv1alpha1.Backup) {
					g.Expect(tmpBackup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			By("creating cluster with backup")
			restoreFromBackup := fmt.Sprintf(`{"%s":"%s"}`, mysqlCompName, backupName)
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(mysqlCompName, mysqlCompType).
				SetReplicas(3).
				AddAnnotations(constant.RestoreFromBackUpAnnotationKey, restoreFromBackup).Create(&testCtx).GetObject()
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, client.ObjectKeyFromObject(clusterObj))
			sts := stsList.Items[0]
			Expect(sts.Spec.Template.Spec.InitContainers).Should(HaveLen(1))

			By("remove init container after all components are Running")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					mysqlCompName: {Phase: appsv1alpha1.RunningClusterCompPhase},
				}
			})).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(&sts), func(g Gomega, tmpSts *appsv1.StatefulSet) {
				g.Expect(tmpSts.Spec.Template.Spec.InitContainers).Should(BeEmpty())
			})).Should(Succeed())

			By("clean up annotations after cluster running")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Phase = appsv1alpha1.RunningClusterPhase
			})).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Annotations[constant.RestoreFromBackUpAnnotationKey]).Should(BeEmpty())
			})).Should(Succeed())
		})
	})

	Context("when creating cluster with workloadType=replication component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication componentDefRef.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication componentDefRef.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompType).
				AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()
		})

		// REVIEW/TODO: following test always failed at cluster.phase.observerGeneration=1
		//     with cluster.phase.phase=creating
		It("Should success with primary sts and secondary sts", func() {
			By("Mock a cluster obj with replication componentDefRef.")
			pvcSpec := testapps.NewPVC("1Gi")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
				SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
				SetReplicas(testapps.DefaultReplicationReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster enter running phase")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

			By("Checking statefulSet number")
			var stsList *appsv1.StatefulSetList
			Eventually(func(g Gomega) {
				stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
				g.Expect(stsList.Items).Should(HaveLen(testapps.DefaultReplicationReplicas))
			}).Should(Succeed())

			By("Checking statefulSet role label")
			for _, sts := range stsList.Items {
				if strings.HasSuffix(sts.Name, fmt.Sprintf("%s-%s", clusterObj.Name, testapps.DefaultRedisCompName)) {
					Expect(sts.Labels[constant.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Primary))
				} else {
					Expect(sts.Labels[constant.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Secondary))
				}
			}

			By("Checking statefulSet template volumes mount")
			for _, sts := range stsList.Items {
				Expect(sts.Spec.VolumeClaimTemplates).Should(BeEmpty())
				for _, volume := range sts.Spec.Template.Spec.Volumes {
					if volume.Name == testapps.DataVolumeName {
						Expect(strings.HasPrefix(volume.VolumeSource.PersistentVolumeClaim.ClaimName, testapps.DataVolumeName+"-"+clusterKey.Name)).Should(BeTrue())
					}
				}
			}
		})

		It("Should successfully doing volume expansion", func() {
			storageClassName := "test-storage"
			pvcSpec := testapps.NewPVC("1Gi")
			pvcSpec.StorageClassName = &storageClassName
			updatedPVCSpec := testapps.NewPVC("2Gi")
			updatedPVCSpec.StorageClassName = &storageClassName

			By("Mock a cluster obj with replication componentDefRef.")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
				SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
				SetReplicas(testapps.DefaultReplicationReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster enter running phase")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

			// REVIEW: this test flow, should wait/fake still Running phase?

			By("Checking statefulset count")
			stsList := testk8s.ListAndCheckStatefulSetCount(&testCtx, clusterKey, testapps.DefaultReplicationReplicas)

			By("Creating mock pods in StatefulSet")
			pods := mockPodsForReplicationTest(clusterObj, stsList.Items)
			for _, pod := range pods {
				Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
				pod.Status.Conditions = []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}}
				Expect(testCtx.Cli.Status().Update(testCtx.Ctx, &pod)).Should(Succeed())
			}

			By("Checking pod count and ready")
			Eventually(func(g Gomega) {
				podList := testk8s.ListAndCheckPodCountWithComponent(&testCtx, clusterKey, testapps.DefaultRedisCompName, testapps.DefaultReplicationReplicas)
				for _, pod := range podList.Items {
					g.Expect(len(pod.Status.Conditions) > 0).Should(BeTrue())
					g.Expect(pod.Status.Conditions[0].Status).Should(Equal(corev1.ConditionTrue))
				}
			}).Should(Succeed())

			By("Mocking statefulset status to ready")
			for _, sts := range stsList.Items {
				sts.Status.ObservedGeneration = sts.Generation
				sts.Status.AvailableReplicas = 1
				sts.Status.Replicas = 1
				sts.Status.ReadyReplicas = 1
				err := testCtx.Cli.Status().Update(testCtx.Ctx, &sts)
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("Checking reconcile succeeded")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

			By("Creating storageclass")
			_ = testapps.CreateStorageClass(testCtx, storageClassName, true)

			pvcList := &corev1.PersistentVolumeClaimList{}

			By("Mocking PVCs status to bound")
			Expect(testCtx.Cli.List(testCtx.Ctx, pvcList, client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: testapps.DefaultRedisCompName,
			}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
			Expect(pvcList.Items).Should(HaveLen(testapps.DefaultReplicationReplicas))
			for _, pvc := range pvcList.Items {
				pvc.Status.Phase = corev1.ClaimBound
				Expect(testCtx.Cli.Status().Update(testCtx.Ctx, &pvc)).Should(Succeed())
			}

			By("Checking PVCs status bound")
			Eventually(func(g Gomega) {
				g.Expect(testCtx.Cli.List(testCtx.Ctx, pvcList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: testapps.DefaultRedisCompName,
				}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
				Expect(pvcList.Items).Should(HaveLen(testapps.DefaultReplicationReplicas))
				for _, pvc := range pvcList.Items {
					g.Expect(pvc.Status.Phase).Should(Equal(corev1.ClaimBound))
				}
			}).Should(Succeed())

			By("Updating PVC volume size")
			patch := client.MergeFrom(clusterObj.DeepCopy())
			componentSpec := clusterObj.GetComponentByName(testapps.DefaultRedisCompName)
			componentSpec.VolumeClaimTemplates[0].Spec = &updatedPVCSpec
			Expect(testCtx.Cli.Patch(ctx, clusterObj, patch)).Should(Succeed())

			By("Waiting cluster update reconcile succeed")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))

			By("Checking pvc volume size")
			Eventually(func(g Gomega) {
				g.Expect(testCtx.Cli.List(testCtx.Ctx, pvcList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: testapps.DefaultRedisCompName,
				}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
				g.Expect(len(pvcList.Items) == testapps.DefaultReplicationReplicas).To(BeTrue())
				for _, pvc := range pvcList.Items {
					g.Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).Should(BeEquivalentTo(updatedPVCSpec.Resources.Requests[corev1.ResourceStorage]))
				}
			}).Should(Succeed())
		})
	})
})

func createBackupPolicyTpl(clusterDefObj *appsv1alpha1.ClusterDefinition) {
	By("Creating a BackupPolicyTemplate")
	backupPolicyTplKey := types.NamespacedName{Name: "test-backup-policy-template-mysql"}
	backupPolicyTpl := &dataprotectionv1alpha1.BackupPolicyTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: backupPolicyTplKey.Name,
			Labels: map[string]string{
				clusterDefLabelKey: clusterDefObj.Name,
			},
		},
		Spec: dataprotectionv1alpha1.BackupPolicyTemplateSpec{
			BackupToolName: "mysql-xtrabackup",
		},
	}
	Expect(testCtx.CreateObj(testCtx.Ctx, backupPolicyTpl)).Should(Succeed())
}
