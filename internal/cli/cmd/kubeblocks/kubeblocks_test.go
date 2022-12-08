/*
Copyright ApeCloud Inc.

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const nameSpace = "test"

var _ = Describe("kubeblocks", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(nameSpace)

		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("kubeblocks", func() {
		cmd = NewKubeBlocksCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})

	It("check install", func() {
		var cfg string
		cmd = newInstallCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
			},
		}

		By("command without kubeconfig flag")
		Expect(o.complete(tf, cmd)).Should(HaveOccurred())

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		Expect(o.complete(tf, cmd)).To(Succeed())
		Expect(o.HelmCfg).ShouldNot(BeNil())
		Expect(o.Namespace).To(Equal("test"))
	})

	It("run install", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.FakeActionConfig(),
				Namespace: "default",
			},
			Version: version.DefaultKubeBlocksVersion,
			Monitor: true,
		}
		Expect(o.Run()).Should(MatchError(MatchRegexp("not a valid Chart repository")))
		Expect(len(o.Sets)).To(Equal(1))
		Expect(o.Sets[0]).To(Equal(kMonitorParam))

		notes, err := o.installChart()
		Expect(err).Should(MatchError(MatchRegexp("failed to download")))
		Expect(notes).Should(Equal(""))

		o.printNotes("")
	})

	It("check uninstall", func() {
		var cfg string
		cmd = newUninstallCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &Options{
			IOStreams: streams,
		}
		Expect(o.complete(tf, cmd)).Should(Succeed())
		Expect(o.Namespace).Should(Equal(nameSpace))
		Expect(o.HelmCfg).ShouldNot(BeNil())
	})

	It("run uninstall", func() {
		o := &Options{
			IOStreams: streams,
			HelmCfg:   helm.FakeActionConfig(),
			Namespace: "default",
		}

		Expect(o.run()).Should(MatchError(MatchRegexp("release: not found")))
	})

	It("remove finalizer", func() {
		clusterDef := testing.FakeClusterDef()
		clusterDef.Finalizers = []string{"test"}
		appVer := testing.FakeAppVersion()
		appVer.Finalizers = []string{"test"}

		testCases := []struct {
			clusterDef *dbaasv1alpha1.ClusterDefinition
			appVersion *dbaasv1alpha1.AppVersion
			expected   string
		}{
			{
				clusterDef: testing.FakeClusterDef(),
				appVersion: testing.FakeAppVersion(),
				expected:   "Unable to remove nonexistent key: finalizers",
			},
			{
				clusterDef: clusterDef,
				appVersion: testing.FakeAppVersion(),
				expected:   "Unable to remove nonexistent key: finalizers",
			},
			{
				clusterDef: clusterDef,
				appVersion: appVer,
				expected:   "",
			},
		}

		for _, c := range testCases {
			client := testing.FakeDynamicClient(c.clusterDef, c.appVersion)
			if c.expected != "" {
				Expect(removeFinalizers(client)).Should(MatchError(MatchRegexp(c.expected)))
			} else {
				Expect(removeFinalizers(client)).Should(Succeed())
			}
		}
	})

	It("delete crd", func() {
		clusterCrd := v1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "clusters.dbaas.kubeblocks.io",
			},
			Spec:   v1.CustomResourceDefinitionSpec{},
			Status: v1.CustomResourceDefinitionStatus{},
		}
		clusterDefCrd := v1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "clusterdefinitions.dbaas.kubeblocks.io",
			},
			Spec:   v1.CustomResourceDefinitionSpec{},
			Status: v1.CustomResourceDefinitionStatus{},
		}
		appVerCrd := v1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "appversions.dbaas.kubeblocks.io",
			},
			Spec:   v1.CustomResourceDefinitionSpec{},
			Status: v1.CustomResourceDefinitionStatus{},
		}

		client := testing.FakeDynamicClient(&clusterCrd, &clusterDefCrd, &appVerCrd)
		Expect(deleteCRDs(client)).Should(Succeed())
	})
})
