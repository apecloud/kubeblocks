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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/cli/values"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("backupconfig", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}

		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("run cmd", func() {
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
				HelmCfg:   helm.NewFakeConfig(testing.Namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(mockDeploy()),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version:   version.DefaultKubeBlocksVersion,
			Monitor:   true,
			ValueOpts: values.Options{Values: []string{"snapshot-controller.enabled=true"}},
		}
		cmd := NewConfigCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(o.Install()).Should(Succeed())
	})
})
