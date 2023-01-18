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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/spf13/cobra"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
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
		Expect(o.Sets[0]).To(Equal(fmt.Sprintf(kMonitorParam, true)))
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
		mockDeploy := func() *appsv1.Deployment {
			deploy := &appsv1.Deployment{}
			deploy.SetLabels(map[string]string{
				"app.kubernetes.io/name":    types.KubeBlocksChartName,
				"app.kubernetes.io/version": "0.3.0",
			})
			return deploy
		}

		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.FakeActionConfig(),
				Namespace: "default",
				Client:    testing.FakeClientSet(mockDeploy()),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version: version.DefaultKubeBlocksVersion,
			Monitor: true,
			check:   false,
		}
		cmd := newUpgradeCmd(tf, streams)
		_ = cmd.Flags().Set("monitor", "true")
		Expect(o.upgrade(cmd)).Should(HaveOccurred())
		Expect(len(o.Sets)).To(Equal(1))
		Expect(o.Sets[0]).To(Equal(fmt.Sprintf(kMonitorParam, true)))
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
			client := mockDynamicClientWithCRD(c.clusterDef, c.clusterVersion, c.backupTool)
			objs, _ := getKBObjects(testing.FakeClientSet(), client, "")
			if c.expected != "" {
				Expect(removeFinalizers(client, objs)).Should(MatchError(MatchRegexp(c.expected)))
			} else {
				Expect(removeFinalizers(client, objs)).Should(Succeed())
			}
		}
	})

	It("delete crd", func() {
		client := mockDynamicClientWithCRD()
		objs, _ := getKBObjects(testing.FakeClientSet(), client, "")
		Expect(deleteCRDs(client, objs.crds)).Should(Succeed())
	})

	It("preCheck", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			},
			check: true,
		}
		By("kubernetes version is empty")
		versionInfo := map[util.AppName]string{}
		Expect(o.preCheck(versionInfo).Error()).Should(ContainSubstring("failed to get kubernetes version"))

		versionInfo[util.KubernetesApp] = ""
		Expect(o.preCheck(versionInfo).Error()).Should(ContainSubstring("failed to get kubernetes version"))

		By("kubernetes version is smaller than required version")
		versionInfo[util.KubernetesApp] = "v1.20.0"
		Expect(o.preCheck(versionInfo).Error()).Should(ContainSubstring("should be larger than"))

		By("kubernetes is provided by cloud provider")
		versionInfo[util.KubernetesApp] = "v1.25.0-eks"
		Expect(o.preCheck(versionInfo)).Should(Succeed())

		By("kubernetes is not provided by cloud provider")
		versionInfo[util.KubernetesApp] = "v1.25.0"
		Expect(o.preCheck(versionInfo)).Should(Succeed())
	})

	It("disableUnsupportedSets", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			},
		}
		cases := []struct {
			desc     string
			sets     []string
			expected []string
		}{
			{
				"sets is empty", []string{}, []string{},
			},
			{
				"sets is empty", nil, nil,
			},
			{
				"sets without unsupported flag",
				[]string{"test=false"},
				[]string{"test=false"},
			},
			{
				"sets with unsupported flag and its value is false",
				[]string{"test=false", "loadbalancer.enable=false"},
				[]string{"test=false", "loadbalancer.enable=false"},
			},
			{
				"sets with unsupported flag and its value is true",
				[]string{"test=false", "loadbalancer.enable=true"},
				[]string{"test=false"},
			},
			{
				"sets with more unsupported flags and the value is true",
				[]string{"test=false", "loadbalancer.enable=true", "snapshot-controller.enable=true"},
				[]string{"test=false"},
			},
			{
				"sets with more unsupported flags and the value is true",
				[]string{"test=false", "loadbalancer.enable=true, snapshot-controller.enable=true"},
				[]string{"test=false"},
			},
			{
				"sets with more unsupported flags and some values are true, some values are false",
				[]string{"test=false", "loadbalancer.enable=false, snapshot-controller.enable=true"},
				[]string{"test=false", "loadbalancer.enable=false"},
			},
			{
				"sets with more unsupported flags and some values are true, some values are false",
				[]string{"test=false,loadbalancer.enable=false,snapshot-controller.enable=true"},
				[]string{"test=false", "loadbalancer.enable=false"},
			},
		}

		for _, c := range cases {
			By(c.desc)
			o.Sets = c.sets
			o.disableUnsupportedSets()
			Expect(o.Sets).Should(Equal(c.expected))
		}
	})
})

func mockDynamicClientWithCRD(objects ...runtime.Object) dynamic.Interface {
	clusterCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusters.dbaas.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.Group,
		},
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
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.Group,
		},
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
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.Group,
		},
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
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.DPGroup,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}

	allObjs := []runtime.Object{&clusterCRD, &clusterDefCRD, &clusterVersionCRD, &backupToolCRD}
	allObjs = append(allObjs, objects...)
	return testing.FakeDynamicClient(allObjs...)
}
