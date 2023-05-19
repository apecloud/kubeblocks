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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
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

	It("printAddonMsg", func() {
		const (
			reason = "test-failed-reason"
		)

		fakeAddOn := func(name string, conditionTrue bool, msg string) *extensionsv1alpha1.Addon {
			addon := &extensionsv1alpha1.Addon{}
			addon.Name = name
			addon.Status = extensionsv1alpha1.AddonStatus{}
			if conditionTrue {
				addon.Status.Phase = extensionsv1alpha1.AddonEnabled
			} else {
				addon.Status.Phase = extensionsv1alpha1.AddonFailed
				addon.Status.Conditions = []metav1.Condition{
					{
						Message: msg,
						Reason:  reason,
						Status:  metav1.ConditionFalse,
					},
					{
						Message: msg,
						Reason:  reason,
						Status:  metav1.ConditionFalse,
					},
				}
			}
			return addon
		}

		testCases := []struct {
			desc     string
			addons   []*extensionsv1alpha1.Addon
			expected string
		}{
			{
				desc:     "addons is nil",
				addons:   nil,
				expected: "",
			},
			{
				desc: "addons without false condition",
				addons: []*extensionsv1alpha1.Addon{
					fakeAddOn("addon", true, ""),
				},
				expected: "",
			},
			{
				desc: "addons with false condition",
				addons: []*extensionsv1alpha1.Addon{
					fakeAddOn("addon1", true, ""),
					fakeAddOn("addon2", false, "failed to enable addon2"),
				},
				expected: "failed to enable addon2",
			},
		}

		for _, c := range testCases {
			By(c.desc)
			out := &bytes.Buffer{}
			printAddonMsg(out, c.addons, true)
			Expect(out.String()).To(ContainSubstring(c.expected))
		}
	})
})
