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
	"context"
	"fmt"
	chaosv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Chaos resources list and delete", func() {
	var (
		tf           *cmdtesting.TestFactory
		namespace    = "test"
		podChaosName = "testPodChaos"
		podchaos     = testing.FakePodchaos(podChaosName, namespace)
	)
	BeforeEach(func() {
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
					urlPrefix + "/podchaos":                  httpResp(podchaos),
					urlPrefix + "/podchaos/" + podchaos.Name: httpResp(podchaos),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		_ = chaosv1alpha1.AddToScheme(scheme.Scheme)
		tf.FakeDynamicClient = testing.FakeDynamicClient(podchaos)

		gvr := GetGVR(Group, Version, ResourcePodChaos)
		resourceList, err := tf.FakeDynamicClient.Resource(gvr).Namespace(namespace).Get(context.Background(), podChaosName, metav1.GetOptions{})
		if err != nil {
			fmt.Errorf("failed to list %s: %s", gvr, err)
		}
		fmt.Println("sfa")
		fmt.Println(resourceList.IsList())
	})

	AfterEach(func() {
		tf.Cleanup()
	})
	Context("test fault list", func() {
		It("fault list", func() {
			o := &ListAndDeleteOptions{Factory: tf}
			o.ResourceKinds = []string{"PodChaos"}
			Expect(o.RunList()).Should(Succeed())
		})
	})
})
