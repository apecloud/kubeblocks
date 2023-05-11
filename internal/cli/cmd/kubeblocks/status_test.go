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

package kubeblocks

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("kubeblocks status", func() {
	var (
		namespace  = "test"
		streams    genericclioptions.IOStreams
		tf         *cmdtesting.TestFactory
		stsName    = "test-sts"
		deployName = "test-deploy"
	)

	BeforeEach(func() {
		tf = cmdtesting.NewTestFactory().WithNamespace("test")
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		// add workloads
		extraLabels := map[string]string{
			"appName": "JohnSnow",
			"slogan":  "YouknowNothing",
		}

		deploy := testing.FakeDeploy(deployName, namespace, extraLabels)
		deploymentList := &appsv1.DeploymentList{}
		deploymentList.Items = []appsv1.Deployment{*deploy}

		sts := testing.FakeStatefulSet(stsName, namespace, extraLabels)
		statefulSetList := &appsv1.StatefulSetList{}
		statefulSetList.Items = []appsv1.StatefulSet{*sts}
		stsPods := testing.FakePodForSts(sts)

		job := testing.FakeJob("test-job", namespace, extraLabels)
		jobList := &batchv1.JobList{}
		jobList.Items = []batchv1.Job{*job}

		cronjob := testing.FakeCronJob("test-cronjob", namespace, extraLabels)
		cronjobList := &batchv1.CronJobList{}
		cronjobList.Items = []batchv1.CronJob{*cronjob}

		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				return map[string]*http.Response{
					urlPrefix + "/deployments":  httpResp(deploymentList),
					urlPrefix + "/statefulsets": httpResp(statefulSetList),
					urlPrefix + "/jobs":         httpResp(jobList),
					urlPrefix + "/cronjobs":     httpResp(cronjobList),
					urlPrefix + "/pods":         httpResp(stsPods),
				}[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(deploy, sts)
		streams = genericclioptions.NewTestIOStreamsDiscard()
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("pre-run status", func() {
		var cfg string
		cmd := newStatusCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &statusOptions{
			IOStreams: streams,
		}

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")

		Expect(o.complete(tf)).To(Succeed())
		Expect(o.client).ShouldNot(BeNil())
		Expect(o.dynamic).ShouldNot(BeNil())
		Expect(o.ns).To(Equal(metav1.NamespaceAll))
		Expect(o.showAll).To(Equal(false))
	})

	It("list resources", func() {
		clientSet, _ := tf.KubernetesClientSet()
		o := &statusOptions{
			IOStreams: streams,
			ns:        namespace,
			client:    clientSet,
			mc:        testing.FakeMetricsClientSet(),
			dynamic:   tf.FakeDynamicClient,
			showAll:   true,
		}
		By("make sure mocked deploy is injected")
		ctx := context.Background()
		deploys, err := o.dynamic.Resource(types.DeployGVR()).Namespace(namespace).List(ctx, metav1.ListOptions{})
		Expect(err).Should(Succeed())
		Expect(len(deploys.Items)).Should(BeEquivalentTo(1))

		statefulsets, err := o.dynamic.Resource(types.StatefulSetGVR()).Namespace(namespace).List(ctx, metav1.ListOptions{})
		Expect(err).Should(Succeed())
		Expect(len(statefulsets.Items)).Should(BeEquivalentTo(1))

		By("check deployment can be hit by selector")
		allErrs := make([]error, 0)
		o.buildSelectorList(ctx, &allErrs)
		unstructuredList := listResourceByGVR(ctx, o.dynamic, namespace, kubeBlocksWorkloads, o.selectorList, &allErrs)
		// will list update to five types of worklaods
		Expect(len(unstructuredList)).Should(BeEquivalentTo(5))
		for _, list := range unstructuredList {
			if list.GetKind() == constant.DeploymentKind || list.GetKind() == constant.StatefulSetKind || list.GetKind() == constant.JobKind || list.GetKind() == constant.CronJobKind {
				Expect(len(list.Items)).Should(BeEquivalentTo(1))
			} else {
				Expect(len(list.Items)).Should(BeEquivalentTo(0))
			}
		}
		Expect(o.run()).To(Succeed())
	})
})
