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
		engineType = MySQL
		name       = "test-cluster"
		namespace  = "test-namespace"
	)

	It("get and validate engine helm chart", func() {
		By("unsupported engine type")
		_, err := GetHelmChart("unsupported")
		Expect(err).Should(HaveOccurred())

		By("get engine helm chart")
		c, err := GetHelmChart(engineType)
		Expect(err).Should(Succeed())
		Expect(c).ShouldNot(BeNil())

		By("get manifests")
		manifests, err := GetManifests(c, namespace, name, nil)
		Expect(err).Should(Succeed())
		Expect(manifests).ShouldNot(BeEmpty())

		By("get engine schema")
		s, err := GetEngineSchema(c)
		Expect(err).Should(Succeed())
		Expect(s).ShouldNot(BeNil())
		Expect(s.Schema).ShouldNot(BeNil())
		Expect(s.SubSchema).ShouldNot(BeNil())
		Expect(s.SubChartName).ShouldNot(BeEmpty())

		By("get engine cluster definition")
		cd, err := GetEngineClusterDef(c)
		Expect(err).Should(Succeed())
		Expect(cd).ShouldNot(BeNil())

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
			err = ValidateValues(s, tc.values)
			if tc.success {
				Expect(err).Should(Succeed())
			} else {
				Expect(err).Should(HaveOccurred())
			}
		}
	})

	It("get cluster chart name", func() {
		By("mysql engine type")
		res := getEngineChartName(MySQL)
		Expect(res).Should(Equal("apecloud-mysql-cluster"))

		By("other engine type")
		eType := "unknown"
		res = getEngineChartName(EngineType(eType))
		Expect(res).Should(ContainSubstring("cluster"))
	})
})
