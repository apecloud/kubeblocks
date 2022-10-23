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

var _ = Describe("clusterDefinition webhook", func() {
	var (
		clusterDefinitionName = "clusterdefinition-webhook-mysql-definition"
	)
	Context("When clusterDefinition create and update", func() {
		It("Should webhook validate passed", func() {

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)
				return err == nil
			}, 10, 1).Should(BeTrue())

			By("By validating spec.cluster is null")
			tmpClusterStrategies := clusterDef.Spec.Cluster
			clusterDef.Spec.Cluster = nil
			Expect(k8sClient.Update(ctx, clusterDef)).Should(Succeed())
			clusterDef.Spec.Cluster = tmpClusterStrategies

			By("By updating a clusterDefinition")
			// validate spec.cluster.strategies.create?.order and spec.components[?].typeName is consistent, including component typeName and length
			createOrder := clusterDef.Spec.Cluster.Strategies.Create.Order
			createOrder[0] = "replicaset"
			Expect(k8sClient.Update(ctx, clusterDef)).ShouldNot(Succeed())
			// restore
			createOrder[0] = "replicaSets"

			By("By testing spec.cluster.strategies.create.order is consistent with spec.components[?].typeName")
			clusterDef.Spec.Cluster.Strategies.Create.Order = []string{"replicaSets", "proxy", "proxy_test"}
			Expect(k8sClient.Update(ctx, clusterDef)).ShouldNot(Succeed())
			// restore
			clusterDef.Spec.Cluster.Strategies.Create.Order = createOrder

			// validate spec.components[?].roleGroups and .strategies.create.order is consistent, including roleGroup name and length
			roleGroups := clusterDef.Spec.Components[0].Strategies.Create.Order
			roleGroups[0] = "primary_test"
			Expect(k8sClient.Update(ctx, clusterDef)).ShouldNot(Succeed())
			// restore
			roleGroups[0] = "primary"

			By("By testing spec.components[?].strategies.create.order is consistent with spec.components[?].roleGroups")
			clusterDef.Spec.Components[0].Strategies.Create.Order = []string{"primary", "follower", "candidate"}
			Expect(k8sClient.Update(ctx, clusterDef)).ShouldNot(Succeed())
			// restore
			clusterDef.Spec.Components[0].Strategies.Create.Order = []string{"primary", "follower"}

			By("By testing spec.roleGroupTemplates[?].typeName is consistent with spec.components[?].roleGroups ")
			// validate spec.roleGroupTemplates
			clusterDef.Spec.RoleGroupTemplates[0].TypeName = "primary_test"
			Expect(k8sClient.Update(ctx, clusterDef)).ShouldNot(Succeed())

		})
	})
})

// createTestClusterDefinitionObj  other webhook_test called this function, carefully for modifying the function
func createTestClusterDefinitionObj(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     %s
spec:
  type: state.mysql-8
  cluster:
    strategies:
      create:
        order: [replicaSets,proxy]
  components:
  - typeName: replicaSets
    roleGroups:
    - primary
    - follower
    strategies:
      create:
        order: [primary,follower]
  - typeName: proxy
   
  roleGroupTemplates:
  - typeName: primary
    defaultReplicas: 1
  - typeName: follower
    defaultReplicas: 2`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}
