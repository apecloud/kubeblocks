/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubeblocks

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("kubeblocks status", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("pre-run status", func() {
		var cfg string
		cmd = newStatusCmd(tf, streams)
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

	It("run status", func() {
		ns := "demo"

		mockDeploy := func() *appsv1.Deployment {
			deploy := &appsv1.Deployment{}
			deploy.SetNamespace(ns)
			deploy.SetLabels(map[string]string{
				"app.kubernetes.io/name":    types.KubeBlocksChartName,
				"app.kubernetes.io/version": "latest",
			})
			return deploy
		}

		o := &statusOptions{
			IOStreams: streams,
			ns:        ns,
			client:    testing.FakeClientSet(mockDeploy()),
			mc:        testing.FakeMetricsClientSet(),
			dynamic:   testing.FakeDynamicClient(mockDeploy()),
			showAll:   true,
		}
		By("make sure mocked deploy is injected")
		ctx := context.Background()
		deploys, err := o.dynamic.Resource(types.DeployGVR()).Namespace(ns).List(ctx, metav1.ListOptions{})
		Expect(err).Should(Succeed())
		Expect(len(deploys.Items)).Should(BeEquivalentTo(1))

		By("check deployment can be hit by selector")
		allErrs := make([]error, 0)
		unstructuredList := listResourceByGVR(ctx, o.dynamic, ns, kubeBlocksWorkloads, selectorList, &allErrs)
		Expect(len(unstructuredList)).Should(BeEquivalentTo(len(kubeBlocksWorkloads) * len(selectorList)))
		Expect(o.run()).To(Succeed())
	})
})
