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
	"bytes"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Expose", func() {
	const (
		namespace   = "test"
		clusterName = "test"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		cluster = testing.FakeCluster(clusterName, namespace)
		pods    = testing.FakePods(3, namespace, clusterName)
	)
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				mapping := map[string]*http.Response{
					"/api/v1/nodes/" + testing.NodeName:   httpResp(testing.FakeNode()),
					urlPrefix + "/services":               httpResp(&corev1.ServiceList{}),
					urlPrefix + "/events":                 httpResp(&corev1.EventList{}),
					urlPrefix + "/persistentvolumeclaims": httpResp(&corev1.PersistentVolumeClaimList{}),
					urlPrefix + "/pods":                   httpResp(pods),
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

	It("describe", func() {
		cmd := NewDescribeCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("complete", func() {
		o := newOptions(tf, streams)
		Expect(o.complete(nil)).Should(HaveOccurred())
		Expect(o.complete([]string{clusterName})).Should(Succeed())
		Expect(o.names).Should(Equal([]string{clusterName}))
		Expect(o.client).ShouldNot(BeNil())
		Expect(o.dynamic).ShouldNot(BeNil())
		Expect(o.namespace).Should(Equal(namespace))
	})

	It("run", func() {
		o := newOptions(tf, streams)
		Expect(o.complete([]string{clusterName})).Should(Succeed())
		Expect(o.run()).Should(Succeed())
	})

	It("showEvents", func() {
		out := &bytes.Buffer{}
		showEvents(testing.FakeEvents(), "test-cluster", namespace, out)
		strs := strings.Split(out.String(), "\n")

		// sorted
		firstEvent := strs[3]
		secondEvent := strs[4]
		Expect(strings.Compare(firstEvent, secondEvent) < 0).Should(BeTrue())
	})
})
