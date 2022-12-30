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
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

var (
	StatelessComponentName = "nginx"
)

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
    type: nginx
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
