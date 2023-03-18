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
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
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
		Eventually(func() bool {
			Expect(k8sClient.List(ctx, deployList, client.MatchingLabels{
				constant.KBAppComponentLabelKey: componentName,
				constant.AppInstanceLabelKey:    clusterName}, client.Limit(1))).Should(Succeed())
			return len(deployList.Items) == 1
		}).Should(BeTrue())
		return &deployList.Items[0]
	}

	handleAndCheckComponentStatus := func(componentName string, event *corev1.Event,
		expectPhase appsv1alpha1.Phase, checkClusterPhase bool) {
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, func(g Gomega, newCluster *appsv1alpha1.Cluster) {
			g.Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())
			if checkClusterPhase {
				g.Expect(newCluster.Status.Phase == expectPhase).Should(BeTrue())
				return
			}
			compStatus := newCluster.Status.Components[componentName]
			g.Expect(compStatus.Phase == expectPhase).Should(BeTrue())
		})).Should(Succeed())
	}

	setInvolvedObject := func(event *corev1.Event, kind, objectName string) {
		event.InvolvedObject.Kind = kind
		event.InvolvedObject.Name = objectName
	}

	testHandleClusterPhaseWhenCompsNotReady := func(clusterObj *appsv1alpha1.Cluster, componentPhase,
		expectClusterPhase appsv1alpha1.Phase) {
		// mock Stateful component is Abnormal
		clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
			statefulMySQLCompName: {
				Phase: componentPhase,
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
					g.Expect(fetched.Status.Phase).To(Equal(appsv1alpha1.CreatingPhase))
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
			handleAndCheckComponentStatus(statefulMySQLCompName, event, appsv1alpha1.FailedPhase, false)

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
			handleAndCheckComponentStatus(consensusMySQLCompName, event, appsv1alpha1.FailedPhase, false)

			By("test merge pod event message")
			event.Message = "0/1 nodes can scheduled, cpu insufficient"
			handleAndCheckComponentStatus(consensusMySQLCompName, event, appsv1alpha1.FailedPhase, false)

			By("test Failed phase for consensus component when leader pod is not ready")
			setInvolvedObject(event, constant.StatefulSetKind, stsName)
			podName1 := stsName + "-1"
			pod := createStsPod(podName1, "leader", consensusMySQLCompName)
			handleAndCheckComponentStatus(consensusMySQLCompName, event, appsv1alpha1.FailedPhase, false)

			By("test Abnormal phase for consensus component")
			// mock leader pod ready and sts.status.availableReplicas is 1
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				testk8s.MockPodAvailable(pod, metav1.NewTime(time.Now()))
			})).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, types.NamespacedName{Name: stsName,
				Namespace: testCtx.DefaultNamespace}, func(tmpSts *appsv1.StatefulSet) {
				testk8s.MockStatefulSetReady(tmpSts)
				tmpSts.Status.AvailableReplicas = *tmpSts.Spec.Replicas - 1
			})()).Should(Succeed())
			handleAndCheckComponentStatus(consensusMySQLCompName, event, appsv1alpha1.AbnormalPhase, false)

			By("watch warning event from Deployment and component workload type is Stateless")
			deploy := getDeployment(nginxCompName)
			setInvolvedObject(event, constant.DeploymentKind, deploy.Name)
			handleAndCheckComponentStatus(nginxCompName, event, appsv1alpha1.FailedPhase, false)

			// mock cluster is running.
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Status.Phase = appsv1alpha1.RunningPhase
				for name, compStatus := range tmpCluster.Status.Components {
					compStatus.Phase = appsv1alpha1.RunningPhase
					tmpCluster.Status.SetComponentStatus(name, compStatus)
				}
			})()).Should(Succeed())

			By("test the cluster phase when stateless component is Failed and other components are Running")
			// set nginx component phase to Failed
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				compStatus := tmpCluster.Status.Components[nginxCompName]
				compStatus.Phase = appsv1alpha1.FailedPhase
				tmpCluster.Status.SetComponentStatus(nginxCompName, compStatus)
			})()).Should(Succeed())

			// expect cluster phase is Abnormal by cluster controller.
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					g.Expect(tmpCluster.Status.Phase == appsv1alpha1.AbnormalPhase).Should(BeTrue())
				})).Should(Succeed())

			By("test the cluster phase when cluster only contains a component of Stateful workload, and the component is Failed or Abnormal")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(statefulMySQLCompName, statefulMySQLCompType).SetReplicas(3).GetObject()
			// mock Stateful component is Failed and expect cluster phase is FailedPhase
			testHandleClusterPhaseWhenCompsNotReady(clusterObj, appsv1alpha1.FailedPhase, appsv1alpha1.FailedPhase)

			// mock Stateful component is Abnormal and expect cluster phase is Abnormal
			testHandleClusterPhaseWhenCompsNotReady(clusterObj, appsv1alpha1.AbnormalPhase, appsv1alpha1.AbnormalPhase)
		})

		It("test the consistency of status.components and spec.components", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVersion()
			cluster := createCluster()
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Generation == tmpCluster.Status.ObservedGeneration).Should(BeTrue())
				g.Expect(len(tmpCluster.Spec.ComponentSpecs) == len(tmpCluster.Status.Components)).Should(BeTrue())
			})).Should(Succeed())

			changeAndCheckComponents := func(changeFunc func(), expectObservedGeneration int64, checkFun func(Gomega, *appsv1alpha1.Cluster)) {
				Expect(testapps.ChangeObj(&testCtx, cluster, func() {
					changeFunc()
				})).Should(Succeed())
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
					g.Expect(len(tmpCluster.Status.Components) == 2).Should(BeTrue())
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
					g.Expect(len(tmpCluster.Status.Components) == 3 && isExist).Should(BeTrue())
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
