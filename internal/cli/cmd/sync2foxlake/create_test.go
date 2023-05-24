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
	restclient "k8s.io/client-go/rest"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
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
		ns := scheme.Codecs.WithoutConversion()
		cluster := testing.FakeCluster(fakeClusterName, namespace)
		foxlakecluster := testing.FakeCluster(fakeFoxLakeClusterName, namespace)
		foxlakecluster.Spec.ClusterDefRef = fakeFoxLakeClusterDefName
		foxlakecd := testing.FakeClusterDef()
		foxlakecd.Name = fakeFoxLakeClusterDefName
		foxlakecd.Spec.ComponentDefs[0].CharacterType = "foxlake"
		pods := testing.FakePods(3, namespace, fakeClusterName)
		pods.Items[0].Labels[constant.KBAppComponentLabelKey] = "foxlake-metadb"
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}
		tf.Client = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: ns,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				return map[string]*http.Response{
					urlPrefix + "/services":   httpResp(testing.FakeServices()),
					urlPrefix + "/secrets":    httpResp(testing.FakeSecrets(namespace, fakeClusterName)),
					urlPrefix + "/pods":       httpResp(pods),
					urlPrefix + "/configmaps": httpResp(testing.FakeConfigMap("test")),
				}[req.URL.Path], nil
			}),
		}

		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, foxlakecluster, foxlakecd, testing.FakeClusterDef(), testing.FakeClusterVersion())
		tf.ClientConfigVal = &restclient.Config{APIPath: "/api", ContentConfig: restclient.ContentConfig{NegotiatedSerializer: scheme.Codecs, GroupVersion: &schema.GroupVersion{Version: "v1"}}}
		streams = genericclioptions.NewTestIOStreamsDiscard()
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("new create options", func() {
		o := &CreateSync2FoxLakeOptions{Sync2FoxLakeExecOptions: newSync2FoxLakeExecOptions(tf, streams)}
		o.Sink = fakeFoxLakeClusterName
		o.Source = "root:root@127.0.0.1:3306"
		o.SelectedDatabase = "testdb"

		By("complete")
		Expect(o.complete([]string{name})).To(Succeed())

		Expect(o.Name).To(Equal(name))

		Expect(o.SourceEndpointModel.EndpointType).Should(Equal(AddressEndpointType))
		Expect(o.SourceEndpointModel.Endpoint).Should(Equal("127.0.0.1:3306"))
		Expect(o.SourceEndpointModel.UserName).Should(Equal("root"))
		Expect(o.SourceEndpointModel.Password).Should(Equal("root"))
		Expect(o.SourceEndpointModel.Host).Should(Equal("127.0.0.1"))
		Expect(o.SourceEndpointModel.Port).Should(Equal("3306"))

		Expect(o.SinkEndpointModel.EndpointType).Should(Equal(ClusterNameEndpointType))
		Expect(o.SinkEndpointModel.EndpointCharacterType).Should(Equal("foxlake"))
		Expect(o.SinkEndpointModel.Endpoint).Should(Equal(fakeFoxLakeClusterName))
		Expect(o.SinkEndpointModel.UserName).Should(Equal("test-user"))
		Expect(o.SinkEndpointModel.Password).Should(Equal("test-password"))
		Expect(o.SinkEndpointModel.Host).Should(Equal("192.168.0.1"))
		Expect(o.SinkEndpointModel.Port).Should(Equal("3306"))

		o.Sink = fakeClusterName
		Expect(o.complete([]string{name})).To(HaveOccurred())

		By("run")
		o.Cm.Data[name] = "::::::"
		err := o.run()
		Expect(err.Error()).To(Equal("Sync2foxlake task name " + name + " already exists"))

		o.Cm.Data = nil
		Expect(o.run()).To(HaveOccurred())
		Expect(len(o.Cm.Data)).To(Equal(1))
		Expect(o.Cm.Data[name]).To(Equal(fakeClusterName + ":testdb:" + fakeClusterName + "-pod-0:192.168.0.1:3306:test-user:test-password"))
	})
})
