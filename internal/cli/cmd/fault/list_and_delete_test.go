/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package fault

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Chaos resources list and delete", func() {
	var (
		tf           *cmdtesting.TestFactory
		streams      genericiooptions.IOStreams
		namespace    = "test"
		podChaosName = "testPodChaos"
		podChaos     = testing.FakePodChaos(podChaosName, namespace)
	)

	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}
		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: Group, Version: Version},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/apis/" + GroupVersion + "/namespaces/" + namespace
				mapping := map[string]*http.Response{
					urlPrefix + "/podchaos/" + podChaos.Name: httpResp(podChaos),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		_ = v1alpha1.AddToScheme(scheme.Scheme)
		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(scheme.Scheme, podChaos)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("test list and delete chaos resources", func() {
		It("test fault list", func() {
			args := []string{"podchaoses"}
			o := &ListAndDeleteOptions{Factory: tf, IOStreams: streams}
			Expect(o.Complete(args)).Should(Succeed())
			Expect(o.RunList()).Should(Succeed())
		})

		It("test fault delete", func() {
			args := []string{"podchaoses"}
			o := &ListAndDeleteOptions{Factory: tf, IOStreams: streams}
			Expect(o.Complete(args)).Should(Succeed())
			Expect(o.RunDelete()).Should(Succeed())
		})
	})
})
