/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package kubeblocks

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
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
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("Preflight API Test", func() {
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

	It("complete and validate test", func() {
		p := &PreflightOptions{
			factory:        tf,
			IOStreams:      streams,
			PreflightFlags: preflight.NewPreflightFlags(),
		}
		Expect(p.complete(tf, nil)).Should(HaveOccurred())
		Expect(p.validate()).Should(HaveOccurred())
		Expect(p.complete(tf, []string{"file1", "file2"})).Should(Succeed())
		Expect(len(p.checkFileList)).Should(Equal(2))
		Expect(p.validate()).Should(Succeed())
	})

	It("run test", func() {
		p := &PreflightOptions{
			factory:        tf,
			IOStreams:      streams,
			PreflightFlags: preflight.NewPreflightFlags(),
		}
		*p.Interactive = false
		*p.Format = "yaml"
		p.checkFileList = []string{"../../testing/testdata/hostpreflight.yaml"}
		By("non-interactive mode, and expect success")
		Eventually(func(g Gomega) {
			err := p.run()
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())
		By("non-interactive mode, and expect error")
		p.checkFileList = []string{"../../testing/testdata/hostpreflight_nil.yaml"}
		Eventually(func(g Gomega) {
			err := p.run()
			g.Expect(err).To(HaveOccurred())
		}).Should(Succeed())
	})

	It("LoadVendorCheckYaml test, and expect fail", func() {
		res, err := LoadVendorCheckYaml(util.UnknownProvider)
		Expect(err).Should(HaveOccurred())
		Expect(len(res)).Should(Equal(0))
	})

	It("LoadVendorCheckYaml test, and expect success", func() {
		res, err := LoadVendorCheckYaml(util.EKSProvider)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(res)).Should(Equal(2))
	})
	It("newPreflightPath test, and expect success", func() {
		res := newPreflightPath("test")
		Expect(res).Should(Equal("data/test_preflight.yaml"))
	})
	It("newHostPreflightPath test, and expect success", func() {
		res := newHostPreflightPath("test")
		Expect(res).Should(Equal("data/test_hostpreflight.yaml"))
	})
})
