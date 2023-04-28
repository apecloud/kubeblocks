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
	"bytes"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("list", func() {
	var (
		streams genericclioptions.IOStreams
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
	)

	const (
		namespace             = "test"
		clusterName           = "test"
		clusterName1          = "test1"
		clusterName2          = "test2"
		verticalScalingReason = "VerticalScaling"
	)

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)

		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		cluster := testing.FakeCluster(clusterName, namespace, metav1.Condition{
			Type:   appsv1alpha1.ConditionTypeApplyResources,
			Status: metav1.ConditionFalse,
			Reason: "HorizontalScaleFailed",
		})
		clusterWithVerticalScaling := testing.FakeCluster(clusterName1, namespace, metav1.Condition{
			Type:   appsv1alpha1.ConditionTypeLatestOpsRequestProcessed,
			Status: metav1.ConditionFalse,
			Reason: verticalScalingReason,
		})
		clusterWithVerticalScaling.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
		clusterWithAbnormalPhase := testing.FakeCluster(clusterName2, namespace)
		clusterWithAbnormalPhase.Status.Phase = appsv1alpha1.AbnormalClusterPhase
		pods := testing.FakePods(3, namespace, clusterName)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				return map[string]*http.Response{
					"/namespaces/" + namespace + "/clusters":                 httpResp(&appsv1alpha1.ClusterList{Items: []appsv1alpha1.Cluster{*cluster}}),
					"/namespaces/" + namespace + "/clusters/" + clusterName:  httpResp(cluster),
					"/namespaces/" + namespace + "/clusters/" + clusterName1: httpResp(clusterWithVerticalScaling),
					"/namespaces/" + namespace + "/clusters/" + clusterName2: httpResp(clusterWithAbnormalPhase),
					"/namespaces/" + namespace + "/secrets":                  httpResp(testing.FakeSecrets(namespace, clusterName)),
					"/api/v1/nodes/" + testing.NodeName:                      httpResp(testing.FakeNode()),
					urlPrefix + "/services":                                  httpResp(&corev1.ServiceList{}),
					urlPrefix + "/secrets":                                   httpResp(testing.FakeSecrets(namespace, clusterName)),
					urlPrefix + "/pods":                                      httpResp(pods),
					urlPrefix + "/events":                                    httpResp(testing.FakeEvents()),
				}[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, clusterWithVerticalScaling, clusterWithAbnormalPhase, testing.FakeClusterDef(), testing.FakeClusterVersion())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("list", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{clusterName, clusterName1, clusterName2})
		Expect(out.String()).Should(ContainSubstring(testing.ClusterDefName))
		Expect(out.String()).Should(ContainSubstring(verticalScalingReason))
		Expect(out.String()).Should(ContainSubstring(cluster.ConditionsError))
		Expect(out.String()).Should(ContainSubstring(string(appsv1alpha1.AbnormalClusterPhase)))
	})

	It("list instances", func() {
		cmd := NewListInstancesCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.NodeName))
	})

	It("list components", func() {
		cmd := NewListComponentsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.ComponentName))
	})

	It("list events", func() {
		cmd := NewListEventsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{"test"})
		Expect(len(strings.Split(out.String(), "\n")) > 1).Should(BeTrue())
	})

	It("output wide", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		Expect(cmd.Flags().Set("output", "wide")).Should(Succeed())
		cmd.Run(cmd, []string{"test"})
		Expect(out.String()).Should(ContainSubstring(testing.ClusterVersionName))
	})

	It("output wide without args", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		Expect(cmd.Flags().Set("output", "wide")).Should(Succeed())
		cmd.Run(cmd, []string{})
		Expect(out.String()).Should(ContainSubstring(testing.ClusterVersionName))
	})
})
