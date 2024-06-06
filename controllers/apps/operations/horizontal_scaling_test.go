/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("HorizontalScaling OpsRequest", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		insTplName            = "foo"
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterAnnotationAndPhaseForOps := func(opsRes *OpsResource) {
		Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())
	}

	Context("Test OpsRequest", func() {
		commonHScaleConsensusCompTest := func(reqCtx intctrlutil.RequestCtx,
			changeClusterSpec func(cluster *appsv1alpha1.Cluster),
			horizontalScaling appsv1alpha1.HorizontalScaling) (*OpsResource, []*corev1.Pod) {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			if changeClusterSpec != nil {
				Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1alpha1.Cluster) {
					changeClusterSpec(cluster)
				})).Should(Succeed())
			}
			pods := testapps.MockInstanceSetPods(&testCtx, nil, opsRes.Cluster, consensusComp)
			By("create opsRequest for horizontal scaling of consensus component")
			initClusterAnnotationAndPhaseForOps(opsRes)
			horizontalScaling.ComponentName = consensusComp
			opsRes.OpsRequest = createHorizontalScaling(clusterName, horizontalScaling)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase, consensusComp)

			By("expect for opsRequest phase is Creating after doing action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("check for the replicas of consensus component after doing action again when opsRequest phase is Creating")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				if horizontalScaling.Replicas == nil {
					return
				}
				lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[consensusComp]
				var expectedCompReplicas int32
				switch {
				case horizontalScaling.ScaleIn != nil:
					expectedCompReplicas = *lastCompConfiguration.Replicas - *horizontalScaling.ScaleIn.ReplicaChanges
				case horizontalScaling.ScaleOut != nil:
					expectedCompReplicas = *lastCompConfiguration.Replicas + *horizontalScaling.ScaleOut.ReplicaChanges
				default:
					expectedCompReplicas = *horizontalScaling.Replicas
				}
				g.Expect(tmpCluster.Spec.GetComponentByName(consensusComp).Replicas).Should(BeEquivalentTo(expectedCompReplicas))
			})).Should(Succeed())

			By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes, pods
		}

		checkOpsRequestPhaseIsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			// mock consensus component is Running
			mockConsensusCompToRunning(opsRes)
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		}

		checkCancelledSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsCancelledPhase))
			opsProgressDetails := opsRes.OpsRequest.Status.Components[consensusComp].ProgressDetails
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			Expect(len(opsProgressDetails)).Should(Equal(2))
		}

		deletePods := func(pods ...*corev1.Pod) {
			for i := range pods {
				testk8s.MockPodIsTerminating(ctx, testCtx, pods[i])
				testk8s.RemovePodFinalizer(ctx, testCtx, pods[i])
			}
		}

		createPods := func(templateName string, ordinals ...int) []*corev1.Pod {
			var pods []*corev1.Pod
			prefix := ""
			if templateName != "" {
				prefix = "-" + templateName
			}
			for i := range ordinals {
				podName := fmt.Sprintf("%s-%s%s-%d", clusterName, consensusComp, prefix, ordinals[i])
				pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "Readonly")
				pods = append(pods, pod)
			}
			return pods
		}

		testHScaleReplicas := func(
			changeClusterSpec func(cluster *appsv1alpha1.Cluster),
			horizontalScaling appsv1alpha1.HorizontalScaling,
			mockHScale func(podList []*corev1.Pod)) {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, changeClusterSpec, horizontalScaling)
			mockHScale(podList)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		}

		testCancelHScale := func(
			horizontalScaling appsv1alpha1.HorizontalScaling,
			isScaleDown bool) {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, nil, horizontalScaling)
			var pod *corev1.Pod
			if isScaleDown {
				By("delete the pod")
				pod = podList[2]
				deletePods(pod)
			} else {
				By("create the pod")
				pod = createPods("", 3)[0]
			}

			By("cancel HScale opsRequest after one pod has been deleted")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))
			if isScaleDown {
				By("re-create the pod for rollback")
				createPods("", 2)
			} else {
				By("delete the pod for rollback")
				deletePods(pod)
			}

			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)
			Expect(findStatusProgressDetail(opsRes.OpsRequest.Status.Components[consensusComp].ProgressDetails,
				getProgressObjectKey(constant.PodKind, pod.Name)).Status).Should(Equal(appsv1alpha1.SucceedProgressStatus))
		}

		It("test to scale in replicas with `replicas`", func() {
			By("scale in replicas from 3 to 1 ")
			horizontalScaling := appsv1alpha1.HorizontalScaling{}
			horizontalScaling.Replicas = pointer.Int32(1)
			testHScaleReplicas(nil, horizontalScaling, func(podList []*corev1.Pod) {
				By("delete the pods")
				deletePods(podList[1], podList[2])
			})
		})

		It("test to scale out replicas with `replicas`", func() {
			By("scale out replicas from 3 to 5")
			horizontalScaling := appsv1alpha1.HorizontalScaling{}
			horizontalScaling.Replicas = pointer.Int32(5)
			testHScaleReplicas(nil, horizontalScaling, func(podList []*corev1.Pod) {
				By("create the pods(ordinal:[3,4])")
				createPods("", 3, 4)
			})
		})

		It("test to scale out replicas with `scaleOut`", func() {
			By("scale out replicas from 3 to 5 with `scaleOut`")
			horizontalScaling := appsv1alpha1.HorizontalScaling{ScaleOut: &appsv1alpha1.ScaleOut{}}
			horizontalScaling.ScaleOut.ReplicaChanges = pointer.Int32(2)
			testHScaleReplicas(nil, horizontalScaling, func(podList []*corev1.Pod) {
				By("create the pods(ordinal:[3,4])")
				createPods("", 3, 4)
			})
		})

		It("test to scale in replicas with `scaleIn`", func() {
			By("scale in replicas from 3 to 1")
			horizontalScaling := appsv1alpha1.HorizontalScaling{ScaleIn: &appsv1alpha1.ScaleIn{}}
			horizontalScaling.ScaleIn.ReplicaChanges = pointer.Int32(2)
			testHScaleReplicas(nil, horizontalScaling, func(podList []*corev1.Pod) {
				By("delete the pods")
				deletePods(podList[1], podList[2])
			})
		})

		It("cancel the opsRequest which scaling in replicas with `replicas`", func() {
			By("scale in replicas of component from 3 to 1")
			testCancelHScale(appsv1alpha1.HorizontalScaling{Replicas: pointer.Int32(1)}, true)
		})

		It("cancel the opsRequest which scaling up replicas with `replicas`", func() {
			By("scale out replicas of component from 3 to 5")
			testCancelHScale(appsv1alpha1.HorizontalScaling{Replicas: pointer.Int32(5)}, false)
		})

		It("cancel the opsRequest which scaling up replicas with `scaleOut`", func() {
			By("scale out replicas of component from 3 to 5")
			testCancelHScale(appsv1alpha1.HorizontalScaling{ScaleOut: &appsv1alpha1.ScaleOut{ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(2)}}}, false)
		})

		It("cancel the opsRequest which scaling in replicas with `scaleIn`", func() {
			By("scale in replicas of component from 3 to 1")
			testCancelHScale(appsv1alpha1.HorizontalScaling{ScaleIn: &appsv1alpha1.ScaleIn{ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(2)}}}, true)
		})

		setClusterCompSpec := func(cluster *appsv1alpha1.Cluster, instances []appsv1alpha1.InstanceTemplate, offlineInstances []string) {
			for i, v := range cluster.Spec.ComponentSpecs {
				if v.Name == consensusComp {
					cluster.Spec.ComponentSpecs[i].OfflineInstances = offlineInstances
					cluster.Spec.ComponentSpecs[i].Instances = instances
					break
				}
			}
		}

		testHScaleWithSpecifiedPod := func(changeClusterSpec func(cluster *appsv1alpha1.Cluster),
			horizontalScaling appsv1alpha1.HorizontalScaling,
			expectOfflineInstances []string,
			mockHScale func(podList []*corev1.Pod)) *OpsResource {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, changeClusterSpec, horizontalScaling)
			By("verify cluster spec is correct")
			targetSpec := opsRes.Cluster.Spec.GetComponentByName(consensusComp)
			Expect(targetSpec.OfflineInstances).Should(HaveLen(len(expectOfflineInstances)))
			expectedOfflineInsSet := sets.New(expectOfflineInstances...)
			for _, v := range targetSpec.OfflineInstances {
				_, ok := expectedOfflineInsSet[v]
				Expect(ok).Should(BeTrue())
			}

			By("mock specified pods deleted")
			mockHScale(podList)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
			return opsRes
		}

		It("test offline the specified pod of the component", func() {
			toDeletePodName := fmt.Sprintf("%s-%s-1", clusterName, consensusComp)
			offlineInstances := []string{toDeletePodName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1alpha1.Cluster) {
				setClusterCompSpec(cluster, []appsv1alpha1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, nil)
			}, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{
					ReplicaChanger:           appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: offlineInstances,
				},
			}, offlineInstances, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod "%s"`, toDeletePodName))
				deletePods(podList[2])
			})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		})

		It("test offline the specified pod and scale out another replicas", func() {
			toDeletePodName := fmt.Sprintf("%s-%s-1", clusterName, consensusComp)
			offlineInstances := []string{toDeletePodName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1alpha1.Cluster) {
				setClusterCompSpec(cluster, []appsv1alpha1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, nil)
			}, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{
					ReplicaChanger:           appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: offlineInstances,
				},
				ScaleOut: &appsv1alpha1.ScaleOut{
					ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
				},
			}, offlineInstances, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod "%s"`, toDeletePodName))
				deletePods(podList[2])
				By("create a new pod(ordinal:2) by replicas")
				createPods("", 2)
			})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
		})

		It("test offline the specified pod and auto-sync replicaChanges", func() {
			offlineInstanceName := fmt.Sprintf("%s-%s-%s-0", clusterName, consensusComp, insTplName)
			offlineInstances := []string{offlineInstanceName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1alpha1.Cluster) {
				setClusterCompSpec(cluster, []appsv1alpha1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, nil)
			}, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: offlineInstances,
				},
			}, offlineInstances, func(podList []*corev1.Pod) {
				By("delete the specified pod " + offlineInstanceName)
				deletePods(podList[0])
			})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
			By("expect replicas to 2 and template " + insTplName + " replicas to 0")
			compSpec := opsRes.Cluster.Spec.GetComponentByName(consensusComp)
			Expect(compSpec.Replicas).Should(BeEquivalentTo(2))
			Expect(*compSpec.Instances[0].Replicas).Should(BeEquivalentTo(0))
		})

		It("test online the specified pod of the instance template and auto-sync replicaChanges", func() {
			offlineInstanceName := fmt.Sprintf("%s-%s-%s-0", clusterName, consensusComp, insTplName)
			offlineInstances := []string{offlineInstanceName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1alpha1.Cluster) {
				setClusterCompSpec(cluster, []appsv1alpha1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, offlineInstances)
			}, appsv1alpha1.HorizontalScaling{
				ScaleOut: &appsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: offlineInstances,
				},
			}, []string{}, func(podList []*corev1.Pod) {
				By("create the specified pod " + offlineInstanceName)
				testapps.MockInstanceSetPod(&testCtx, nil, clusterName, consensusComp, offlineInstanceName, "follower", "Readonly")
			})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
			By("expect replicas to 4")
			compSpec := opsRes.Cluster.Spec.GetComponentByName(consensusComp)
			Expect(compSpec.Replicas).Should(BeEquivalentTo(4))
			Expect(*compSpec.Instances[0].Replicas).Should(BeEquivalentTo(2))
		})

		It("test offline and online the specified pod and auto-sync replicaChanges", func() {
			onlinePodName := fmt.Sprintf("%s-%s-1", clusterName, consensusComp)
			offlinePodName := fmt.Sprintf("%s-%s-%s-0", clusterName, consensusComp, insTplName)
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1alpha1.Cluster) {
				setClusterCompSpec(cluster, []appsv1alpha1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, []string{onlinePodName})
			}, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{offlinePodName},
				},
				ScaleOut: &appsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: []string{onlinePodName},
				},
			}, []string{offlinePodName}, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod"%s"`, offlinePodName))
				deletePods(podList[0])

				By(fmt.Sprintf(`create the pod "%s" which is removed from offlineInstances`, onlinePodName))
				createPods("", 1)
			})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			By("expect replicas to 3")
			Expect(opsRes.Cluster.Spec.GetComponentByName(consensusComp).Replicas).Should(BeEquivalentTo(3))
		})

		It("h-scale new instance templates and scale in all old replicas", func() {
			templateFoo := appsv1alpha1.InstanceTemplate{
				Name:     insTplName,
				Replicas: func() *int32 { r := int32(3); return &r }(),
			}
			templateBar := appsv1alpha1.InstanceTemplate{
				Name:     "bar",
				Replicas: func() *int32 { r := int32(3); return &r }(),
			}
			instances := []appsv1alpha1.InstanceTemplate{templateFoo, templateBar}
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, pods := commonHScaleConsensusCompTest(reqCtx, nil, appsv1alpha1.HorizontalScaling{
				ScaleOut: &appsv1alpha1.ScaleOut{
					NewInstances: []appsv1alpha1.InstanceTemplate{templateFoo, templateBar},
				},
				ScaleIn: &appsv1alpha1.ScaleIn{
					ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(3)},
				},
			})
			By("verify cluster spec is correct")
			var targetSpec *appsv1alpha1.ClusterComponentSpec
			for i := range opsRes.Cluster.Spec.ComponentSpecs {
				spec := &opsRes.Cluster.Spec.ComponentSpecs[i]
				if spec.Name == consensusComp {
					targetSpec = spec
				}
			}

			// auto-sync replicaChanges of the component to 6
			Expect(targetSpec.Replicas).Should(BeEquivalentTo(6))
			Expect(targetSpec.Instances).Should(HaveLen(2))
			Expect(targetSpec.Instances).Should(Equal(instances))
			By("mock six pods are created")
			createPods(insTplName, 0, 1, 2)
			createPods("bar", 0, 1, 2)
			By("delete three pods")
			deletePods(pods...)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})
		createOpsAndToCreatingPhase := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, horizontalScaling appsv1alpha1.HorizontalScaling) *appsv1alpha1.OpsRequest {
			horizontalScaling.ComponentName = consensusComp
			opsRes.OpsRequest = createHorizontalScaling(clusterName, horizontalScaling)
			opsRes.OpsRequest.Spec.Force = true
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase, consensusComp)

			By("expect for opsRequest phase is Creating after doing action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("do Action")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes.OpsRequest
		}

		It("test offline the specified pod but it is not online", func() {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}

			By("offline the specified pod but it is not online")
			offlineInsName := fmt.Sprintf("%s-%s-4", clusterName, consensusComp)
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{
					ReplicaChanger:           appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: []string{offlineInsName},
				},
			})
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsFailedPhase))
			conditions := opsRes.OpsRequest.Status.Conditions
			Expect(conditions[len(conditions)-1].Message).Should(ContainSubstring(
				fmt.Sprintf(`instance "%s" specified in onlineInstancesToOffline is not online`, offlineInsName)))
		})

		It("test run multi horizontalScaling opsRequest with force flag", func() {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			By("create first opsRequest to add 1 replicas with `scaleOut` field and expect replicas to 4")
			ops1 := createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				ScaleOut: &appsv1alpha1.ScaleOut{ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			})
			Expect(opsRes.Cluster.Spec.GetComponentByName(consensusComp).Replicas).Should(BeEquivalentTo(4))

			By("create secondary opsRequest to add 1 replicas with `replicasToAdd` field and expect replicas to 5")
			ops2 := createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				ScaleOut: &appsv1alpha1.ScaleOut{ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			})
			Expect(opsRes.Cluster.Spec.GetComponentByName(consensusComp).Replicas).Should(BeEquivalentTo(5))

			By("create third opsRequest to offline a pod which is created by another running opsRequest and expect it to fail")
			offlineInsName := fmt.Sprintf("%s-%s-3", clusterName, consensusComp)
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{
					ReplicaChanger:           appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: []string{offlineInsName},
				},
			})
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsFailedPhase))
			conditions := opsRes.OpsRequest.Status.Conditions
			Expect(conditions[len(conditions)-1].Message).Should(ContainSubstring(fmt.Sprintf(`instance "%s" cannot be taken offline as it has been created by another running opsRequest`, offlineInsName)))

			By("create a opsRequest to delete 1 replicas which is created by another running opsRequest and expect it to fail")
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				ScaleIn: &appsv1alpha1.ScaleIn{ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			})
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsFailedPhase))
			conditions = opsRes.OpsRequest.Status.Conditions
			Expect(conditions[len(conditions)-1].Message).Should(ContainSubstring(`cannot be taken offline as it has been created by another running opsRequest`))

			By("create a opsRequest with `replicas` field and expect to abort the running ops")
			// if existing an overwrite replicas operation for a component or instanceTemplate, need to abort.
			ops3 := createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				Replicas: pointer.Int32(3),
			})
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(appsv1alpha1.OpsAbortedPhase))
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(appsv1alpha1.OpsAbortedPhase))

			By("create a opsRequest with `scaleOut` field and expect to abort last running ops")
			// if running opsRequest exists an overwrite replicas operation for a component or instanceTemplate, need to abort.
			createOpsAndToCreatingPhase(reqCtx, opsRes, appsv1alpha1.HorizontalScaling{
				ScaleOut: &appsv1alpha1.ScaleOut{ReplicaChanger: appsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			})
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops3))).Should(Equal(appsv1alpha1.OpsAbortedPhase))
			Expect(opsRes.Cluster.Spec.GetComponentByName(consensusComp).Replicas).Should(BeEquivalentTo(4))
		})
	})
})

func createHorizontalScaling(clusterName string, horizontalScaling appsv1alpha1.HorizontalScaling) *appsv1alpha1.OpsRequest {
	horizontalOpsName := "horizontal-scaling-ops-" + testCtx.GetRandomStr()
	ops := testapps.NewOpsRequestObj(horizontalOpsName, testCtx.DefaultNamespace,
		clusterName, appsv1alpha1.HorizontalScalingType)
	ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
		horizontalScaling,
	}
	opsRequest := testapps.CreateOpsRequest(ctx, testCtx, ops)
	opsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
	return opsRequest
}

func cancelOpsRequest(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, cancelTime time.Time) {
	opsRequest := opsRes.OpsRequest
	opsRequest.Spec.Cancel = true
	opsBehaviour := GetOpsManager().OpsMap[opsRequest.Spec.Type]
	Expect(testapps.ChangeObjStatus(&testCtx, opsRequest, func() {
		opsRequest.Status.CancelTimestamp = metav1.Time{Time: cancelTime}
		opsRequest.Status.Phase = appsv1alpha1.OpsCancellingPhase
	})).Should(Succeed())
	Expect(opsBehaviour.CancelFunc(reqCtx, k8sClient, opsRes)).ShouldNot(HaveOccurred())
}

func mockConsensusCompToRunning(opsRes *OpsResource) {
	// mock consensus component is Running
	compStatus := opsRes.Cluster.Status.Components[consensusComp]
	compStatus.Phase = appsv1alpha1.RunningClusterCompPhase
	opsRes.Cluster.Status.Components[consensusComp] = compStatus
}
