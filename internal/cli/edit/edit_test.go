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
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("List", func() {
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	mockClient := func() *corev1.PodList {
		pods, _, _ := cmdtesting.TestData()
		tf = cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()

		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, pods)},
		}
		return pods
	}

	AfterEach(func() {
		tf.Cleanup()
	})

	It("test edit", func() {
		pods := mockClient()
		o := NewEditOptions(tf, streams, schema.GroupVersionResource{Group: "", Resource: "pods", Version: types.K8sCoreAPIVersion})
		cmd := &cobra.Command{
			Use:   "edit-test",
			Short: "edit test.",
			Run: func(cmd *cobra.Command, args []string) {

			},
		}
		o.AddFlags(cmd)
		podName := pods.Items[0].Name
		Expect(o.Complete(cmd, []string{})).Should(MatchError("missing the name"))
		Expect(o.Complete(cmd, []string{podName})).ShouldNot(HaveOccurred())
	})
})
