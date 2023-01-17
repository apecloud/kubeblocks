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
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const namespace = "test"

var _ = Describe("kubeblocks", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}

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
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
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
				Client:    testing.FakeClientSet(),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version:         version.DefaultKubeBlocksVersion,
			Monitor:         true,
			CreateNamespace: true,
		}
		Expect(o.Install()).Should(HaveOccurred())
		Expect(len(o.Sets)).To(Equal(1))
		Expect(o.Sets[0]).To(Equal(kMonitorParam))
		Expect(o.installChart()).Should(HaveOccurred())
		o.printNotes()
	})

	It("check upgrade", func() {
		var cfg string
		cmd = newUpgradeCmd(tf, streams)
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
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
		Expect(o.complete(tf, cmd)).To(Succeed())
		Expect(o.HelmCfg).ShouldNot(BeNil())
		Expect(o.Namespace).To(Equal("test"))
	})

	It("run upgrade", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.FakeActionConfig(),
				Namespace: "default",
			},
			Version: version.DefaultKubeBlocksVersion,
			Monitor: true,
		}
		Expect(o.Upgrade()).Should(HaveOccurred())
		Expect(len(o.Sets)).To(Equal(1))
		Expect(o.Sets[0]).To(Equal(kMonitorParam))
		Expect(o.upgradeChart()).Should(HaveOccurred())

		o.printNotes()
	})

	It("check uninstall", func() {
		var cfg string
		cmd = newUninstallCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &Options{
			IOStreams: streams,
		}
		Expect(o.complete(tf, cmd)).Should(Succeed())
		Expect(o.Namespace).Should(Equal(namespace))
		Expect(o.HelmCfg).ShouldNot(BeNil())
	})

	It("run uninstall", func() {
		o := &Options{
			IOStreams: streams,
			HelmCfg:   helm.FakeActionConfig(),
			Namespace: "default",
			Client:    testing.FakeClientSet(),
			Dynamic:   testing.FakeDynamicClient(),
		}

		Expect(o.uninstall()).Should(Succeed())
	})

	It("remove finalizer", func() {
		clusterDef := testing.FakeClusterDef()
		clusterDef.Finalizers = []string{"test"}
		clusterVersion := testing.FakeClusterVersion()
		clusterVersion.Finalizers = []string{"test"}
		backupTool := testing.FakeBackupTool()
		backupTool.Finalizers = []string{"test"}

		testCases := []struct {
			clusterDef     *dbaasv1alpha1.ClusterDefinition
			clusterVersion *dbaasv1alpha1.ClusterVersion
			backupTool     *dpv1alpha1.BackupTool
			expected       string
		}{
			{
				clusterDef:     testing.FakeClusterDef(),
				clusterVersion: testing.FakeClusterVersion(),
				backupTool:     testing.FakeBackupTool(),
				expected:       "Unable to remove nonexistent key: finalizers",
			},
			{
				clusterDef:     clusterDef,
				clusterVersion: testing.FakeClusterVersion(),
				backupTool:     testing.FakeBackupTool(),
				expected:       "Unable to remove nonexistent key: finalizers",
			},
			{
				clusterDef:     clusterDef,
				clusterVersion: clusterVersion,
				backupTool:     backupTool,
				expected:       "",
			},
		}

		for _, c := range testCases {
			client := testing.FakeDynamicClient(c.clusterDef, c.clusterVersion, c.backupTool)
			objs, _ := getKBObjects(testing.FakeClientSet(), client, "")
			if c.expected != "" {
				Expect(removeFinalizers(client, objs)).Should(MatchError(MatchRegexp(c.expected)))
			} else {
				Expect(removeFinalizers(client, objs)).Should(Succeed())
			}
		}
	})

	It("delete crd", func() {
		clusterCRD := v1.CustomResourceDefinition{
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
		clusterDefCRD := v1.CustomResourceDefinition{
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
		clusterVersionCRD := v1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "clusterversions.dbaas.kubeblocks.io",
			},
			Spec:   v1.CustomResourceDefinitionSpec{},
			Status: v1.CustomResourceDefinitionStatus{},
		}

		backupToolCRD := v1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "backuptools.dataprotection.kubeblocks.io",
			},
			Spec:   v1.CustomResourceDefinitionSpec{},
			Status: v1.CustomResourceDefinitionStatus{},
		}
		client := testing.FakeDynamicClient(&clusterCRD, &clusterDefCRD, &clusterVersionCRD, &backupToolCRD)
		objs, _ := getKBObjects(testing.FakeClientSet(), client, "")
		Expect(deleteCRDs(client, objs.crds)).Should(Succeed())
	})

	It("checkIfKubeBlocksInstalled", func() {
		By("KubeBlocks is not installed")
		client := testing.FakeClientSet()
		installed, version, err := checkIfKubeBlocksInstalled(client)
		Expect(err).Should(Succeed())
		Expect(installed).Should(Equal(false))
		Expect(version).Should(BeEmpty())

		mockDeploy := func(version string) *appsv1.Deployment {
			deploy := &appsv1.Deployment{}
			label := map[string]string{
				"app.kubernetes.io/name": types.KubeBlocksChartName,
			}
			if len(version) > 0 {
				label["app.kubernetes.io/version"] = version
			}
			deploy.SetLabels(label)
			return deploy
		}

		By("KubeBlocks is installed")
		client = testing.FakeClientSet(mockDeploy(""))
		installed, version, err = checkIfKubeBlocksInstalled(client)
		Expect(err).Should(Succeed())
		Expect(installed).Should(Equal(true))
		Expect(version).Should(BeEmpty())

		By("KubeBlocks 0.1.0 is installed")
		client = testing.FakeClientSet(mockDeploy("0.1.0"))
		installed, version, err = checkIfKubeBlocksInstalled(client)
		Expect(err).Should(Succeed())
		Expect(installed).Should(Equal(true))
		Expect(version).Should(Equal("0.1.0"))
	})

	It("confirmUninstall", func() {
		in := &bytes.Buffer{}
		_, _ = in.Write([]byte("\n"))
		Expect(confirmUninstall(in)).Should(HaveOccurred())

		in.Reset()
		_, _ = in.Write([]byte("uninstall-kubeblocks\n"))
		Expect(confirmUninstall(in)).Should(Succeed())
	})
})
