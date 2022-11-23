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

package dbaas

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {

	var (
		ctx         = context.Background()
		clusterName = "cluster-for-status"
		namespace   = "default"
		timeout     = time.Second * 20
		interval    = time.Second
	)

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	createClusterDef := func() {
		clusterDefYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     cluster-definition-for-status
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    pdbSpec:
      minAvailable: 1
    componentType: Stateful
    defaultReplicas: 3
    podSpec:
      containers:
      - name: mysql
  - typeName: consensus
    componentType: Consensus
    defaultReplicas: 3
    consensusSpec:
      leader:
        name: "leader"
        accessMode: ReadWrite
      followers:
      - name: "follower"
        accessMode: Readonly
    podSpec:
      containers:
      - name: mysql
  - typeName: proxy
    defaultReplicas: 3
    componentType: Stateless
    podSpec:
      containers:
      - name: nginx
`
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
	}

	createAppversion := func() {
		appVerYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version-for-status
spec:
  clusterDefinitionRef: cluster-definition-for-status
  components:
  - type: consensus
    podSpec:
      containers:
      - name: mysql
        image: registry.jihulab.com/apecloud/mysql-server/mysql/wesql-server-arm:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
  - type: replicasets
    podSpec: 
      containers:
      - name: mysql
        image: registry.jihulab.com/apecloud/mysql-server/mysql/wesql-server-arm:latest
`
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYaml), appVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, appVersion)).Should(Succeed())
	}

	createCluster := func() *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  appVersionRef: app-version-for-status
  clusterDefinitionRef: cluster-definition-for-status
  components:
  - monitor: false
    name: replicasets
    replicas: 3
    type: replicasets
  - monitor: false
    name: nginx
    type: proxy
  - monitor: false
    name: consensus
    type: consensus
  terminationPolicy: WipeOut
`, clusterName)
		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, &dbaasv1alpha1.Cluster{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
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
      - image: docker.io/apecloud/wesql-server-8.0:0.1.2
        imagePullPolicy: IfNotPresent
        name: mysql`, componentName, clusterName, stsName, componentName, clusterName, componentName, clusterName)
		sts := &appsv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(stsYaml), sts)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, sts)).Should(Succeed())
		return sts
	}

	createStsPod := func(podName, podRole, componentName string) {
		podYaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    cs.dbaas.kubeblocks.io/role: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  containers:
  - image: docker.io/apecloud/wesql-server-8.0:0.1.2
    imagePullPolicy: IfNotPresent
    name: mysql`, componentName, clusterName, podRole, podName)
		pod := &corev1.Pod{}
		Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), pod)).Should(Succeed())
		// wait until pod created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, &corev1.Pod{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	createDeployment := func(componentName, deployName string) {
		deploymentYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  minReadySeconds: 10
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/component-name: %s
      app.kubernetes.io/instance: %s
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: %s
        app.kubernetes.io/instance: %s
    spec:
      containers:
      - image: nginx:latest
        imagePullPolicy: IfNotPresent
        name: nginx
`, componentName, clusterName, deployName, componentName, clusterName, componentName, clusterName)
		deploy := &appsv1.Deployment{}
		Expect(yaml.Unmarshal([]byte(deploymentYaml), deploy)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), deploy)).Should(Succeed())
		// wait until deployment created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: deployName, Namespace: namespace}, &appsv1.Deployment{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	initComponentStatefulSet := func(componentName string) string {
		stsName := clusterName + "-" + componentName
		sts := createSts(stsName, componentName)
		// wait until statefulSet created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: sts.Namespace}, &appsv1.StatefulSet{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return stsName
	}

	handleAndCheckComponentStatus := func(componentName string, event *corev1.Event, expectPhase dbaasv1alpha1.Phase, checkClusterPhase bool) {
		Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())
		Eventually(func() bool {
			newCluster := &dbaasv1alpha1.Cluster{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: namespace}, newCluster); err != nil {
				return false
			}
			statusComponents := newCluster.Status.Components
			if statusComponents == nil || statusComponents[componentName] == nil {
				return false
			}
			if checkClusterPhase {
				return statusComponents[componentName].Phase == expectPhase &&
					newCluster.Status.Phase == expectPhase
			}
			return statusComponents[componentName].Phase == expectPhase
		}, timeout*3, interval).Should(BeTrue())

	}

	setInvolvedObject := func(event *corev1.Event, kind, objectName string) {
		event.InvolvedObject.Kind = kind
		event.InvolvedObject.Name = objectName
	}

	Context("test cluster Failed/Abnormal phase ", func() {
		It("test cluster Failed/Abnormal phase", func() {
			By("create cluster related resources")
			createClusterDef()
			createAppversion()
			createCluster()

			By("watch normal event")
			event := &corev1.Event{
				Count:   1,
				Type:    corev1.EventTypeNormal,
				Message: "create pod failed because the pvc is deleting",
			}
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())

			By("watch warning event from StatefulSet, but mismatch condition ")
			// create statefulSet for replicasets component
			componentName := "replicasets"
			stsName := initComponentStatefulSet(componentName)
			stsInvolvedObject := corev1.ObjectReference{
				Name:      stsName,
				Kind:      StatefulSetKind,
				Namespace: "default",
			}
			event.InvolvedObject = stsInvolvedObject
			event.Type = corev1.EventTypeWarning
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())

			By("watch warning event from StatefulSet and component type is Stateful")
			event.Count = 3
			event.FirstTimestamp = metav1.Time{Time: time.Now()}
			event.LastTimestamp = metav1.Time{Time: time.Now().Add(31 * time.Second)}
			handleAndCheckComponentStatus(componentName, event, dbaasv1alpha1.FailedPhase, false)

			By("watch warning event from Pod and component type is Consensus")
			// create statefulSet for consensus component
			componentName = "consensus"
			stsName = initComponentStatefulSet(componentName)
			// create a failed pod
			podName := stsName + "-0"
			createStsPod(podName, "", componentName)
			setInvolvedObject(event, PodKind, podName)
			handleAndCheckComponentStatus(componentName, event, dbaasv1alpha1.FailedPhase, false)

			By("test merge pod event message")
			event.Message = "0/1 nodes can scheduled, cpu insufficient"
			handleAndCheckComponentStatus(componentName, event, dbaasv1alpha1.FailedPhase, false)

			By("test Abnormal phase for consensus component")
			setInvolvedObject(event, StatefulSetKind, stsName)
			podName1 := stsName + "-1"
			createStsPod(podName1, "leader", componentName)
			handleAndCheckComponentStatus(componentName, event, dbaasv1alpha1.AbnormalPhase, false)

			By("watch warning event from Deployment and component type is Stateless")
			deploymentName := "nginx-deploy"
			componentName = "nginx"
			setInvolvedObject(event, DeploymentKind, deploymentName)
			createDeployment(componentName, deploymentName)
			handleAndCheckComponentStatus(componentName, event, dbaasv1alpha1.FailedPhase, true)
		})
	})

})
