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
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/test/testdata"
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

	Context("setEnableAllLogs test", func() {
		var cluster *dbaasv1alpha1.Cluster
		var clusterDef *dbaasv1alpha1.ClusterDefinition
		var err error
		BeforeEach(func() {
			cluster, err = testdata.GetResourceFromTestData[dbaasv1alpha1.Cluster]("cli_testdata/mysql_logconfigs.yaml")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cluster.Spec.Components[0].EnabledLogs).Should(BeNil())
			clusterDef, err = testdata.GetResourceFromTestData[dbaasv1alpha1.ClusterDefinition]("cli_testdata/mysql_logconfigs_cd.yaml")
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("set all logs type enable", func() {
			setEnableAllLogs(cluster, clusterDef)
			Expect(len(cluster.Spec.Components[0].EnabledLogs)).Should(Equal(2))
		})
	})

	Context("multipleSourceComponent test", func() {
		defer GinkgoRecover()
		streams := genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stdout,
		}
		It("target file in website", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("OK"))
			}))
			defer ts.Close()
			fileURL := ts.URL + "/docs/file"
			bytes, err := MultipleSourceComponents(fileURL, streams.In)
			Expect(bytes).Should(Equal([]byte("OK")))
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("file doesn't exist", func() {
			fileName := "no-existing-file"
			bytes, err := MultipleSourceComponents(fileName, streams.In)
			Expect(bytes).Should(BeNil())
			Expect(err).Should(HaveOccurred())
		})
	})

	It("build cluster component", func() {
		viper.Set("CLUSTER_DEFAULT_STORAGE_SIZE", "10Gi")
		viper.Set("CLUSTER_DEFAULT_REPLICAS", 1)
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		comps, err := buildClusterComp(dynamic, testing.ClusterDefName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(comps).ShouldNot(BeNil())
		Expect(len(comps)).Should(Equal(2))
		Expect(comps[0]["volumeClaimTemplates"]).ShouldNot(BeNil())
		Expect(comps[0]["replicas"]).Should(BeEquivalentTo(1))
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
