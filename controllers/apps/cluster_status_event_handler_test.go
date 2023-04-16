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
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {

	var (
		ctx                = context.Background()
		clusterName        = ""
		clusterDefName     = ""
		clusterVersionName = ""
	)

	setupResourceNames := func() {
		suffix := testCtx.GetRandomStr()
		clusterName = "cluster-for-status-" + suffix
		clusterDefName = "clusterdef-for-status-" + suffix
		clusterVersionName = "cluster-version-for-status-" + suffix
	}

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		if clusterName != "" {
			testapps.ClearClusterResources(&testCtx)

			inNS := client.InNamespace(testCtx.DefaultNamespace)
			ml := client.HasLabels{testCtx.TestObjLabelKey}
			// testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
			// testapps.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
			testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		}

		// reset all resource names
		setupResourceNames()
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	const statefulMySQLCompType = "stateful"
	const statefulMySQLCompName = "mysql1"

	const consensusMySQLCompType = "consensus"
	const consensusMySQLCompName = "mysql2"

	const nginxCompType = "stateless"
	const nginxCompName = "nginx"

	createClusterDef := func() {
		_ = testapps.NewClusterDefFactory(clusterDefName).
			AddComponent(testapps.StatefulMySQLComponent, statefulMySQLCompType).
			AddComponent(testapps.ConsensusMySQLComponent, consensusMySQLCompType).
			AddComponent(testapps.StatelessNginxComponent, nginxCompType).
			Create(&testCtx)
	}

	createClusterVersion := func() {
		_ = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponent(statefulMySQLCompType).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			AddComponent(consensusMySQLCompType).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			AddComponent(nginxCompType).AddContainerShort(testapps.DefaultNginxContainerName, testapps.NginxImage).
			Create(&testCtx)
	}

	createCluster := func() *appsv1alpha1.Cluster {
		return testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
			AddComponent(statefulMySQLCompName, statefulMySQLCompType).SetReplicas(3).
			AddComponent(consensusMySQLCompName, consensusMySQLCompType).SetReplicas(3).
			AddComponent(nginxCompName, nginxCompType).SetReplicas(3).
			Create(&testCtx).GetObject()
	}

	//createStsPod := func(podName, podRole, componentName string) *corev1.Pod {
	//	return testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
	//		AddAppInstanceLabel(clusterName).
	//		AddAppComponentLabel(componentName).
	//		AddRoleLabel(podRole).
	//		AddAppManangedByLabel().
	//		AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
	//		Create(&testCtx).GetObject()
	//}

	//getDeployment := func(componentName string) *appsv1.Deployment {
	//	deployList := &appsv1.DeploymentList{}
	//	Eventually(func(g Gomega) {
	//		g.Expect(k8sClient.List(ctx, deployList,
	//			client.MatchingLabels{
	//				constant.KBAppComponentLabelKey: componentName,
	//				constant.AppInstanceLabelKey:    clusterName},
	//			client.Limit(1))).ShouldNot(HaveOccurred())
	//		g.Expect(deployList.Items).Should(HaveLen(1))
	//	}).Should(Succeed())
	//	return &deployList.Items[0]
	//}

	//handleAndCheckComponentStatus := func(componentName string, event *corev1.Event,
	//	expectClusterPhase appsv1alpha1.ClusterPhase,
	//	expectCompPhase appsv1alpha1.ClusterComponentPhase,
	//	checkClusterPhase bool) {
	//	Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace},
	//		func(g Gomega, newCluster *appsv1alpha1.Cluster) {
	//			g.Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())
	//			if checkClusterPhase {
	//				g.Expect(newCluster.Status.Phase).Should(Equal(expectClusterPhase))
	//			} else {
	//				compStatus := newCluster.Status.Components[componentName]
	//				g.Expect(compStatus.Phase).Should(Equal(expectCompPhase))
	//			}
	//		})).Should(Succeed())
	//}

	//setInvolvedObject := func(event *corev1.Event, kind, objectName string) {
	//	event.InvolvedObject.Kind = kind
	//	event.InvolvedObject.Name = objectName
	//}

	Context("test cluster Failed/Abnormal phase", func() {
		It("test cluster Failed/Abnormal phase", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVersion()
			//cluster := createCluster()
			createCluster()

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

			// TODO(refactor): mock event for stateful workload
			//By("watch warning event from StatefulSet and component workload type is Stateful")
			//podName0 := stsName + "-0"
			//pod0 := createStsPod(podName0, "", statefulMySQLCompName)
			//Expect(testapps.ChangeObjStatus(&testCtx, pod0, func() {
			//	pod0.Status.ContainerStatuses = []corev1.ContainerStatus{
			//		{
			//			State: corev1.ContainerState{
			//				Waiting: &corev1.ContainerStateWaiting{
			//					Reason:  "ImagePullBackOff",
			//					Message: "Back-off pulling image nginx:latest",
			//				},
			//			},
			//		},
			//	}
			//	pod0.Status.Conditions = []corev1.PodCondition{
			//		{
			//			Type:               corev1.ContainersReady,
			//			Status:             corev1.ConditionFalse,
			//			LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)), // Timed-out
			//		},
			//	}
			//})).Should(Succeed())
			//event.Count = 3
			//event.FirstTimestamp = metav1.Time{Time: time.Now()}
			//event.LastTimestamp = metav1.Time{Time: time.Now().Add(EventTimeOut + time.Second)}
			//handleAndCheckComponentStatus(statefulMySQLCompName, event,
			//	appsv1alpha1.FailedClusterPhase,
			//	appsv1alpha1.FailedClusterCompPhase,
			//	false)

			//By("watch warning event from Pod and component workload type is Consensus")
			//// wait for StatefulSet created by cluster controller
			//stsName = clusterName + "-" + consensusMySQLCompName
			//Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace},
			//	func(g Gomega, fetched *appsv1.StatefulSet) {
			//		g.Expect(fetched.Generation).To(BeEquivalentTo(1))
			//	})).Should(Succeed())
			//// create a failed pod
			//podName := stsName + "-0"
			//createStsPod(podName, "", consensusMySQLCompName)
			//setInvolvedObject(event, constant.PodKind, podName)
			//handleAndCheckComponentStatus(consensusMySQLCompName, event,
			//	appsv1alpha1.FailedClusterPhase,
			//	appsv1alpha1.FailedClusterCompPhase,
			//	false)
			//By("test merge pod event message")
			//event.Message = "0/1 nodes can scheduled, cpu insufficient"
			//handleAndCheckComponentStatus(consensusMySQLCompName, event,
			//	appsv1alpha1.FailedClusterPhase,
			//	appsv1alpha1.FailedClusterCompPhase,
			//	false)

			//By("test Failed phase for consensus component when leader pod is not ready")
			//setInvolvedObject(event, constant.StatefulSetKind, stsName)
			//podName1 := stsName + "-1"
			//pod := createStsPod(podName1, "leader", consensusMySQLCompName)
			//handleAndCheckComponentStatus(consensusMySQLCompName, event,
			//	appsv1alpha1.FailedClusterPhase,
			//	appsv1alpha1.FailedClusterCompPhase,
			//	false)

			//By("test Abnormal phase for consensus component")
			//// mock leader pod ready and sts.status.availableReplicas is 1
			//Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
			//	testk8s.MockPodAvailable(pod, metav1.NewTime(time.Now()))
			//})).ShouldNot(HaveOccurred())
			//Expect(testapps.GetAndChangeObjStatus(&testCtx, types.NamespacedName{Name: stsName,
			//	Namespace: testCtx.DefaultNamespace}, func(tmpSts *appsv1.StatefulSet) {
			//	testk8s.MockStatefulSetReady(tmpSts)
			//	tmpSts.Status.AvailableReplicas = *tmpSts.Spec.Replicas - 1
			//})()).ShouldNot(HaveOccurred())
			//handleAndCheckComponentStatus(consensusMySQLCompName, event,
			//	appsv1alpha1.AbnormalClusterPhase,
			//	appsv1alpha1.AbnormalClusterCompPhase,
			//	false)

			//By("watch warning event from Deployment and component workload type is Stateless")
			//deploy := getDeployment(nginxCompName)
			//setInvolvedObject(event, constant.DeploymentKind, deploy.Name)
			//handleAndCheckComponentStatus(nginxCompName, event,
			//	appsv1alpha1.FailedClusterPhase,
			//	appsv1alpha1.FailedClusterCompPhase,
			//	false)
			// mock cluster is running.
			//Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
			//	tmpCluster.Status.Phase = appsv1alpha1.RunningClusterPhase
			//	for name, compStatus := range tmpCluster.Status.Components {
			//		compStatus.Phase = appsv1alpha1.RunningClusterCompPhase
			//		tmpCluster.Status.SetComponentStatus(name, compStatus)
			//	}
			//})()).ShouldNot(HaveOccurred())

			//By("test the cluster phase when stateless component is Failed and other components are Running")
			//// set nginx component phase to Failed
			//Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
			//	compStatus := tmpCluster.Status.Components[nginxCompName]
			//	compStatus.Phase = appsv1alpha1.FailedClusterCompPhase
			//	tmpCluster.Status.SetComponentStatus(nginxCompName, compStatus)
			//})()).ShouldNot(HaveOccurred())

			// expect cluster phase is Abnormal by cluster controller.
			//Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
			//	func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
			//		g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.AbnormalClusterPhase))
			//	})).Should(Succeed())
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

			changeAndCheckComponents := func(changeFunc func(), expectObservedGeneration int64, checkFun func(Gomega, *appsv1alpha1.Cluster)) {
				Expect(testapps.ChangeObj(&testCtx, cluster, func() {
					changeFunc()
				})).ShouldNot(HaveOccurred())
				// wait for cluster controller reconciles to complete.
				Eventually(testapps.GetClusterObservedGeneration(&testCtx, client.ObjectKeyFromObject(cluster))).Should(Equal(expectObservedGeneration))
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), checkFun)).Should(Succeed())
			}

			By("delete consensus component")
			consensusClusterComponent := cluster.Spec.ComponentSpecs[2]
			changeAndCheckComponents(
				func() {
					cluster.Spec.ComponentSpecs = cluster.Spec.ComponentSpecs[:2]
				}, 2,
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					g.Expect(tmpCluster.Status.Components).Should(HaveLen(2))
				})

			// TODO check when delete and add the same component, wait for deleting related workloads when delete component in lifecycle.
			By("add consensus component")
			consensusClusterComponent.Name = "consensus1"
			changeAndCheckComponents(
				func() {
					cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, consensusClusterComponent)
				}, 3,
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					_, isExist := tmpCluster.Status.Components[consensusClusterComponent.Name]
					g.Expect(tmpCluster.Status.Components).Should(HaveLen(3))
					g.Expect(isExist).Should(BeTrue())
				})

			By("modify consensus component name")
			modifyConsensusName := "consensus2"
			changeAndCheckComponents(
				func() {
					cluster.Spec.ComponentSpecs[2].Name = modifyConsensusName
				}, 4,
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					_, isExist := tmpCluster.Status.Components[modifyConsensusName]
					g.Expect(isExist).Should(BeTrue())
				})
		})
	})
})
