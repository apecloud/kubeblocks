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

package accounts

import (
	"net/http"

	"github.com/dapr/components-contrib/bindings"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	channelutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

var _ = Describe("Grant Account Options", func() {
	const (
		namespace   = "test"
		clusterName = "apple"
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
					urlPrefix + "/pods":                       httpResp(pods),
					urlPrefix + "/pods/" + pods.Items[0].Name: httpResp(&pods.Items[0]),
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

	Context("new options", func() {
		It("new option", func() {
			for _, op := range []bindings.OperationKind{channelutil.GrantUserRoleOp, channelutil.RevokeUserRoleOp} {
				o := NewGrantOptions(tf, streams, op)
				Expect(o).ShouldNot(BeNil())
			}
			for _, op := range []bindings.OperationKind{channelutil.CreateUserOp, channelutil.DeleteUserOp, channelutil.DescribeUserOp, channelutil.ListUsersOp} {
				o := NewGrantOptions(tf, streams, op)
				Expect(o).Should(BeNil())
			}
		})

		It("validate options", func() {
			for _, op := range []bindings.OperationKind{channelutil.GrantUserRoleOp, channelutil.RevokeUserRoleOp} {
				o := NewGrantOptions(tf, streams, op)
				Expect(o).ShouldNot(BeNil())
				args := []string{}
				Expect(o.Validate(args)).Should(MatchError(errClusterNameorInstName))

				// add one element
				By("add one more args, should fail")
				args = []string{"foo"}
				Expect(o.Validate(args)).Should(MatchError(errMissingUserName))

				o.info.UserName = "foo"
				Expect(o.Validate(args)).Should(MatchError(errMissingRoleName))

				o.info.RoleName = "bar"
				Expect(o.Validate(args)).Should(MatchError(errInvalidRoleName))
				for _, r := range []string{"readonly", "readwrite", "superuser"} {
					o.info.RoleName = r
					Expect(o.Validate(args)).Should(Succeed())
				}
			}
		})

		It("complete option", func() {
			o := NewGrantOptions(tf, streams, channelutil.GrantUserRoleOp)
			Expect(o).ShouldNot(BeNil())
			o.PodName = pods.Items[0].Name
			o.ClusterName = clusterName
			o.info.UserName = "alice"
			o.info.RoleName = "readonly"
			Expect(o.Complete(tf)).Should(Succeed())

			Expect(o.Client).ShouldNot(BeNil())
			Expect(o.Dynamic).ShouldNot(BeNil())
			Expect(o.Namespace).Should(Equal(namespace))
			Expect(o.Pod).ShouldNot(BeNil())
			Expect(o.Pod.Name).Should(Equal(o.PodName))
			Expect(o.RequestMeta).ShouldNot(BeNil())
			Expect(o.RequestMeta).Should(HaveLen(2))
		})
	})
})
