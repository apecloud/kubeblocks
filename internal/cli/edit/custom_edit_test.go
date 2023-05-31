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

package edit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Custom edit", func() {
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("test edit the cluster resource before updating", func() {
		options := NewCustomEditOptions(tf, streams, "update")
		resObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"clusterDefinitionRef": "apecloud-mysql",
				},
			},
		}
		Expect(options.Run(resObj, true)).Should(Succeed())
	})

	It("test edit the cluster resource before creating", func() {
		options := NewCustomEditOptions(tf, streams, "create")
		resObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"clusterDefinitionRef": "apecloud-mysql",
				},
			},
		}
		Expect(options.Run(resObj, true)).Should(Succeed())
	})
})
