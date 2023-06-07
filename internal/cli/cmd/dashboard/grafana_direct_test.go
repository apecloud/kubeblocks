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

	var testCharType string
	expectURL := map[string]string{
		"apecloud-mysql": "http://127.0.0.1:8080/d/apecloud-mysql",
		"cadvisor":       "http://127.0.0.1:8080/d/cadvisor",
		"jmx":            "http://127.0.0.1:8080/d/jmx",
		"kafka":          "http://127.0.0.1:8080/d/kafka",
		"mongodb":        "http://127.0.0.1:8080/d/mongodb",
		"node":           "http://127.0.0.1:8080/d/node",
		"postgresql":     "http://127.0.0.1:8080/d/postgresql",
		"redis":          "http://127.0.0.1:8080/d/redis",
		"weaviate":       "http://127.0.0.1:8080/d/weaviate",
	}

	It("build grafana direct url", func() {
		testURL := "http://127.0.0.1:8080"
		testCharType = "invalid"
		Expect(buildGrafanaDirectURL(&testURL, testCharType)).Should(HaveOccurred())
		for charType, targetURL := range expectURL {
			testURL = "http://127.0.0.1:8080"
			Expect(buildGrafanaDirectURL(&testURL, charType)).Should(Succeed())
			Expect(testURL).Should(Equal(targetURL))
		}
	})
})
