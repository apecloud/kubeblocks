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

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

var (
	StatelessComponentName = "nginx"
	StatelessComponentType = "proxy"
)

// CreateStatelessCluster creates a cluster with a component of Stateless type for testing.
func CreateStatelessCluster(testCtx testutil.TestContext, clusterDefName, clusterVersionName, clusterName string) *dbaasv1alpha1.Cluster {
	clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
  labels:
    clusterversion.kubeblocks.io/name: %s
    clusterdefinition.kubeblocks.io/name: %s
  name: %s
  namespace: default
spec:
  clusterVersionRef: %s
  clusterDefinitionRef: %s
  components:
  - name: nginx
    type: proxy
    monitor: false
  terminationPolicy: WipeOut`, clusterVersionName, clusterDefName, clusterName, clusterVersionName, clusterDefName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), cluster)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return cluster
}

// MockStatelessComponentDeploy mocks a deployment workload of the stateless component.
func MockStatelessComponentDeploy(testCtx testutil.TestContext, clusterName string) *appsv1.Deployment {
	deployName := clusterName + "-" + StatelessComponentName
	deploymentYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.nginx-8-cluster-definition
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  minReadySeconds: 10
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/component-name: %s
      app.kubernetes.io/instance: %s
      app.kubernetes.io/name: state.nginx-8-cluster-definition
      kubeblocks.io/test: test
      app.kubernetes.io/managed-by: kubeblocks
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: %s
        app.kubernetes.io/instance: %s
        app.kubernetes.io/name: state.nginx-8-cluster-definition
        kubeblocks.io/test: test
        app.kubernetes.io/managed-by: kubeblocks
    spec:
      containers:
      - image: nginx:latest
        imagePullPolicy: IfNotPresent
        name: nginx
        ports:
        - containerPort: 80
          protocol: TCP
        resources:
          limits:
            cpu: 200m
            memory: 200Mi
`, StatelessComponentName, clusterName, deployName, StatelessComponentName, clusterName, StatelessComponentName, clusterName)
	deploy := &appsv1.Deployment{}
	gomega.Expect(yaml.Unmarshal([]byte(deploymentYaml), deploy)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), deploy)).Should(gomega.Succeed())
	// wait until deployment created
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: deployName, Namespace: testCtx.DefaultNamespace}, &appsv1.Deployment{})
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return deploy
}

// MockStatelessPod mocks the pods of the deployment workload.
func MockStatelessPod(testCtx testutil.TestContext, clusterName, componentName, podName string) *corev1.Pod {
	podYaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: default
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.nginx-8-cluster-definition
    app.kubernetes.io/managed-by: kubeblocks
spec:
  containers:
  - image: nginx:latest
    imagePullPolicy: IfNotPresent
    name: nginx
    resources:
      limits:
        cpu: 300m
        memory: 470Mi
      requests:
        cpu: 280m
        memory: 390Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-2nth8
      readOnly: true
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  nodeName: minikube
  preemptionPolicy: PreemptLowerPriority
  priority: 0
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
    tolerationSeconds: 300
  volumes:
  - name: kube-api-access-2nth8
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          expirationSeconds: 3607
          path: token
      - configMap:
          items:
          - key: ca.crt
            path: ca.crt
          name: kube-root-ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace`, podName, componentName, clusterName)
	pod := &corev1.Pod{}
	gomega.Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), pod)).Should(gomega.Succeed())
	// wait until deployment created
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: podName,
			Namespace: testCtx.DefaultNamespace}, &corev1.Pod{})).Should(gomega.Succeed())
	}, timeout, interval).Should(gomega.Succeed())
	return pod
}
