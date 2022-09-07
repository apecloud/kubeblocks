/*
Copyright 2022.

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("opsDefinition webhook", func() {
	var (
		clusterDefinitionName = "opsdefinition-webhook-clusterdefinition"
		opsDefinitionName     = "opsdefinition-webhook-ospdefinition"
	)
	Context("When opsDefinition create and update", func() {
		It("Should webhook validate passed", func() {
			By("By testing create a new opsDefinition when clusterDefinition not exist")
			opsDefinition := createTestOpsDefinition(clusterDefinitionName, opsDefinitionName, UpgradeType)
			Expect(k8sClient.Create(ctx, opsDefinition)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())

			By("By  create a new opsDefinition")
			Expect(k8sClient.Create(ctx, opsDefinition)).Should(Succeed())

			By("By testing spec.Strategy.Components[*].type is valid")
			opsDefinition.Spec.Strategy.Components[0].Type = "replicaSets1"
			Expect(k8sClient.Update(ctx, opsDefinition)).ShouldNot(Succeed())

		})
	})
})

func createTestOpsDefinition(clusterDefinitionName, opsDefinitionName string, opsType OpsType) *OpsDefinition {
	opsDefYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: OpsDefinition
metadata:
  name: %s
  namespace: default
  labels:
    clusterdefinition.infracreate.com/name: %s
spec:
  clusterDefinitionRef: %s
  type: %s
`, opsDefinitionName, clusterDefinitionName, clusterDefinitionName, opsType)
	opsDefinition := &OpsDefinition{}
	_ = yaml.Unmarshal([]byte(opsDefYaml), opsDefinition)
	opsDefinition.Spec.Strategy = &Strategy{
		Components: []OpsDefComponent{
			{Type: "replicaSets"},
		},
	}
	return opsDefinition
}
