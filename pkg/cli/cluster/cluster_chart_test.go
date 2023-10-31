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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster engine", func() {
	const (
		clusterType = "mysql"
		name        = "test-cluster"
		namespace   = "test-namespace"
		kubeVersion = "v99.99.0"
	)

	It("get and validate engine helm chart", func() {
		By("unsupported engine type")
		_, err := BuildChartInfo("unsupported")
		Expect(err).Should(HaveOccurred())

		By("build cluster chart info")
		c, err := BuildChartInfo(clusterType)
		Expect(err).Should(Succeed())
		Expect(c).ShouldNot(BeNil())
		Expect(c.Schema).ShouldNot(BeNil())
		Expect(c.SubSchema).ShouldNot(BeNil())
		Expect(c.SubChartName).ShouldNot(BeEmpty())

		By("get manifests")
		manifests, err := GetManifests(c.Chart, namespace, name, kubeVersion, nil)
		Expect(err).Should(Succeed())
		Expect(manifests).ShouldNot(BeEmpty())

		By("validate values")
		testCases := []struct {
			desc    string
			values  map[string]interface{}
			success bool
		}{
			{
				"cpu is greater than maximum",
				map[string]interface{}{
					"cpu": 1000,
				},
				// cpu should greater than 0.1
				false,
			},
			{
				"terminationPolicy is unknown",
				map[string]interface{}{
					"terminationPolicy": "unknown",
				},
				// "unknown" is not a valid value
				false,
			},
			{
				"all values are valid",
				map[string]interface{}{
					"cpu":               1.0,
					"terminationPolicy": "Halt",
				},
				true,
			},
		}
		for _, tc := range testCases {
			By(tc.desc)
			err = ValidateValues(c, tc.values)
			if tc.success {
				Expect(err).Should(Succeed())
			} else {
				Expect(err).Should(HaveOccurred())
			}
		}
	})
})
