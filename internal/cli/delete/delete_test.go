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

package delete

import (
	"bytes"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Delete", func() {
	var (
		streams genericclioptions.IOStreams
		in      *bytes.Buffer
		tf      *cmdtesting.TestFactory
		o       *DeleteOptions
	)

	const (
		namespace   = "test"
		clusterName = "clusterName"
	)

	BeforeEach(func() {
		streams, in, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)

		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		cluster := testing.FakeCluster(clusterName, namespace)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				return httpResp(cluster), nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())
		o = NewDeleteOptions(tf, streams, types.ClusterGVR())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("validate", func() {
		o.Names = []string{"foo"}
		By("set force and GracePeriod")
		o.Force = true
		o.GracePeriod = 1
		o.Now = false
		Expect(o.validate()).Should(HaveOccurred())

		o.Force = true
		o.GracePeriod = 0
		o.Now = false
		Expect(o.validate()).Should(Succeed())

		By("set now and GracePeriod")
		o.Force = false
		o.Now = true
		o.GracePeriod = 1
		Expect(o.validate()).Should(HaveOccurred())

		o.Force = false
		o.Now = true
		o.GracePeriod = -1
		Expect(o.validate()).Should(Succeed())

		By("set force only")
		o.Force = true
		o.Now = false
		o.GracePeriod = -1
		Expect(o.validate()).Should(Succeed())

		By("set GracePeriod only")
		o.Force = false
		o.Now = false
		o.GracePeriod = 1
		Expect(o.validate()).Should(Succeed())

		o.Force = false
		o.GracePeriod = -1
		o.Now = false

		By("set name and label")
		o.Names = []string{"foo"}
		o.LabelSelector = "foo=bar"
		o.AllNamespaces = false
		Expect(o.validate()).Should(HaveOccurred())

		By("set name and all")
		o.Names = []string{"foo"}
		o.LabelSelector = ""
		o.AllNamespaces = true
		Expect(o.validate()).Should(HaveOccurred())

		By("set all and label")
		o.Names = nil
		o.AllNamespaces = true
		o.LabelSelector = "foo=bar"
		Expect(o.validate()).Should(Succeed())

		By("set name")
		o.Names = []string{"foo"}
		o.AllNamespaces = false
		o.LabelSelector = ""
		Expect(o.validate()).Should(Succeed())

		By("set nothing")
		o.Names = nil
		o.LabelSelector = ""
		Expect(o.validate()).Should(MatchError(MatchRegexp("no name was specified")))
	})

	It("complete", func() {
		o.Names = []string{"foo"}
		Expect(o.validate()).Should(Succeed())

		By("confirm")
		in.Reset()
		Expect(o.complete()).Should(HaveOccurred())
		in.Reset()
		_, _ = in.Write([]byte("bar\n"))
		Expect(o.complete()).Should(HaveOccurred())
		in.Reset()
		_, _ = in.Write([]byte("foo\n"))
		Expect(o.complete()).Should(Succeed())

		Expect(o.Result).ShouldNot(BeNil())
	})

	It("build a delete command", func() {
		cmd := &cobra.Command{
			Use:     "test-delete",
			Short:   "Test a delete command",
			Example: "Test command example",
			RunE: func(cmd *cobra.Command, args []string) error {
				o.Names = args
				return o.Run()
			},
		}
		o.AddFlags(cmd)

		Expect(cmd).ShouldNot(BeNil())

		By("do not use pre-delete hook")
		_, _ = in.Write([]byte(clusterName + "\n"))
		Expect(cmd.RunE(cmd, []string{clusterName})).Should(Succeed())

		By("set pre-delete hook")
		// block cluster deletion
		fakePreDeleteHook := func(object runtime.Object) error {
			if object.GetObjectKind().GroupVersionKind().Kind == appsv1alpha1.ClusterKind {
				return fmt.Errorf("fake pre-delete hook error")
			} else {
				return nil
			}
		}
		o.PreDeleteHook = fakePreDeleteHook
		_, _ = in.Write([]byte(clusterName + "\n"))
		Expect(cmd.RunE(cmd, []string{clusterName})).Should(HaveOccurred())
	})
})
