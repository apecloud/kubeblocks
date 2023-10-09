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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	yamlv2 "gopkg.in/yaml.v2"
)

var _ = Describe("monitor_controller", func() {

	It("should generate config correctly from config yaml", func() {
		Eventually(func(g Gomega) {
			valMap := map[string]any{
				"transport": "http",
				//"meta.allow_native_password": false,
				//"meta.endpoint":              "http://",
				"password": "labels[\"pass\"]",
			}
			tplName := "test.cue"
			bytes, err := BuildFromCUEForOTel(tplName, valMap, "output")

			Expect(err).ShouldNot(HaveOccurred())
			slice := yamlv2.MapSlice{}
			err = yamlv2.Unmarshal(bytes, &slice)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(bytes).ShouldNot(BeNil())

		}).Should(Succeed())
	})

	It("should generate config correctly from config yaml", func() {
		Eventually(func(g Gomega) {

		}).Should(Succeed())
	})
})
