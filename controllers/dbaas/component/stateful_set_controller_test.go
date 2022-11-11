/*
Copyright ApeCloud Inc.

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

package component

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("StatefulSet Controller", func() {
	var (
		timeout        = time.Second * 20
		interval       = time.Second
		clusterName    = "wesql"
		clusterDefName = "cluster-definition-consensus"
		stsName        = "wesql-wesql-test"
		namespace      = "default"
	)

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	createCluster := func() *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
       kubeblocks.io/ops-request: |
          {"Updating":"wesql-restart-test"}
  labels:
    appversion.kubeblocks.io/name: app-version-consensus
    clusterdefinition.kubeblocks.io/name: cluster-definition-consensus
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  appVersionRef: app-version-consensus
  clusterDefinitionRef: cluster-definition-consensus
  components:
  - monitor: false
    name: wesql-test
    replicas: 3
    type: replicasets
  terminationPolicy: WipeOut
status:
  clusterDefGeneration: 2
  components:
    wesql-test:
      consensusSetStatus:
        followers:
        - accessMode: Readonly
          name: follower
          pod: wesql-wesql-test-1
        - accessMode: Readonly
          name: follower
          pod: wesql-wesql-test-2
        - accessMode: Readonly
          name: follower
          pod: 
        leader:
          accessMode: ReadWrite
          name: leader
          pod: wesql-wesql-test-0
      phase: Running
  observedGeneration: 2
  operations:
    horizontalScalable:
    - name: wesql-test
    restartable:
    - wesql-test
    verticalScalable:
    - wesql-test
  phase: Running`, clusterName)
		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), cluster)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, &dbaasv1alpha1.Cluster{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return cluster
	}

	createClusterDef := func() {
		clusterDefYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  components:
  - antiAffinity: false
    componentType: Consensus
    typeName: replicasets
    consensusSpec:
      followers:
      - accessMode: Readonly
        name: follower
        replicas: 0
      leader:
        accessMode: ReadWrite
        name: leader
        replicas: 0
      updateStrategy: BestEffortParallel
    defaultReplicas: 3
    minAvailable: 0
  type: state.mysql-8
status:
  observedGeneration: 1
  phase: Available`, clusterDefName)
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), clusterDef)).Should(Succeed())
		// wait until clusterDef created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefName}, &dbaasv1alpha1.ClusterDefinition{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	createOpsRequest := func() {
		opsRequestYaml := `apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: wesql-restart-test
  labels:
    cluster.kubeblocks.io/name: wesql
    app.kubernetes.io/managed-by: kubeblocks
  namespace: default
spec:
  clusterRef: wesql
  componentOps:
  - componentNames:
    - wesql-test
  type: Restart`
		ops := &dbaasv1alpha1.OpsRequest{}
		Expect(yaml.Unmarshal([]byte(opsRequestYaml), ops)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), ops)).Should(Succeed())
		// wait until opsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "wesql-restart-test", Namespace: namespace}, &dbaasv1alpha1.OpsRequest{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	createStatefulSet := func() *appsv1.StatefulSet {
		statefulsetYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/component-name: wesql-test
    app.kubernetes.io/instance: wesql
    app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  podManagementPolicy: Parallel
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/component-name: wesql-test
      app.kubernetes.io/instance: wesql
      app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
  serviceName: wesql-wesql-test
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: wesql-test
        app.kubernetes.io/instance: wesql
        app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    spec:
      containers:
      - image: docker.io/apecloud/wesql-server-8.0:0.1.2
        imagePullPolicy: IfNotPresent
        name: mysql
  updateStrategy:
    type: OnDelete
status:
  availableReplicas: 3
  collisionCount: 0
  currentRevision: wesql-wesql-test-7cbdcbfb5c
  observedGeneration: 3
  readyReplicas: 3
  replicas: 3
  updateRevision: wesql-wesql-test-5cd4fc6699
  updatedReplicas: 3`, stsName)
		sts := &appsv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(statefulsetYaml), sts)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), sts)).Should(Succeed())
		// wait until statefulset created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: namespace}, &appsv1.StatefulSet{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return sts
	}

	createStsPod := func(podName, podRole, accessMode string) {
		podYaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/component-name: wesql-test
    app.kubernetes.io/instance: wesql
    app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    controller-revision-hash: wesql-wesql-test-6fdd48d9cd
    cs.dbaas.kubeblocks.io/access-mode: %s
    cs.dbaas.kubeblocks.io/role: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  containers:
  - image: docker.io/apecloud/wesql-server-8.0:0.1.2
    imagePullPolicy: IfNotPresent
    name: mysql`, accessMode, podRole, podName)
		pod := &corev1.Pod{}
		Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), pod)).Should(Succeed())
		// wait until pod created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, &corev1.Pod{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		patch := client.MergeFrom(pod.DeepCopy())
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		}
		Expect(k8sClient.Status().Patch(context.Background(), pod, patch)).Should(Succeed())
	}

	testUpdateStrategy := func(updateStrategy dbaasv1alpha1.UpdateStrategy, componentName string, index int) {
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefName}, clusterDef)).Should(Succeed())
		clusterDef.Spec.Components[0].ConsensusSpec.UpdateStrategy = dbaasv1alpha1.Serial
		Expect(k8sClient.Update(context.Background(), clusterDef)).Should(Succeed())

		newSts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: namespace}, newSts)).Should(Succeed())
		stsPatch := client.MergeFrom(newSts.DeepCopy())
		newSts.Status.CurrentRevision = fmt.Sprintf("wesql-wesql-test-%dfdd48d8cd", index)
		Expect(k8sClient.Status().Patch(context.Background(), newSts, stsPatch)).Should(Succeed())

		By("waiting the component is Running")
		Eventually(func() bool {
			cluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
			return cluster.Status.Components[componentName].Phase == dbaasv1alpha1.RunningPhase
		}, timeout, interval).Should(BeTrue())
	}

	Context("test controller", func() {
		It("", func() {
			createClusterDef()
			cluster := createCluster()
			createOpsRequest()
			sts := createStatefulSet()

			By("patch cluster to Updating")
			componentName := "wesql-test"
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
			cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{
				componentName: {
					Phase: dbaasv1alpha1.UpdatingPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())

			By("create pod of statefulset")
			for i := 0; i < 3; i++ {
				podName := fmt.Sprintf("wesql-wesql-test-%d", i)
				podRole := "follower"
				accessMode := "Readonly"
				if i == 0 {
					podRole = "leader"
					accessMode = "ReadWrite"
				}
				createStsPod(podName, podRole, accessMode)
			}

			By("mock restart cluster")
			sts.Spec.Template.Annotations = map[string]string{
				"kubeblocks.io/restart": time.Now().Format(time.RFC3339),
			}
			Expect(k8sClient.Update(context.Background(), sts)).Should(Succeed())

			By("mock statefulset is ready")
			newSts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: namespace}, newSts)).Should(Succeed())
			stsPatch := client.MergeFrom(newSts.DeepCopy())
			newSts.Status.UpdateRevision = "wesql-wesql-test-6fdd48d9cd"
			newSts.Status.ObservedGeneration = 2
			newSts.Status.AvailableReplicas = 3
			newSts.Status.ReadyReplicas = 3
			newSts.Status.Replicas = 3
			Expect(k8sClient.Status().Patch(context.Background(), newSts, stsPatch)).Should(Succeed())

			By("waiting the component is Running")
			Eventually(func() bool {
				cluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
				return cluster.Status.Components[componentName].Phase == dbaasv1alpha1.RunningPhase
			}, timeout, interval).Should(BeTrue())

			By("test updateStrategy with Serial")
			testUpdateStrategy(dbaasv1alpha1.Serial, componentName, 1)

			By("test updateStrategy with Parallel")
			testUpdateStrategy(dbaasv1alpha1.Parallel, componentName, 2)
		})
	})
})
