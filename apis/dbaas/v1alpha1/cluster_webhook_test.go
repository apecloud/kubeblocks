/*
Copyright 2022 The KubeBlocks Authors

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

package v1alpha1

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("cluster webhook", func() {
	var (
		clusterName               = "cluster-webhook-mysql"
		clusterDefinitionName     = "cluster-webhook-mysql-definition"
		sencondeClusterDefinition = "cluster-webhook-mysql-definition2"
		appVersionName            = "cluster-webhook-mysql-appversion"
	)
	Context("When cluster create and update", func() {
		It("Should webhook validate passed", func() {
			By("By testing creating a new clusterDefinition when no appVersion and clusterDefinition")
			cluster, _ := createTestCluster(clusterDefinitionName, appVersionName, clusterName)
			Expect(k8sClient.Create(ctx, cluster)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())

			clusterDefSecond, _ := createTestClusterDefinitionObj(sencondeClusterDefinition)
			Expect(k8sClient.Create(ctx, clusterDefSecond)).Should(Succeed())

			// wait until ClusterDefinition created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)
				return err == nil
			}, 10, 1).Should(BeTrue())

			By("By creating a new appVersion")
			appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
			Expect(k8sClient.Create(ctx, appVersion)).Should(Succeed())
			// wait until AppVersion created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: appVersionName}, appVersion)
				return err == nil
			}, 10, 1).Should(BeTrue())

			By("By creating a new Cluster")
			cluster, _ = createTestCluster(clusterDefinitionName, appVersionName, clusterName)
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			By("By testing update spec.clusterDefinitionRef")
			cluster.Spec.ClusterDefRef = sencondeClusterDefinition
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())

			By("By testing spec.components[?].type not found in clusterDefinitionRef")
			cluster.Spec.Components[0].Type = "replicaSet"
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())
			// restore
			cluster.Spec.Components[0].Type = "replicaSets"

			By("By testing spec.components[?].name is duplicated")
			cluster.Spec.Components[0].Name = "proxy"
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())

		})
	})
})

func createTestCluster(clusterDefinitionName, appVersionName, clusterName string) (*Cluster, error) {
	clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  appVersionRef: %s
  components:
  - name: replicaSets
    type: replicaSets
    replicas: 1
  - name: proxy
    type: proxy
    replicas: 1
`, clusterName, clusterDefinitionName, appVersionName)
	cluster := &Cluster{}
	err := yaml.Unmarshal([]byte(clusterYaml), cluster)
	return cluster, err
}
