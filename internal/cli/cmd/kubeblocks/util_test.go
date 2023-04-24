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
