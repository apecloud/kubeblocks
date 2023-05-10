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

package clusterversion

import (
	"bytes"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("clusterversion", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	out := new(bytes.Buffer)

	mockRestTable := func() *v1.Table {
		var Type = "string"
		tableHeader := make([]v1.TableColumnDefinition, 4)
		tableHeader[0].Name = "NAME"
		tableHeader[0].Type = Type
		tableHeader[1].Name = "CLUSTER-DEFINITION"
		tableHeader[1].Type = Type
		tableHeader[2].Name = "STATUS"
		tableHeader[2].Type = Type
		tableHeader[3].Name = "AGE"
		tableHeader[3].Type = Type
		value := make([]v1.TableRow, 1)
		value[0].Cells = make([]interface{}, 4)
		value[0].Cells[0] = testing.ClusterVersionName
		value[0].Cells[1] = testing.ClusterDefName
		value[0].Cells[2] = "Available"
		value[0].Cells[3] = "0s"

		table := &v1.Table{
			TypeMeta: v1.TypeMeta{
				Kind:       "Table",
				APIVersion: "meta.k8s.io/v1",
			},
			ColumnDefinitions: tableHeader,
			Rows:              value,
		}
		return table
	}

	mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
		tf := testing.NewTestFactory(testing.Namespace)
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

	BeforeEach(func() {
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		_ = v1.AddMetaToScheme(scheme.Scheme)
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		table := mockRestTable()
		tf = mockClient(table)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("clusterversion cmd", func() {
		cmd := NewClusterVersionCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})

	It("list", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("list --cluster-definition", func() {
		cmd := NewListCmd(tf, streams)
		cmd.Run(cmd, []string{"--cluster-definition=" + testing.ClusterDefName})
		expected := `NAME                   CLUSTER-DEFINITION        STATUS      AGE
fake-cluster-version   fake-cluster-definition   Available   0s
`
		Expect(expected).Should(Equal(out.String()))
	})
})
