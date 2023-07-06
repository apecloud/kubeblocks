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

package cluster

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("create cluster by clusterType type", func() {
	const (
		clusterType = "mysql"
	)

	var (
		tf            *cmdtesting.TestFactory
		streams       genericclioptions.IOStreams
		createOptions *create.CreateOptions
		mockClient    = func(data runtime.Object) *cmdtesting.TestFactory {
			tf = testing.NewTestFactory(testing.Namespace)
			codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
			tf.UnstructuredClient = &clientfake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
				Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
			}
			tf.Client = tf.UnstructuredClient
			tf.FakeDynamicClient = testing.FakeDynamicClient(data)
			return tf
		}
	)

	BeforeEach(func() {
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		_ = metav1.AddMetaToScheme(scheme.Scheme)
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = mockClient(testing.FakeClusterVersion())
		createOptions = &create.CreateOptions{
			IOStreams: streams,
			Factory:   tf,
		}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("cluster sub command", func() {
		By("create commands")
		cmds := buildCreateSubCmds(createOptions)
		Expect(cmds).ShouldNot(BeNil())
		Expect(cmds[0].HasFlags()).Should(BeTrue())

		By("create command options")
		o, err := newSubCmdsOptions(createOptions, clusterType)
		Expect(err).Should(Succeed())
		Expect(o).ShouldNot(BeNil())
		Expect(o.chartInfo).ShouldNot(BeNil())

		By("complete")
		cmd := cmds[0]
		o.Format = printer.YAML
		Expect(o.CreateOptions.Complete()).Should(Succeed())
		Expect(o.complete(cmd, nil)).Should(Succeed())
		Expect(o.Name).ShouldNot(BeEmpty())
		Expect(o.values).ShouldNot(BeNil())

		By("validate")
		o.chartInfo.ClusterDef = testing.ClusterDefName
		Expect(o.validate()).Should(Succeed())
		Expect(o.values[cluster.VersionSchemaProp.String()]).Should(Equal(testing.ClusterVersionName))

		By("run")
		o.DryRun = "client"
		Expect(o.run()).Should(Succeed())
	})
})
