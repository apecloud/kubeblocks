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

package cluster

import (
	"encoding/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func generateComponents(component dbaasv1alpha1.ClusterComponent, count int) []map[string]interface{} {
	var componentVals []map[string]interface{}
	byteVal, err := json.Marshal(component)
	Expect(err == nil).Should(BeTrue())
	for i := 0; i < count; i++ {
		var componentVal map[string]interface{}
		err = json.Unmarshal(byteVal, &componentVal)
		Expect(err == nil).Should(BeTrue())
		componentVals = append(componentVals, componentVal)
	}
	Expect(len(componentVals)).To(Equal(count))
	return componentVals
}

func expectEqual(expectComponents []map[string]interface{}, actualComponents []map[string]interface{}) {
	expectByte, _ := json.Marshal(expectComponents)
	actualByte, _ := json.Marshal(actualComponents)
	Expect(string(actualByte)).To(Equal(string(expectByte)))
}

var _ = Describe("create", func() {
	Context("setMonitor", func() {
		var actualComponents []map[string]interface{}
		var expectComponents []map[string]interface{}

		BeforeEach(func() {
			var component dbaasv1alpha1.ClusterComponent
			component.Monitor = true
			actualComponents = generateComponents(component, 3)
			component.Monitor = false
			expectComponents = generateComponents(component, 3)
		})

		It("set monitor param to false", func() {
			setMonitor(false, actualComponents)
			expectEqual(expectComponents, actualComponents)
		})

		It("set monitor param to true", func() {
			setMonitor(true, actualComponents)
			expectEqual(actualComponents, actualComponents)
		})
	})

	Context("setEnableAllLogs Test", func() {
		cluster := &dbaasv1alpha1.Cluster{}
		clusterByte := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql
spec:
  appVersionRef: app-version-consensus
  clusterDefinitionRef: cluster-definition-consensus
  components:
    - name: wesql-test
      type: replicasets
`
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		clusterDefByte := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-consensus
spec:
  type: state.mysql-8
  components:
    - typeName: replicasets
      componentType: Consensus
      logConfigs:
        - name: error
          filePathPattern: /log/mysql/mysqld.err
        - name: slow
          filePathPattern: /log/mysql/*slow.log
      podSpec:
        containers:
          - name: mysql
            imagePullPolicy: IfNotPresent`
		_ = yaml.Unmarshal([]byte(clusterDefByte), clusterDef)
		_ = yaml.Unmarshal([]byte(clusterByte), cluster)
		setEnableAllLogs(cluster, clusterDef)
		Expect(len(cluster.Spec.Components[0].EnabledLogs)).Should(Equal(2))
	})
	Context("multipleSourceComponent Test", func() {
		defer GinkgoRecover()
		fileName := "https://kubernetes.io/docs/tasks/debug/"
		streams := genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stdout,
		}
		bytes, err := multipleSourceComponents(fileName, streams)
		Expect(bytes).ShouldNot(BeNil())
		Expect(err).ShouldNot(HaveOccurred())
		// corner case for no existing local file
		fileName = "no-existing-file"
		bytes, err = multipleSourceComponents(fileName, streams)
		Expect(bytes).Should(BeNil())
		Expect(err).ShouldNot(BeNil())
	})
})
