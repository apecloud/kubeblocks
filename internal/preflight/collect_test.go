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

package preflight

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	preflightTesting "github.com/apecloud/kubeblocks/internal/preflight/testing"
)

var _ = Describe("collect_test", func() {
	var (
		timeOut       = 10 * time.Second
		namespace     = "test"
		clusterName   = "test"
		tf            *cmdtesting.TestFactory
		cluster       = testing.FakeCluster(clusterName, namespace)
		pods          = testing.FakePods(3, namespace, clusterName)
		preflight     *preflightv1beta2.Preflight
		hostPreflight *preflightv1beta2.HostPreflight
	)

	BeforeEach(func() {
		_, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" {
					return nil, nil
				}
				urlPrefix := "/api/v1/namespaces/" + namespace
				mapping := map[string]*http.Response{
					"/api/v1/nodes/" + testing.NodeName: httpResp(testing.FakeNode()),
					urlPrefix + "/services":             httpResp(&corev1.ServiceList{}),
					urlPrefix + "/events":               httpResp(&corev1.EventList{}),
					urlPrefix + "/pods":                 httpResp(pods),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())

		preflight = preflightTesting.FakeKbPreflight()
		hostPreflight = preflightTesting.FakeKbHostPreflight()
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("CollectPreflight test, and expect success ", func() {
		Eventually(func(g Gomega) {
			progressCh := make(chan interface{})
			go func() {
				for {
					g.Expect(<-progressCh).NotTo(BeNil())
				}
			}()
			results, err := CollectPreflight(context.TODO(), preflight, hostPreflight, progressCh)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(results)).Should(BeNumerically(">=", 3))
		}).WithTimeout(timeOut).Should(Succeed())
	})

	It("CollectHostData Test, and expect success", func() {
		Eventually(func(g Gomega) {
			progressCh := make(chan interface{})
			go func() {
				for {
					g.Expect(<-progressCh).NotTo(BeNil())
				}
			}()
			results, err := CollectHostData(context.TODO(), hostPreflight, progressCh)
			g.Expect(err).NotTo(HaveOccurred())
			_, ok := (*results).(KBHostCollectResult)
			g.Expect(ok).Should(BeTrue())
		}).WithTimeout(timeOut).Should(Succeed())
	})

	It("CollectRemoteData test, and expect success", func() {
		Eventually(func(g Gomega) {
			progressCh := make(chan interface{})
			go func() {
				for {
					g.Expect(<-progressCh).NotTo(BeNil())
				}
			}()
			collectResult, err := CollectRemoteData(context.TODO(), &preflightv1beta2.HostPreflight{}, progressCh)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(collectResult).NotTo(BeNil())
		}).WithTimeout(timeOut).Should(Succeed())
	})

	It("ParseTimeFlags test, and expect success", func() {
		sinceStr := "5m"
		sinceTimeStr := "2023-01-09T15:18:46+08:00"
		Expect(ParseTimeFlags(sinceStr, sinceTimeStr, []*troubleshoot.Collect{})).Should(HaveOccurred())
		Expect(ParseTimeFlags(sinceTimeStr, "", []*troubleshoot.Collect{})).Should(Succeed())
		Expect(ParseTimeFlags("", sinceStr, []*troubleshoot.Collect{})).Should(Succeed())
	})

	It("ParseTimeFlags test, and expect error", func() {
		sinceStr := "5error-m"
		sinceTimeStr := "2023-01-09T15:46+:00"
		Expect(ParseTimeFlags("", "", []*troubleshoot.Collect{})).Should(HaveOccurred())
		Expect(ParseTimeFlags(sinceStr, "", []*troubleshoot.Collect{})).Should(HaveOccurred())
		Expect(ParseTimeFlags("", sinceTimeStr, []*troubleshoot.Collect{})).Should(HaveOccurred())

	})
})
