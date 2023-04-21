/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

const kbVersion = "0.3.0"

var _ = Describe("version util", func() {
	It("get version info when client is nil", func() {
		v, err := GetVersionInfo(nil)
		Expect(err).Should(Succeed())
		Expect(v.KubeBlocks).Should(BeEmpty())
		Expect(v.Kubernetes).Should(BeEmpty())
		Expect(v.Cli).ShouldNot(BeEmpty())
	})

	It("get version info when client variable is a nil pointer", func() {
		var client *kubernetes.Clientset
		v, err := GetVersionInfo(client)
		Expect(err).Should(Succeed())
		Expect(v.KubeBlocks).Should(BeEmpty())
		Expect(v.Kubernetes).Should(BeEmpty())
		Expect(v.Cli).ShouldNot(BeEmpty())
	})

	It("get vsion info when KubeBlocks is deployed", func() {
		client := testing.FakeClientSet(testing.FakeKBDeploy(kbVersion))
		v, err := GetVersionInfo(client)
		Expect(err).Should(Succeed())
		Expect(v.KubeBlocks).Should(Equal(kbVersion))
		Expect(v.Kubernetes).ShouldNot(BeEmpty())
		Expect(v.Cli).ShouldNot(BeEmpty())
	})

	It("get version info when KubeBlocks is not deployed", func() {
		client := testing.FakeClientSet()
		v, err := GetVersionInfo(client)
		Expect(err).Should(Succeed())
		Expect(v.KubeBlocks).Should(BeEmpty())
		Expect(v.Kubernetes).ShouldNot(BeEmpty())
		Expect(v.Cli).ShouldNot(BeEmpty())
	})

	It("getKubeBlocksVersion", func() {
		client := testing.FakeClientSet(testing.FakeKBDeploy(""))
		v, err := getKubeBlocksVersion(client)
		Expect(v).Should(BeEmpty())
		Expect(err).Should(HaveOccurred())

		client = testing.FakeClientSet(testing.FakeKBDeploy(kbVersion))
		v, err = getKubeBlocksVersion(client)
		Expect(v).Should(Equal(kbVersion))
		Expect(err).Should(Succeed())
	})

	It("GetK8sVersion", func() {
		client := testing.FakeClientSet()
		v, err := GetK8sVersion(client.Discovery())
		Expect(v).ShouldNot(BeEmpty())
		Expect(err).Should(Succeed())
	})
})
