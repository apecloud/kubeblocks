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
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("Deployment Controller", func() {
	var (
		randomStr   = testCtx.GetRandomStr()
		timeout     = time.Second * 20
		interval    = time.Second
		clusterName = "wesql-stateless-" + randomStr
		deployName  = "wesql-nginx-" + randomStr
		namespace   = "default"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	createCluster := func() *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
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
  - name: nginx
    type: nginx
    monitor: false
  terminationPolicy: WipeOut
status:
  clusterDefGeneration: 2
  components:
    nginx:
      phase: Running
  observedGeneration: 2
  operations:
    horizontalScalable:
    - name: nginx
    restartable:
    - nginx
    verticalScalable:
    - nginx
  phase: Running`, clusterName)
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

	createDeployment := func() *appsv1.Deployment {
		deploymentYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component-name: nginx
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  minReadySeconds: 10
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/component-name: nginx
      app.kubernetes.io/instance: %s
      app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: nginx
        app.kubernetes.io/instance: %s
        app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    spec:
      containers:
      - image: nginx:latest
        imagePullPolicy: IfNotPresent
        name: nginx
        ports:
        - containerPort: 80
          protocol: TCP
`, clusterName, deployName, clusterName, clusterName)
		deploy := &appsv1.Deployment{}
		Expect(yaml.Unmarshal([]byte(deploymentYaml), deploy)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), deploy)).Should(Succeed())
		// wait until deployment created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: deployName, Namespace: namespace}, &appsv1.Deployment{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return deploy
	}

	Context("test controller", func() {
		It("", func() {
			cluster := createCluster()
			_ = createDeployment()

			By("patch cluster to Updating")
			componentName := "nginx"
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
			cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{
				componentName: {
					Phase: dbaasv1alpha1.UpdatingPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())

			By("mock deployment is ready")
			newDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: deployName, Namespace: namespace}, newDeployment)).Should(Succeed())
			deployPatch := client.MergeFrom(newDeployment.DeepCopy())
			newDeployment.Status.ObservedGeneration = 1
			newDeployment.Status.AvailableReplicas = 3
			newDeployment.Status.ReadyReplicas = 3
			newDeployment.Status.Replicas = 3
			Expect(k8sClient.Status().Patch(context.Background(), newDeployment, deployPatch)).Should(Succeed())

			By("waiting the component is Running")
			Eventually(func() bool {
				cluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
				return cluster.Status.Components[componentName].Phase == dbaasv1alpha1.RunningPhase
			}, timeout, interval).Should(BeTrue())
		})
	})
})
