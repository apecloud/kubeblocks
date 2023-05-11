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
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("Expose", func() {
	const (
		namespace = "test"
		opsName   = "test-ops"
		opsName1  = "test-ops1"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		in      *bytes.Buffer
	)
	generateOpsObject := func(opsName string, phase appsv1alpha1.OpsPhase) *appsv1alpha1.OpsRequest {
		return &appsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      opsName,
				Namespace: namespace,
			},
			Spec: appsv1alpha1.OpsRequestSpec{
				ClusterRef: "test-cluster",
				Type:       "Restart",
			},
			Status: appsv1alpha1.OpsRequestStatus{
				Phase: phase,
			},
		}
	}
	BeforeEach(func() {
		streams, in, _, _ = genericclioptions.NewTestIOStreams()
		tf = clitesting.NewTestFactory(namespace)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	initClient := func(opsRequest runtime.Object) {
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				return httpResp(opsRequest), nil
			}),
		}

		tf.FakeDynamicClient = clitesting.FakeDynamicClient(opsRequest)
		tf.Client = tf.UnstructuredClient
	}

	It("test completeForDeleteOps function", func() {
		clusterName := "wesql"
		args := []string{clusterName}
		clusterLabel := util.BuildLabelSelectorByNames("", args)
		testLabel := "kubeblocks.io/test=test"

		By("test delete OpsRequest with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.OpsGVR())
		Expect(completeForDeleteOps(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with cluster and custom label")
		o.LabelSelector = testLabel
		Expect(completeForDeleteOps(o, args)).Should(Succeed())
		Expect(o.LabelSelector == testLabel+","+clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with name")
		o.Names = []string{"test1"}
		Expect(completeForDeleteOps(o, nil)).Should(Succeed())
		Expect(len(o.ConfirmedNames)).Should(Equal(1))
	})

	It("Testing the deletion of running OpsRequest", func() {
		By("init opsRequests and k8s client")
		runningOps := generateOpsObject(opsName1, appsv1alpha1.OpsRunningPhase)
		initClient(runningOps)

		By("expect error when deleting running opsRequest")
		o := delete.NewDeleteOptions(tf, streams, types.OpsGVR())
		o.PreDeleteHook = preDeleteOps
		o.Names = []string{runningOps.Name}
		in.Write([]byte(runningOps.Name + "\n"))
		err := o.Run()
		Expect(err).ShouldNot(BeNil())
		Expect(err.Error()).Should(Equal(fmt.Sprintf(`OpsRequest "%s" is Running, you can specify "--force" to delete it`, runningOps.Name)))

		By("expect success when deleting running opsRequest with --force")
		o.GracePeriod = 0
		o.Names = []string{runningOps.Name}
		in.Write([]byte(runningOps.Name + "\n"))
		o.Force = true
		err = o.Run()
		Expect(err).Should(BeNil())
	})
})
