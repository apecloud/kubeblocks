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
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("StatefulSet Controller", func() {

	var (
		randomStr      = testCtx.GetRandomStr()
		timeout        = time.Second * 20
		interval       = time.Second
		clusterName    = "wesql-" + randomStr
		clusterDefName = "cluster-definition-consensus-" + randomStr
		appVersionName = "app-version-operations-" + randomStr
		stsName        = "wesql-wesql-test-" + randomStr
		namespace      = "default"
		opsRequestName = "wesql-restart-test-" + randomStr
	)

	cleanWorkloads := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanWorkloads()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanWorkloads()
	})

	createCluster := func() *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
       kubeblocks.io/ops-request: |
          {"Updating":"%s"}
  labels:
    appversion.kubeblocks.io/name: app-version-consensus
    clusterdefinition.kubeblocks.io/name: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  appVersionRef: %s
  clusterDefinitionRef: %s
  components:
  - monitor: false
    name: wesql-test
    replicas: 3
    type: replicasets
  terminationPolicy: WipeOut
`, opsRequestName, clusterDefName, clusterName, appVersionName, clusterDefName)
		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), cluster)).Should(Succeed())
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
      leader:
        accessMode: ReadWrite
        name: leader
      updateStrategy: BestEffortParallel
    defaultReplicas: 3
  type: state.mysql-8`, clusterDefName)
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), clusterDef)).Should(Succeed())
		// wait until clusterDef created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefName}, &dbaasv1alpha1.ClusterDefinition{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	createAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:  %s
spec:
  clusterDefinitionRef: %s
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server-8.0:0.1.2
`, appVersionName, clusterDefName)
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, appVersion)).Should(Succeed())
		return appVersion
	}

	createOpsRequest := func() {
		opsRequestYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  labels:
    app.kubernetes.io/instance: wesql
    app.kubernetes.io/managed-by: kubeblocks
  namespace: default
spec:
  clusterRef: %s
  componentOps:
  - componentNames:
    - wesql-test
  type: Restart`, opsRequestName, clusterName)
		ops := &dbaasv1alpha1.OpsRequest{}
		Expect(yaml.Unmarshal([]byte(opsRequestYaml), ops)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), ops)).Should(Succeed())
		// wait until opsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequestName, Namespace: namespace}, &dbaasv1alpha1.OpsRequest{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	createStatefulSet := func() *appsv1.StatefulSet {
		statefulsetYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/component-name: wesql-test
    app.kubernetes.io/instance: %s
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
      - args:
        - |
          cluster_info=""; for (( i=0; i<$KB_REPLICASETS_N; i++ )); do
            if [ $i -ne 0 ]; then
              cluster_info="$cluster_info;";
            fi;
            host=$(eval echo \$KB_REPLICASETS_"$i"_HOSTNAME)
            cluster_info="$cluster_info$host:13306";
          done; idx=0; while IFS='-' read -ra ADDR; do
            for i in "${ADDR[@]}"; do
              idx=$i;
            done;
          done <<< "$KB_POD_NAME"; echo $idx; cluster_info="$cluster_info@$(($idx+1))"; echo $cluster_info; docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
        command:
        - /bin/bash
        - -c
        env:
        - name: MYSQL_ROOT_HOST
          value: '%s'
        - name: MYSQL_ROOT_USER
          value: root
        - name: MYSQL_ROOT_PASSWORD
        - name: MYSQL_ALLOW_EMPTY_PASSWORD
          value: "yes"
        - name: MYSQL_DATABASE
          value: mydb
        - name: MYSQL_USER
          value: u1
        - name: MYSQL_PASSWORD
          value: u1
        - name: CLUSTER_ID
          value: "1"
        - name: CLUSTER_START_INDEX
          value: "1"
        - name: REPLICATIONUSER
          value: replicator
        - name: REPLICATION_PASSWORD
        - name: MYSQL_TEMPLATE_CONFIG
        - name: MYSQL_CUSTOM_CONFIG
        - name: MYSQL_DYNAMIC_CONFIG
        - name: KB_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: KB_REPLICASETS_N
          value: "3"
        - name: KB_REPLICASETS_0_HOSTNAME
          value: wesql-wesql-test-0.wesql-wesql-test-headless
        - name: KB_REPLICASETS_1_HOSTNAME
          value: wesql-wesql-test-1.wesql-wesql-test-headless
        - name: KB_REPLICASETS_2_HOSTNAME
          value: wesql-wesql-test-2.wesql-wesql-test-headless
        image: docker.io/apecloud/wesql-server:8.0.30-4.alpha4.20221117.gba56235
        imagePullPolicy: IfNotPresent
        name: mysql
        ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        - containerPort: 13306
          name: paxos
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
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
  updatedReplicas: 3`, clusterName, stsName, "%")
		sts := &appsv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(statefulsetYaml), sts)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), sts)).Should(Succeed())
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
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    controller-revision-hash: wesql-wesql-test-6fdd48d9cd
    cs.dbaas.kubeblocks.io/access-mode: %s
    cs.dbaas.kubeblocks.io/role: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  containers:
  - args:
    - |
      cluster_info=""; for (( i=0; i<$KB_REPLICASETS_N; i++ )); do
        if [ $i -ne 0 ]; then
          cluster_info="$cluster_info;";
        fi;
        host=$(eval echo \$KB_REPLICASETS_"$i"_HOSTNAME)
        cluster_info="$cluster_info$host:13306";
      done; idx=0; while IFS='-' read -ra ADDR; do
        for i in "${ADDR[@]}"; do
          idx=$i;
        done;
      done <<< "$KB_POD_NAME"; echo $idx; cluster_info="$cluster_info@$(($idx+1))"; echo $cluster_info; docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
    command:
    - /bin/bash
    - -c
    env:
    - name: MYSQL_ROOT_HOST
      value: '%s'
    - name: MYSQL_ROOT_USER
      value: root
    - name: MYSQL_ROOT_PASSWORD
    - name: MYSQL_ALLOW_EMPTY_PASSWORD
      value: "yes"
    - name: MYSQL_DATABASE
      value: mydb
    - name: MYSQL_USER
      value: u1
    - name: MYSQL_PASSWORD
      value: u1
    - name: CLUSTER_ID
      value: "1"
    - name: CLUSTER_START_INDEX
      value: "1"
    - name: REPLICATIONUSER
      value: replicator
    - name: REPLICATION_PASSWORD
    - name: MYSQL_TEMPLATE_CONFIG
    - name: MYSQL_CUSTOM_CONFIG
    - name: MYSQL_DYNAMIC_CONFIG
    - name: KB_POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    - name: KB_REPLICASETS_N
      value: "3"
    - name: KB_REPLICASETS_0_HOSTNAME
      value: wesql-wesql-test-0.wesql-wesql-test-headless
    - name: KB_REPLICASETS_1_HOSTNAME
      value: wesql-wesql-test-1.wesql-wesql-test-headless
    - name: KB_REPLICASETS_2_HOSTNAME
      value: wesql-wesql-test-2.wesql-wesql-test-headless
    image: docker.io/apecloud/wesql-server:8.0.30-4.alpha4.20221117.gba56235
    imagePullPolicy: IfNotPresent
    name: mysql
    ports:
    - containerPort: 3306
      name: mysql
      protocol: TCP
    - containerPort: 13306
      name: paxos
      protocol: TCP
    resources: {}
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File`, clusterName, accessMode, podRole, podName, "%")
		pod := &corev1.Pod{}
		Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), pod)).Should(Succeed())
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

	patchPodLabel := func(podName, podRole, accessMode, revision string) {
		pod := &corev1.Pod{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, pod)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		patch := client.MergeFrom(pod.DeepCopy())
		pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] = podRole
		pod.Labels[intctrlutil.ConsensusSetAccessModeLabelKey] = accessMode
		pod.Labels[appsv1.ControllerRevisionHashLabelKey] = revision
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

	testUsingEnvTest := func(sts *appsv1.StatefulSet) {
		By("create pod of statefulset")
		for i := 0; i < 3; i++ {
			podName := fmt.Sprintf("wesql-wesql-test-%s-%d", randomStr, i)
			podRole := "follower"
			accessMode := "Readonly"
			if i == 0 {
				podRole = "leader"
				accessMode = "ReadWrite"
			}
			// mock StatefulSet to create all pods
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
	}

	testUsingRealCluster := func() {
		newSts := &appsv1.StatefulSet{}
		// wait for StatefulSet to create all pods
		Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: namespace}, newSts)
			return newSts.Status.ObservedGeneration == 1
		}, timeout, interval).Should(BeTrue())
		By("patch pod label of StatefulSet")
		for i := 0; i < 3; i++ {
			podName := fmt.Sprintf("wesql-wesql-test-%s-%d", randomStr, i)
			podRole := "follower"
			accessMode := "Readonly"
			if i == 0 {
				podRole = "leader"
				accessMode = "ReadWrite"
			}
			// patch pod label to reach the conditions, then component status will change to Running
			patchPodLabel(podName, podRole, accessMode, newSts.Status.UpdateRevision)
		}
	}

	Context("test controller", func() {
		It("", func() {
			createClusterDef()
			createAppVersionObj()
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

			By("mock the StatefulSet and pods are ready")
			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
				testUsingRealCluster()
			} else {
				testUsingEnvTest(sts)
			}

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
