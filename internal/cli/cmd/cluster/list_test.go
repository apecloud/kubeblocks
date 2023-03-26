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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("list", func() {
	var (
		streams genericclioptions.IOStreams
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
	)

	const (
		namespace   = "test"
		clusterName = "test"
	)

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)

		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		cluster := testing.FakeCluster(clusterName, namespace)
		pods := testing.FakePods(3, namespace, clusterName)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				return map[string]*http.Response{
					"/namespaces/" + namespace + "/clusters":      httpResp(&appsv1alpha1.ClusterList{Items: []appsv1alpha1.Cluster{*cluster}}),
					"/namespaces/" + namespace + "/clusters/test": httpResp(cluster),
					"/namespaces/" + namespace + "/secrets":       httpResp(testing.FakeSecrets(namespace, clusterName)),
					"/api/v1/nodes/" + testing.NodeName:           httpResp(testing.FakeNode()),
					urlPrefix + "/services":                       httpResp(&corev1.ServiceList{}),
					urlPrefix + "/secrets":                        httpResp(testing.FakeSecrets(namespace, clusterName)),
					urlPrefix + "/pods":                           httpResp(pods),
					urlPrefix + "/events":                         httpResp(testing.FakeEvents()),
				}[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("list", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.ClusterDefName))
	})

	It("list instances", func() {
		cmd := NewListInstancesCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.NodeName))
	})

	It("list components", func() {
		cmd := NewListComponentsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.ComponentName))
	})

	It("list events", func() {
		cmd := NewListEventsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(len(strings.Split(out.String(), "\n")) > 1).Should(BeTrue())
	})

	It("output wide", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		Expect(cmd.Flags().Set("output", "wide")).Should(Succeed())
		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.ClusterVersionName))
	})

	It("output wide without args", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		Expect(cmd.Flags().Set("output", "wide")).Should(Succeed())
		cmd.Run(cmd, []string{})
		Expect(out.String()).Should(ContainSubstring(testing.ClusterVersionName))
	})
})
