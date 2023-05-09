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

package sync2foxlake

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("create", func() {
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	const (
		name                      = "fake-task"
		namespace                 = "test"
		fakeClusterName           = "fake-cluster"
		fakeFoxLakeClusterName    = "fake-foxlake-cluster"
		fakeFoxLakeClusterDefName = "fake-foxlake-cluster-def"
	)

	BeforeEach(func() {
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		cluster := testing.FakeCluster(fakeClusterName, namespace)
		foxlakecluster := testing.FakeCluster(fakeFoxLakeClusterName, namespace)
		foxlakecluster.Spec.ClusterDefRef = fakeFoxLakeClusterDefName
		foxlakecd := testing.FakeClusterDef()
		foxlakecd.Name = fakeFoxLakeClusterDefName
		foxlakecd.Spec.ComponentDefs[0].CharacterType = "foxlake"
		pods := testing.FakePods(3, namespace, fakeClusterName)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}
		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				return map[string]*http.Response{
					urlPrefix + "/services": httpResp(testing.FakeServices()),
					urlPrefix + "/secrets":  httpResp(testing.FakeSecrets(namespace, fakeClusterName)),
					urlPrefix + "/pods":     httpResp(pods),
				}[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, foxlakecluster, foxlakecd, testing.FakeClusterDef(), testing.FakeClusterVersion())
		streams = genericclioptions.NewTestIOStreamsDiscard()

	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("validate", func() {
		o := &CreateSync2FoxLakeOptions{CreateOptions: create.CreateOptions{
			Factory:   tf,
			IOStreams: streams,
		}}
		o.Source = "user:123456@127.0.0.1:5432"
		o.Sink = fakeFoxLakeClusterName
		o.Args = []string{name}

		Expect(o.Complete()).ShouldNot(HaveOccurred())
		Expect(o.Validate()).ShouldNot(HaveOccurred())

		Expect(o.Name).To(Equal(name))

		Expect(o.SourceEndpointModel.EndpointType).Should(Equal(AddressEndpointType))
		Expect(o.SourceEndpointModel.Endpoint).Should(Equal("127.0.0.1:5432"))
		Expect(o.SourceEndpointModel.UserName).Should(Equal("user"))
		Expect(o.SourceEndpointModel.Password).Should(Equal("123456"))
		Expect(o.SourceEndpointModel.Host).Should(Equal("127.0.0.1"))
		Expect(o.SourceEndpointModel.Port).Should(Equal("5432"))

		Expect(o.SinkEndpointModel.EndpointType).Should(Equal(ClusterNameEndpointType))
		Expect(o.SinkEndpointModel.EndpointCharacterType).Should(Equal("foxlake"))
		Expect(o.SinkEndpointModel.Endpoint).Should(Equal(fakeFoxLakeClusterName))
		Expect(o.SinkEndpointModel.UserName).Should(Equal("test-user"))
		Expect(o.SinkEndpointModel.Password).Should(Equal("test-password"))
		Expect(o.SinkEndpointModel.Host).Should(Equal("192.168.0.1"))
		Expect(o.SinkEndpointModel.Port).Should(Equal("3306"))

		o.Sink = fakeClusterName
		Expect(o.Validate()).Should(HaveOccurred())

	})

	It("new command", func() {
		cmd := NewSync2FoxLakeCreateCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

})
