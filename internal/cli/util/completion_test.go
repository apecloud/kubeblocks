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

package util

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("completion", func() {
	const (
		namespace   = testing.Namespace
		clusterName = testing.ClusterName
	)

	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
		pods    = testing.FakePods(3, namespace, clusterName)
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("test completion pods", func() {
		cmd := get.NewCmdGet("kbcli", tf, streams)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &clientfake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Query().Get(metav1.LabelSelectorQueryParam("v1")) {
				case fmt.Sprintf("%s=%s", constant.RoleLabelKey, "leader"):
					return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &pods.Items[0])}, nil
				case "":
					return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &pods.Items[0])}, nil
				default:
					return nil, fmt.Errorf("unexpected request: %v", req.URL)
				}
			}),
		}

		Expect(len(CompGetResourceWithLabels(tf, cmd, "pods", []string{}, ""))).Should(Equal(1))
		Expect(len(CompGetResourceWithLabels(tf, cmd, "pods", []string{fmt.Sprintf("%s=%s", constant.RoleLabelKey, "leader")}, ""))).Should(Equal(1))
	})
})
