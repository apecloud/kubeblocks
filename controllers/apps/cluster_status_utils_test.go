/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package apps

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {

	var (
		ctx                = context.Background()
		clusterName        = "cluster-for-status-" + testCtx.GetRandomStr()
		clusterDefName     = "clusterdef-for-status-" + testCtx.GetRandomStr()
		clusterVersionName = "cluster-version-for-status-" + testCtx.GetRandomStr()
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		testapps.ClearClusterResources(&testCtx)

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PodSignature, true, inNS, ml)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	const statefulMySQLCompDefName = "stateful"
	const statefulMySQLCompName = "mysql1"

	const consensusMySQLCompDefName = "consensus"
	const consensusMySQLCompName = "mysql2"

	const statelessCompDefName = "stateless"
	const nginxCompName = "nginx"

	createClusterDef := func() {
		_ = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulMySQLCompDefName).
			AddComponentDef(testapps.ConsensusMySQLComponent, consensusMySQLCompDefName).
			AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
			Create(&testCtx)
	}

	createClusterVersion := func() {
		_ = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponent(statefulMySQLCompDefName).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			AddComponent(consensusMySQLCompDefName).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			AddComponent(statelessCompDefName).AddContainerShort(testapps.DefaultNginxContainerName, testapps.NginxImage).
			Create(&testCtx)
	}

	createCluster := func() *appsv1alpha1.Cluster {
		return testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
			AddComponent(statefulMySQLCompName, statefulMySQLCompDefName).SetReplicas(3).
			AddComponent(consensusMySQLCompName, consensusMySQLCompDefName).SetReplicas(3).
			AddComponent(nginxCompName, statelessCompDefName).SetReplicas(3).
			Create(&testCtx).GetObject()
	}

	createStsPod := func(podName, podRole, componentName string) *corev1.Pod {
		return testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
			AddAppInstanceLabel(clusterName).
			AddAppComponentLabel(componentName).
			AddRoleLabel(podRole).
			AddAppManangedByLabel().
			AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
			Create(&testCtx).GetObject()
	}

	getDeployment := func(componentName string) *appsv1.Deployment {
		deployList := &appsv1.DeploymentList{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, deployList,
				client.MatchingLabels{
					constant.KBAppComponentLabelKey: componentName,
					constant.AppInstanceLabelKey:    clusterName},
				client.Limit(1))).ShouldNot(HaveOccurred())
			g.Expect(deployList.Items).Should(HaveLen(1))
		}).Should(Succeed())
		return &deployList.Items[0]
	}

	handleAndCheckComponentStatus := func(componentName string, event *corev1.Event,
		expectClusterPhase appsv1alpha1.ClusterPhase,
		expectCompPhase appsv1alpha1.ClusterComponentPhase,
		checkClusterPhase bool) {
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace},
			func(g Gomega, newCluster *appsv1alpha1.Cluster) {
				g.Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())
				if checkClusterPhase {
					g.Expect(newCluster.Status.Phase == expectClusterPhase).Should(BeTrue())
					return
				}
				compStatus := newCluster.Status.Components[componentName]
				g.Expect(compStatus.Phase == expectCompPhase).Should(BeTrue())
			})).Should(Succeed())
	}

	setInvolvedObject := func(event *corev1.Event, kind, objectName string) {
		event.InvolvedObject.Kind = kind
		event.InvolvedObject.Name = objectName
	}

	testHandleClusterPhaseWhenCompsNotReady := func(clusterObj *appsv1alpha1.Cluster,
		compPhase appsv1alpha1.ClusterComponentPhase,
		expectClusterPhase appsv1alpha1.ClusterPhase,
	) {
		// mock Stateful component is Abnormal
		clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
			statefulMySQLCompName: {
				Phase: compPhase,
			},
		}
		handleClusterPhaseWhenCompsNotReady(clusterObj, nil, nil)
		Expect(clusterObj.Status.Phase).Should(Equal(expectClusterPhase))
	}

	Context("test cluster Failed/Abnormal phase", func() {
		It("test cluster Failed/Abnormal phase", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVersion()
			cluster := createCluster()

			// wait for cluster's status to become stable so that it won't interfere with later tests
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace},
				func(g Gomega, fetched *appsv1alpha1.Cluster) {
					g.Expect(fetched.Generation).To(BeEquivalentTo(1))
					g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(1))
					g.Expect(fetched.Status.Phase).To(Equal(appsv1alpha1.CreatingClusterPhase))
				})).Should(Succeed())

			By("watch normal event")
			event := &corev1.Event{
				Count:   1,
				Type:    corev1.EventTypeNormal,
				Message: "create pod failed because the pvc is deleting",
			}
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())

			By("watch warning event from StatefulSet, but mismatch condition ")
			// wait for StatefulSet created by cluster controller
			stsName := clusterName + "-" + statefulMySQLCompName
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace},
				func(g Gomega, fetched *appsv1.StatefulSet) {
					g.Expect(fetched.Generation).To(BeEquivalentTo(1))
				})).Should(Succeed())
			stsInvolvedObject := corev1.ObjectReference{
				Name:      stsName,
				Kind:      constant.StatefulSetKind,
				Namespace: testCtx.DefaultNamespace,
			}
			event.InvolvedObject = stsInvolvedObject
			event.Type = corev1.EventTypeWarning
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())

			By("watch warning event from StatefulSet and component workload type is Stateful")
			event.Count = 3
			event.FirstTimestamp = metav1.Time{Time: time.Now()}
			event.LastTimestamp = metav1.Time{Time: time.Now().Add(31 * time.Second)}
			handleAndCheckComponentStatus(statefulMySQLCompName, event,
				appsv1alpha1.FailedClusterPhase,
				appsv1alpha1.FailedClusterCompPhase,
				false)

			By("watch warning event from Pod and component workload type is Consensus")
			// wait for StatefulSet created by cluster controller
			stsName = clusterName + "-" + consensusMySQLCompName
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace},
				func(g Gomega, fetched *appsv1.StatefulSet) {
					g.Expect(fetched.Generation).To(BeEquivalentTo(1))
				})).Should(Succeed())
			// create a failed pod
			podName := stsName + "-0"
			createStsPod(podName, "", consensusMySQLCompName)
			setInvolvedObject(event, constant.PodKind, podName)
			handleAndCheckComponentStatus(consensusMySQLCompName, event,
				appsv1alpha1.FailedClusterPhase,
				appsv1alpha1.FailedClusterCompPhase,
				false)
			By("test merge pod event message")
			event.Message = "0/1 nodes can scheduled, cpu insufficient"
			handleAndCheckComponentStatus(consensusMySQLCompName, event,
				appsv1alpha1.FailedClusterPhase,
				appsv1alpha1.FailedClusterCompPhase,
				false)

			By("test Failed phase for consensus component when leader pod is not ready")
			setInvolvedObject(event, constant.StatefulSetKind, stsName)
			podName1 := stsName + "-1"
			pod := createStsPod(podName1, "leader", consensusMySQLCompName)
			handleAndCheckComponentStatus(consensusMySQLCompName, event,
				appsv1alpha1.FailedClusterPhase,
				appsv1alpha1.FailedClusterCompPhase,
				false)

			By("test Abnormal phase for consensus component")
			// mock leader pod ready and sts.status.availableReplicas is 1
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				testk8s.MockPodAvailable(pod, metav1.NewTime(time.Now()))
			})).ShouldNot(HaveOccurred())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, types.NamespacedName{Name: stsName,
				Namespace: testCtx.DefaultNamespace}, func(tmpSts *appsv1.StatefulSet) {
				testk8s.MockStatefulSetReady(tmpSts)
				tmpSts.Status.AvailableReplicas = *tmpSts.Spec.Replicas - 1
			})()).ShouldNot(HaveOccurred())
			handleAndCheckComponentStatus(consensusMySQLCompName, event,
				appsv1alpha1.AbnormalClusterPhase,
				appsv1alpha1.AbnormalClusterCompPhase,
				false)

			By("watch warning event from Deployment and component workload type is Stateless")
			deploy := getDeployment(nginxCompName)
			setInvolvedObject(event, constant.DeploymentKind, deploy.Name)
			handleAndCheckComponentStatus(nginxCompName, event,
				appsv1alpha1.FailedClusterPhase,
				appsv1alpha1.FailedClusterCompPhase,
				false)
			// mock cluster is running.
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Status.Phase = appsv1alpha1.RunningClusterPhase
				for name, compStatus := range tmpCluster.Status.Components {
					compStatus.Phase = appsv1alpha1.RunningClusterCompPhase
					tmpCluster.Status.SetComponentStatus(name, compStatus)
				}
			})()).ShouldNot(HaveOccurred())

			By("test the cluster phase when stateless component is Failed and other components are Running")
			// set nginx component phase to Failed
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				compStatus := tmpCluster.Status.Components[nginxCompName]
				compStatus.Phase = appsv1alpha1.FailedClusterCompPhase
				tmpCluster.Status.SetComponentStatus(nginxCompName, compStatus)
			})()).ShouldNot(HaveOccurred())

			// expect cluster phase is Abnormal by cluster controller.
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.AbnormalClusterPhase))
				})).Should(Succeed())

			By("test the cluster phase when cluster only contains a component of Stateful workload, and the component is Failed or Abnormal")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(statefulMySQLCompName, statefulMySQLCompDefName).SetReplicas(3).GetObject()
			// mock Stateful component is Failed and expect cluster phase is FailedPhase
			testHandleClusterPhaseWhenCompsNotReady(clusterObj, appsv1alpha1.FailedClusterCompPhase, appsv1alpha1.FailedClusterPhase)

			// mock Stateful component is Abnormal and expect cluster phase is Abnormal
			testHandleClusterPhaseWhenCompsNotReady(clusterObj, appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.AbnormalClusterPhase)
		})

		It("test the consistency of status.components and spec.components", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVersion()
			cluster := createCluster()
			// REVIEW: follow expects is rather inaccurate
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.ObservedGeneration).Should(Equal(tmpCluster.Generation))
				// g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
				g.Expect(tmpCluster.Status.Components).Should(HaveLen(len(tmpCluster.Spec.ComponentSpecs)))
			})).Should(Succeed())

			changeAndCheckComponents := func(changeFunc func(*appsv1alpha1.Cluster), expectObservedGeneration int64, checkFun func(Gomega, *appsv1alpha1.Cluster)) {
				Expect(testapps.ChangeObj(&testCtx, cluster, func(lcluster *appsv1alpha1.Cluster) {
					changeFunc(lcluster)
				})).ShouldNot(HaveOccurred())
				// wait for cluster controller reconciles to complete.
				Eventually(testapps.GetClusterObservedGeneration(&testCtx, client.ObjectKeyFromObject(cluster))).Should(Equal(expectObservedGeneration))
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), checkFun)).Should(Succeed())
			}

			By("delete consensus component")
			consensusClusterComponent := cluster.Spec.ComponentSpecs[2]
			changeAndCheckComponents(
				func(lcluster *appsv1alpha1.Cluster) {
					lcluster.Spec.ComponentSpecs = cluster.Spec.ComponentSpecs[:2]
				}, 2,
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					g.Expect(tmpCluster.Status.Components).Should(HaveLen(2))
				})

			// TODO check when delete and add the same component, wait for deleting related workloads when delete component in lifecycle.
			By("add consensus component")
			consensusClusterComponent.Name = "consensus1"
			changeAndCheckComponents(
				func(lcluster *appsv1alpha1.Cluster) {
					lcluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, consensusClusterComponent)
				}, 3,
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					_, isExist := tmpCluster.Status.Components[consensusClusterComponent.Name]
					g.Expect(tmpCluster.Status.Components).Should(HaveLen(3))
					g.Expect(isExist).Should(BeTrue())
				})

			By("modify consensus component name")
			modifyConsensusName := "consensus2"
			changeAndCheckComponents(
				func(lcluster *appsv1alpha1.Cluster) {
					lcluster.Spec.ComponentSpecs[2].Name = modifyConsensusName
				}, 4,
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					_, isExist := tmpCluster.Status.Components[modifyConsensusName]
					g.Expect(isExist).Should(BeTrue())
				})

		})
	})

})
