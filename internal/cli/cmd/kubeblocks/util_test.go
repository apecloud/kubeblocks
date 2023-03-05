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
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("kubeblocks", func() {
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
