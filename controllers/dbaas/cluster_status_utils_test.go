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

package dbaas

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {

	var (
		ctx                = context.Background()
		clusterName        = "cluster-for-status-" + testCtx.GetRandomStr()
		clusterDefName     = "clusterdef-for-status-" + testCtx.GetRandomStr()
		clusterVersionName = "cluster-version-for-status-" + testCtx.GetRandomStr()
		namespace          = "default"
		timeout            = time.Second * 10
		interval           = time.Second
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		testdbaas.ClearClusterResources(&testCtx)

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	const statefulCompType = "replicasets"
	const statefulCompName = "mysql1"

	const consensusCompType = "consensus"
	const consensusCompName = "mysql2"

	const statelessCompType = "proxy"
	const statelessCompName = "nginx"

	createClusterDef := func() {
		clusterDef := testdbaas.NewClusterDefFactory(&testCtx, clusterDefName, testdbaas.MySQLType).
			AddComponent(testdbaas.StatefulMySQL8, statefulCompType).SetReplicas(3).
			AddComponent(testdbaas.ConsensusMySQL, consensusCompType).SetReplicas(3).
			AddComponent(testdbaas.StatelessNginx, statelessCompType).SetReplicas(3).
			GetClusterDef()
		Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())
	}

	createClusterVersion := func() {
		clusterVersion := testdbaas.NewClusterVersionFactory(&testCtx, clusterVersionName, clusterDefName).
			AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
			AddComponent(consensusCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
			AddComponent(statelessCompType).AddContainerShort("nginx", testdbaas.NginxImage).
			GetClusterVersion()
		Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(Succeed())
	}

	createCluster := func() *dbaasv1alpha1.Cluster {
		cluster := testdbaas.NewClusterFactory(&testCtx, clusterName, clusterDefName, clusterVersionName).
			AddComponent(statefulCompName, statefulCompType).
			AddComponent(consensusCompName, consensusCompType).
			AddComponent(statelessCompName, statelessCompType).
			GetCluster()
		Expect(testCtx.CreateObj(ctx, cluster)).Should(Succeed())
		return cluster
	}

	createSts := func(stsName, componentName string) *appsv1.StatefulSet {
		stsYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  podManagementPolicy: Parallel
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/component-name: %s
      app.kubernetes.io/instance: %s
  serviceName: wesql-wesql-test
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: %s
        app.kubernetes.io/instance: %s
    spec:
      containers:
      - image: docker.io/apecloud/apecloud-mysql-server:latest
        imagePullPolicy: IfNotPresent
        name: mysql`, componentName, clusterName, stsName, componentName, clusterName, componentName, clusterName)
		sts := &appsv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(stsYaml), sts)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, sts)).Should(Succeed())
		return sts
	}

	createStsPod := func(podName, podRole, componentName string) *corev1.Pod {
		return testdbaas.CreateCustomizedObj(&testCtx, "hybrid/hybrid_sts_pod.yaml", &corev1.Pod{},
			testdbaas.CustomizeObjYAML(componentName, clusterName, podRole, podName))
	}

	getDeployment := func(componentName string) *appsv1.Deployment {
		deployList := &appsv1.DeploymentList{}
		Eventually(func() bool {
			Expect(k8sClient.List(ctx, deployList, client.MatchingLabels{
				intctrlutil.AppComponentLabelKey: componentName,
				intctrlutil.AppInstanceLabelKey:  clusterName}, client.Limit(1))).Should(Succeed())
			return len(deployList.Items) == 1
		}).Should(BeTrue())
		return &deployList.Items[0]
	}

	handleAndCheckComponentStatus := func(componentName string, event *corev1.Event,
		expectPhase dbaasv1alpha1.Phase, checkClusterPhase bool) {
		Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKey{Name: clusterName, Namespace: namespace}, func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
			g.Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())
			if checkClusterPhase {
				g.Expect(newCluster.Status.Phase == expectPhase).Should(BeTrue())
				return
			}
			statusComponent := newCluster.Status.Components[componentName]
			g.Expect(statusComponent.Phase == expectPhase).Should(BeTrue())
		}))
	}

	setInvolvedObject := func(event *corev1.Event, kind, objectName string) {
		event.InvolvedObject.Kind = kind
		event.InvolvedObject.Name = objectName
	}

	Context("test cluster Failed/Abnormal phase ", func() {
		It("test cluster Failed/Abnormal phase", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVersion()
			cluster := createCluster()

			// wait for cluster's status to become stable so that it won't interfere with later tests
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKey{Name: clusterName, Namespace: namespace},
				func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
					g.Expect(fetched.Generation).To(BeEquivalentTo(1))
					g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(1))
					g.Expect(fetched.Status.Phase).To(Equal(dbaasv1alpha1.CreatingPhase))
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
			stsName := initComponentStatefulSet(statefulCompName)
			stsInvolvedObject := corev1.ObjectReference{
				Name:      stsName,
				Kind:      intctrlutil.StatefulSetKind,
				Namespace: testCtx.DefaultNamespace,
			}
			event.InvolvedObject = stsInvolvedObject
			event.Type = corev1.EventTypeWarning
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())

			By("watch warning event from StatefulSet and component type is Stateful")
			event.Count = 3
			event.FirstTimestamp = metav1.Time{Time: time.Now()}
			event.LastTimestamp = metav1.Time{Time: time.Now().Add(31 * time.Second)}
			handleAndCheckComponentStatus(statefulCompName, event, dbaasv1alpha1.FailedPhase, false, time.Second*10)

			By("watch warning event from Pod and component type is Consensus")
			// create statefulSet for consensus component
			stsName = initComponentStatefulSet(consensusCompName)
			// create a failed pod
			podName := stsName + "-0"
			createStsPod(podName, "", consensusCompName)
			setInvolvedObject(event, intctrlutil.PodKind, podName)
			handleAndCheckComponentStatus(consensusCompName, event, dbaasv1alpha1.FailedPhase, false, timeout)

			By("test merge pod event message")
			event.Message = "0/1 nodes can scheduled, cpu insufficient"
			handleAndCheckComponentStatus(consensusCompName, event, dbaasv1alpha1.FailedPhase, false, timeout)

			By("test Failed phase for consensus component when leader pod is not ready")
			setInvolvedObject(event, intctrlutil.StatefulSetKind, stsName)
			podName1 := stsName + "-1"
			pod := createStsPod(podName1, "leader", consensusCompName)
			handleAndCheckComponentStatus(consensusCompName, event, dbaasv1alpha1.FailedPhase, false, timeout)

			By("test Abnormal phase for consensus component")
			patch := client.MergeFrom(pod.DeepCopy())
			testk8s.MockPodAvailable(pod, metav1.NewTime(time.Now()))
			Expect(k8sClient.Status().Patch(ctx, pod, patch)).Should(Succeed())
			handleAndCheckComponentStatus(consensusCompName, event, dbaasv1alpha1.AbnormalPhase, false, timeout)

			By("watch warning event from Deployment and component type is Stateless")
			deploymentName := statelessCompName + "-deploy"
			setInvolvedObject(event, intctrlutil.DeploymentKind, deploymentName)
			createDeployment(statelessCompName, deploymentName)
			handleAndCheckComponentStatus(statelessCompName, event, dbaasv1alpha1.FailedPhase, false, timeout)

			By("test the cluster phase when component Failed/Abnormal in Running phase")
			// mock cluster is running.
			Expect(testdbaas.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *dbaasv1alpha1.Cluster) {
				tmpCluster.Status.Phase = dbaasv1alpha1.RunningPhase
				for k := range tmpCluster.Status.Components {
					statusComp := tmpCluster.Status.Components[k]
					statusComp.Phase = dbaasv1alpha1.RunningPhase
					tmpCluster.Status.Components[k] = statusComp
				}
			})()).Should(Succeed())

			// set nginx component phase to Failed
			Expect(testdbaas.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *dbaasv1alpha1.Cluster) {
				statusComp := tmpCluster.Status.Components[statelessCompName]
				statusComp.Phase = dbaasv1alpha1.FailedPhase
				tmpCluster.Status.Components[statelessCompName] = statusComp
			})()).Should(Succeed())

			// expect cluster phase is Abnormal by cluster controller.
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase == dbaasv1alpha1.AbnormalPhase).Should(BeTrue())
			})).Should(Succeed())
		})

		It("test the consistency of status.components and spec.components", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVersion()
			cluster := createCluster()
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				g.Expect(tmpCluster.Generation == tmpCluster.Status.ObservedGeneration).Should(BeTrue())
				g.Expect(len(tmpCluster.Spec.Components) == len(tmpCluster.Status.Components)).Should(BeTrue())
			})).Should(Succeed())

			changeAndCheckComponents := func(changeFunc func(), expectObservedGeneration int64, checkFun func(Gomega, *dbaasv1alpha1.Cluster)) {
				Expect(testdbaas.ChangeObj(&testCtx, cluster, func() {
					changeFunc()
				})).Should(Succeed())

				Expect(testdbaas.ChangeObjStatus(&testCtx, cluster, func() {
					cluster.Status.ObservedGeneration = expectObservedGeneration
				})).Should(Succeed())

				Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), checkFun)).Should(Succeed())

			}

			By("delete consensus component")
			consensusClusterComponent := cluster.Spec.Components[2]
			changeAndCheckComponents(
				func() {
					cluster.Spec.Components = cluster.Spec.Components[:2]
				}, 2,
				func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
					g.Expect(len(tmpCluster.Status.Components) == 2).Should(BeTrue())
				})

			// TODO check when delete and add the same component, wait for deleting related workloads when delete component in lifecycle.
			By("add consensus component")
			consensusClusterComponent.Name = "consensus1"
			changeAndCheckComponents(
				func() {
					cluster.Spec.Components = append(cluster.Spec.Components, consensusClusterComponent)
				}, 3,
				func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
					_, isExist := tmpCluster.Status.Components[consensusClusterComponent.Name]
					g.Expect(len(tmpCluster.Status.Components) == 3 && isExist).Should(BeTrue())
				})

			By("modify consensus component name")
			modifyConsensusName := "consensus2"
			changeAndCheckComponents(
				func() {
					cluster.Spec.Components[2].Name = modifyConsensusName
				}, 4,
				func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
					_, isExist := tmpCluster.Status.Components[modifyConsensusName]
					g.Expect(isExist).Should(BeTrue())
				})

		})
	})

})
