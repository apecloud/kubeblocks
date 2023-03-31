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
	"k8s.io/apimachinery/pkg/api/errors"
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

var _ = Describe("create", func() {
	var (
		o       *CreateOptions
		cd      *appsv1alpha1.ClusterDefinition
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)

	fillResources := func(o *CreateOptions, cpu string, memory string, storage []string) {
		o.CPU = cpu
		o.Memory = memory
		o.Storage = storage
	}

	BeforeEach(func() {
		cd = testing.FakeClusterDef()

		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)

		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}
		cms := testing.FakeComponentClassDef(cd, classDef)

		resources := map[string]runtime.Object{
			"/api/v1/configmaps": cms,
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: "core", Version: "v1"},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				if req.Method == "POST" {
					return httpResp(&corev1.ConfigMap{}), nil
				}
				resource, ok := resources[req.URL.Path]
				if !ok {
					return nil, errors.NewNotFound(schema.GroupResource{}, req.URL.Path)
				}
				return httpResp(resource), nil
			}),
		}
		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(&generalClassFamily, &memoryOptimizedClassFamily, cd)

		o = &CreateOptions{
			Factory:       tf,
			IOStreams:     streams,
			ClusterDefRef: cd.Name,
			ComponentType: testing.ComponentDefName,
		}
		Expect(o.complete(tf)).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should succeed to new command", func() {
		cmd := NewCreateCommand(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	Context("with resource arguments", func() {

		It("should fail if required arguments is missing", func() {
			o.ClassFamily = generalClassFamily.Name
			fillResources(o, "", "48Gi", nil)
			Expect(o.validate([]string{"general-12c48g"})).Should(HaveOccurred())
			fillResources(o, "12", "", nil)
			Expect(o.validate([]string{"general-12c48g"})).Should(HaveOccurred())
			fillResources(o, "12", "48g", nil)
			Expect(o.validate([]string{})).Should(HaveOccurred())
		})

		It("should succeed with required arguments", func() {
			o.ClassFamily = generalClassFamily.Name
			fillResources(o, "12", "48Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			Expect(o.validate([]string{"general-12c48g"})).ShouldNot(HaveOccurred())
			Expect(o.run()).ShouldNot(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring(o.ClassName))
		})

		It("should fail if class name is conflicted", func() {
			o.ClassName = "general-1c1g"
			fillResources(o, "1", "1Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			Expect(o.run()).Should(HaveOccurred())
		})
	})

	Context("with class definitions file", func() {
		It("should succeed", func() {
			o.File = testCustomClassDefsPath
			Expect(o.run()).ShouldNot(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring("custom-1c1g"))
			Expect(out.String()).Should(ContainSubstring("custom-200c400g"))
			// memory optimized classes
			Expect(out.String()).Should(ContainSubstring("custom-1c32g"))
			Expect(out.String()).Should(ContainSubstring("custom-2c64g"))
		})

	})

})
