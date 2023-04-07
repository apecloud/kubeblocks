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

package class

import (
	"bytes"
	"net/http"

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
)

var _ = Describe("list", func() {
	var (
		cd      *appsv1alpha1.ClusterDefinition
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)

	BeforeEach(func() {
		cd = testing.FakeClusterDef()

		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)

		_ = corev1.AddToScheme(scheme.Scheme)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: "core", Version: "v1"},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				return map[string]*http.Response{
					"/api/v1/configmaps": httpResp(testing.FakeComponentClassDef(cd, classDef)),
				}[req.URL.Path], nil
			}),
		}
		tf.Client = tf.UnstructuredClient
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should succeed", func() {
		cmd := NewListCommand(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		cmd.Run(cmd, []string{"--cluster-definition", cd.GetName()})
		Expect(out.String()).To(ContainSubstring("general-1c1g"))
		Expect(out.String()).To(ContainSubstring(testing.ComponentDefName))
		Expect(out.String()).To(ContainSubstring(generalClassFamily.Name))
		Expect(out.String()).To(ContainSubstring(memoryOptimizedClassFamily.Name))
	})
})
