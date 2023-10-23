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

package dashboard

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("grafana open flag", func() {
	const baseURL = "http://127.0.0.1:8080"
	var testClusterType string
	var testURL string
	expectURL := map[string]string{
		"mysql": "http://127.0.0.1:8080/d/mysql",
		"":      "http://127.0.0.1:8080",
	}

	It("build grafana direct url", func() {
		testURL = baseURL
		testClusterType = "invalid"
		Expect(buildGrafanaDirectURL(&testURL, testClusterType)).Should(HaveOccurred())

		testURL = baseURL
		testClusterType = "mysql"
		Expect(buildGrafanaDirectURL(&testURL, testClusterType)).Should(Succeed())
		Expect(testURL).Should(Equal(expectURL[testClusterType]))

		testURL = baseURL
		testClusterType = ""
		Expect(buildGrafanaDirectURL(&testURL, testClusterType)).Should(Succeed())
		Expect(testURL).Should(Equal(expectURL[testClusterType]))
	})
})
