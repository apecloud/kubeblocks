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
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
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
		var cluster *dbaasv1alpha1.Cluster
		var clusterDef *dbaasv1alpha1.ClusterDefinition
		BeforeEach(func() {
			cluster = testing.FakeCluster("log", "test")
			clusterDef = testing.FakeClusterDef()
			Expect(cluster.Spec.Components[0].EnabledLogs).Should(BeNil())
		})
		It("no logConfigs in ClusterDef", func() {
			setEnableAllLogs(cluster, clusterDef)
			Expect(len(cluster.Spec.Components[0].EnabledLogs)).Should(Equal(0))
		})
		It("set logConfigs in ClusterDef", func() {
			clusterDef.Spec.Components[0].LogConfigs = []dbaasv1alpha1.LogConfig{
				{
					Name:            "error",
					FilePathPattern: "/log/mysql/mysqld.err",
				},
				{
					Name:            "slow",
					FilePathPattern: "/log/mysql/*slow.log",
				},
			}
			setEnableAllLogs(cluster, clusterDef)
			Expect(cluster.Spec.Components[0].EnabledLogs).Should(Equal([]string{"error", "slow"}))
		})
	})

	Context("multipleSourceComponent test", func() {
		defer GinkgoRecover()
		streams := genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stdout,
		}
		It("target file stored in website", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte("OK"))
				Expect(err).ShouldNot(HaveOccurred())
			}))
			defer ts.Close()
			fileURL := ts.URL + "/docs/file"
			bytes, err := MultipleSourceComponents(fileURL, streams.In)
			Expect(bytes).Should(Equal([]byte("OK")))
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("target file doesn't exist", func() {
			fileName := "no-existing-file"
			bytes, err := MultipleSourceComponents(fileName, streams.In)
			Expect(bytes).Should(BeNil())
			Expect(err).Should(HaveOccurred())
		})
	})

	It("build default cluster component without environment", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		comps, err := buildClusterComp(dynamic, testing.ClusterDefName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(comps).ShouldNot(BeNil())
		Expect(len(comps)).Should(Equal(2))

		clusterComp := &dbaasv1alpha1.ClusterComponent{}
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(comps[0], clusterComp)
		Expect(getResource(clusterComp.VolumeClaimTemplates[0].Spec.Resources, corev1.ResourceStorage)).Should(Equal("10Gi"))
		Expect(*clusterComp.Replicas).Should(BeEquivalentTo(2))

		resources := clusterComp.Resources
		Expect(resources).ShouldNot(BeNil())
		Expect(getResource(resources, corev1.ResourceCPU)).Should(Equal("1"))
		Expect(getResource(resources, corev1.ResourceMemory)).Should(Equal("1Gi"))
	})

	It("build default cluster component with environment", func() {
		viper.Set("CLUSTER_DEFAULT_STORAGE_SIZE", "5Gi")
		viper.Set("CLUSTER_DEFAULT_REPLICAS", 1)
		viper.Set("CLUSTER_DEFAULT_CPU", "2000m")
		viper.Set("CLUSTER_DEFAULT_MEMORY", "2Gi")
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		comps, err := buildClusterComp(dynamic, testing.ClusterDefName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(comps).ShouldNot(BeNil())
		Expect(len(comps)).Should(Equal(2))

		clusterComp := &dbaasv1alpha1.ClusterComponent{}
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(comps[0], clusterComp)
		Expect(getResource(clusterComp.VolumeClaimTemplates[0].Spec.Resources, corev1.ResourceStorage)).Should(Equal("5Gi"))
		Expect(*clusterComp.Replicas).Should(BeEquivalentTo(1))
		resources := clusterComp.Resources
		Expect(resources).ShouldNot(BeNil())
		Expect(resources.Requests[corev1.ResourceCPU].ToUnstructured()).Should(Equal("2"))
		Expect(resources.Requests[corev1.ResourceMemory].ToUnstructured()).Should(Equal("2Gi"))
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
