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

package troubleshoot

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Collect Test", func() {
	var (
		timeOut     = 10 * time.Second
		namespace   = "test"
		clusterName = "test"
		streams     genericclioptions.IOStreams
		tf          *cmdtesting.TestFactory
		cluster     = testing.FakeCluster(clusterName, namespace)
		pods        = testing.FakePods(3, namespace, clusterName)
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.Group, Version: types.Version},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" {
					return nil, nil
				}
				urlPrefix := "/api/v1/namespaces/" + namespace
				mapping := map[string]*http.Response{
					"/api/v1/nodes/" + testing.NodeName: httpResp(testing.FakeNode()),
					urlPrefix + "/services":             httpResp(&corev1.ServiceList{}),
					urlPrefix + "/events":               httpResp(&corev1.EventList{}),
					urlPrefix + "/pods":                 httpResp(pods),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("parseTimeFlags Test", func() {
		sinceStr := "5m"
		sinceTimeStr := "2023-01-09T15:18:46+08:00"
		Expect(parseTimeFlags(sinceStr, sinceTimeStr, []*troubleshootv1beta2.Collect{})).Should(HaveOccurred())
		Expect(parseTimeFlags("", sinceTimeStr, []*troubleshootv1beta2.Collect{})).Should(Succeed())
		Expect(parseTimeFlags(sinceStr, "", []*troubleshootv1beta2.Collect{})).Should(Succeed())
	})
	It("collectRemoteData Test", func() {
		Eventually(func(g Gomega) {
			progressCh := make(chan interface{})
			go func() {
				for {
					g.Expect(<-progressCh).NotTo(BeNil())
				}
			}()
			p := &preflightOptions{
				factory:        tf,
				IOStreams:      streams,
				PreflightFlags: preflight.NewPreflightFlags(),
			}
			collectResult, err := collectRemoteData(&troubleshootv1beta2.HostPreflight{}, progressCh, *p)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(collectResult).NotTo(BeNil())
		}).WithTimeout(timeOut).Should(Succeed())
	})

	It("collectHostData Test", func() {
		hostByte := `
apiVersion: troubleshoot.sh/v1beta2
kind: HostPreflight
metadata:
  name: cpu
spec:
  collectors:
    - cpu: {}
  analyzers:
    - cpu:
        outcomes:
          - fail:
              when: "physical < 4"
              message: At least 4 physical CPU cores are required
          - fail:
              when: "logical < 8"
              message: At least 8 CPU cores are required
          - warn:
              when: "count < 16"
              message: At least 16 CPU cores preferred
          - pass:
              message: This server has sufficient CPU cores.`
		hostSpec := new(troubleshootv1beta2.HostPreflight)
		Eventually(func(g Gomega) {
			g.Expect(yaml.Unmarshal([]byte(hostByte), hostSpec)).Should(Succeed())
			progressCh := make(chan interface{})
			go func() {
				for {
					g.Expect(<-progressCh).NotTo(BeNil())
				}
			}()
			results, err := collectHostData(hostSpec, progressCh)
			g.Expect(err).NotTo(HaveOccurred())
			_, ok := (*results).(preflight.HostCollectResult)
			g.Expect(ok).Should(BeTrue())
		}).WithTimeout(timeOut).Should(Succeed())
	})
})
