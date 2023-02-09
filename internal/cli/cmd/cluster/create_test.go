/*
Copyright ApeCloud, Inc.

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
	"os"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/json"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

func generateComponents(component dbaasv1alpha1.ClusterComponent, count int) []map[string]interface{} {
	var componentVals []map[string]interface{}
	byteVal, err := json.Marshal(component)
	Expect(err).ShouldNot(HaveOccurred())
	for i := 0; i < count; i++ {
		var componentVal map[string]interface{}
		err = json.Unmarshal(byteVal, &componentVal)
		Expect(err).ShouldNot(HaveOccurred())
		componentVals = append(componentVals, componentVal)
	}
	Expect(len(componentVals)).To(Equal(count))
	return componentVals
}

func getResource(res corev1.ResourceRequirements, name corev1.ResourceName) interface{} {
	return res.Requests[name].ToUnstructured()
}

var _ = Describe("create", func() {
	Context("setMonitor", func() {
		var components []map[string]interface{}
		BeforeEach(func() {
			var component dbaasv1alpha1.ClusterComponent
			component.Monitor = true
			components = generateComponents(component, 3)
		})

		It("set monitor param to false", func() {
			setMonitor(false, components)
			for _, c := range components {
				Expect(c[monitorKey]).ShouldNot(BeTrue())
			}
		})

		It("set monitor param to true", func() {
			setMonitor(true, components)
			for _, c := range components {
				Expect(c[monitorKey]).Should(BeTrue())
			}
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
  clusterVersionRef: cluster-version-consensus
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
  type: state.mysql
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
		bytes, err := MultipleSourceComponents(fileName, streams.In)
		Expect(bytes).ShouldNot(BeNil())
		Expect(err).ShouldNot(HaveOccurred())
		// corner case for no existing local file
		fileName = "no-existing-file"
		bytes, err = MultipleSourceComponents(fileName, streams.In)
		Expect(bytes).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})

	checkComponent := func(comps []map[string]interface{}, storage string, replicas int32, cpu string, memory string) {
		Expect(comps).ShouldNot(BeNil())
		Expect(len(comps)).Should(Equal(2))

		comp := &dbaasv1alpha1.ClusterComponent{}
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(comps[0], comp)
		Expect(getResource(comp.VolumeClaimTemplates[0].Spec.Resources, corev1.ResourceStorage)).Should(Equal(storage))
		Expect(*comp.Replicas).Should(BeEquivalentTo(replicas))

		resources := comp.Resources
		Expect(resources).ShouldNot(BeNil())
		Expect(getResource(resources, corev1.ResourceCPU)).Should(Equal(cpu))
		Expect(getResource(resources, corev1.ResourceMemory)).Should(Equal(memory))
	}

	It("build default cluster component without environment", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		comps, err := buildClusterComp(cd, nil)
		Expect(err).ShouldNot(HaveOccurred())
		checkComponent(comps, "10Gi", 1, "1", "1Gi")
	})

	It("build default cluster component with environment", func() {
		viper.Set("CLUSTER_DEFAULT_STORAGE_SIZE", "5Gi")
		viper.Set("CLUSTER_DEFAULT_REPLICAS", 1)
		viper.Set("CLUSTER_DEFAULT_CPU", "2000m")
		viper.Set("CLUSTER_DEFAULT_MEMORY", "2Gi")
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		comps, err := buildClusterComp(cd, nil)
		Expect(err).ShouldNot(HaveOccurred())
		checkComponent(comps, "5Gi", 1, "2", "2Gi")
	})

	It("build cluster component with set values", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		setsMap := map[string]map[setKey]string{
			testing.ComponentType: {
				keyCPU:      "10",
				keyMemory:   "2Gi",
				keyStorage:  "10Gi",
				keyReplicas: "10",
			},
		}
		comps, err := buildClusterComp(cd, setsMap)
		Expect(err).Should(Succeed())
		checkComponent(comps, "10Gi", 10, "10", "2Gi")
	})

	It("build component and set values map", func() {
		mockCD := func(typeNames []string) *dbaasv1alpha1.ClusterDefinition {
			cd := &dbaasv1alpha1.ClusterDefinition{}
			var comps []dbaasv1alpha1.ClusterDefinitionComponent
			for _, n := range typeNames {
				comp := dbaasv1alpha1.ClusterDefinitionComponent{
					TypeName: n,
				}
				comps = append(comps, comp)
			}
			cd.Spec.Components = comps
			return cd
		}

		testCases := []struct {
			values    []string
			typeNames []string
			expected  map[string]map[setKey]string
		}{
			{
				nil,
				nil,
				map[string]map[setKey]string{},
			},
			{
				[]string{"cpu=1"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU: "1",
					},
				},
			},
			{
				[]string{"cpu=1,memory=2Gi,storage=10Gi"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU:     "1",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
			},
			// values with unknown set key that will be ignored
			{
				[]string{"cpu=1,memory=2Gi,storage=10Gi,t1,t1=v1"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU:     "1",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
			},
			// values with type
			{
				[]string{"type=comp,cpu=1,memory=2Gi,storage=10Gi,t1,t1=v1"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"comp": {
						keyType:    "comp",
						keyCPU:     "1",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
			},
			// set more than one time
			{
				[]string{"cpu=1,memory=2Gi", "storage=10Gi,cpu=2"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU:     "2",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
			},
			{
				[]string{"type=my-comp,cpu=1,memory=2Gi", "storage=10Gi,cpu=2"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyType:    "my-comp",
						keyCPU:     "2",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
			},
			{
				[]string{"type=comp1,cpu=1,memory=2Gi", "type=comp2,storage=10Gi,cpu=2"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"comp1": {
						keyType:   "comp1",
						keyCPU:    "1",
						keyMemory: "2Gi",
					},
					"comp2": {
						keyType:    "comp2",
						keyCPU:     "2",
						keyStorage: "10Gi",
					},
				},
			},
		}

		for _, t := range testCases {
			By(strings.Join(t.values, " "))
			res, err := buildCompSetsMap(t.values, mockCD(t.typeNames))
			Expect(err).Should(Succeed())
			Expect(reflect.DeepEqual(res, t.expected)).Should(BeTrue())
		}
	})

	It("build tolerations", func() {
		raw := []string{"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"}
		res := buildTolerations(raw)
		Expect(len(res)).Should(Equal(1))
	})

	It("generate random cluster name", func() {
		dynamic := testing.FakeDynamicClient()
		name, err := generateClusterName(dynamic, "")
		Expect(err).Should(Succeed())
		Expect(name).ShouldNot(BeEmpty())
	})
})
