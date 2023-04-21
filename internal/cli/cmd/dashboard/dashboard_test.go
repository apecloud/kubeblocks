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

package dashboard

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

const namespace = "test"

var _ = Describe("kubeblocks", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	fakeSvcs := func() *corev1.ServiceList {
		svcs := &corev1.ServiceList{}
		svc := corev1.Service{}
		svc.SetName("kubeblocks-grafana")
		svc.SetNamespace(namespace)
		svc.SetLabels(map[string]string{
			"app.kubernetes.io/instance": "kubeblocks",
			"app.kubernetes.io/name":     "grafana",
		})
		svcs.Items = append(svcs.Items, svc)
		return svcs
	}

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/v1/services" && m == "GET":
					return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, fakeSvcs())}, nil
				default:
					return nil, nil
				}
			}),
		}
		tf.Client = tf.UnstructuredClient
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("dashboard cmd", func() {
		cmd := NewDashboardCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})

	It("list", func() {
		cmd := newListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		By("list options")

		o := newListOptions(tf, streams)
		Expect(o.complete()).Should(Succeed())
		Expect(o.run()).Should(Succeed())
	})

	It("open", func() {
		cmd := newOpenCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		Expect(cmd.Flags().Set(podRunningTimeoutFlag, time.Second.String())).Should(Succeed())
		By("open options")
		o := newOpenOptions(tf, streams)
		Expect(o).ShouldNot(BeNil())
		Expect(o.complete(cmd, []string{})).Should(HaveOccurred())
		Expect(o.complete(cmd, []string{"kubeblocks-grafana"})).Should(HaveOccurred())

		clientSet, err := tf.KubernetesClientSet()
		Expect(err).Should(Succeed())
		o.portForwardOptions.PodClient = clientSet.CoreV1()
		Expect(o.run()).Should(HaveOccurred())
	})
})
