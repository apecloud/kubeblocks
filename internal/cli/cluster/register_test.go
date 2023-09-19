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
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster register", func() {
	It("test builtin chart", func() {
		mysql := &embedConfig{
			chartFS: mysqlChart,
			name:    "apecloud-mysql-cluster.tgz",
			alias:   "",
		}
		Expect(mysql.register("mysql")).Should(HaveOccurred())
		Expect(mysql.register("mysql-other")).Should(Succeed())
		Expect(mysql.getChartFileName()).Should(Equal("apecloud-mysql-cluster.tgz"))
		Expect(mysql.getAlias()).Should(Equal(""))
		chart, err := mysql.loadChart()
		Expect(err).Should(Succeed())
		bytes, err := io.ReadAll(chart)
		Expect(bytes).ShouldNot(BeEmpty())
		Expect(err).Should(Succeed())
	})

	It("test external chart", func() {
		fakeChart := &TypeInstance{
			Name:      "fake",
			URL:       "www.fake-chart-hub/fake.tgz",
			Alias:     "",
			ChartName: "fake.tgz",
		}
		Expect(fakeChart.getAlias()).Should(Equal(""))
		Expect(fakeChart.getChartFileName()).Should(Equal("fake.tgz"))
		_, err := fakeChart.loadChart()
		Expect(err).Should(HaveOccurred())
		Expect(fakeChart.register("fake")).Should(HaveOccurred())
	})

	Context("test Config reader", func() {
		var tempConfigPath string

		var tempCLusterConfig clusterConfig
		var configContent = `- name: orioledb
  helmChartUrl: https://github.com/apecloud/helm-charts/releases/download/orioledb-cluster-0.7.0-alpha.7/orioledb-cluster-0.7.0-alpha.7.tgz
  alias: ""
`
		BeforeEach(func() {
			tempConfigPath = filepath.Join(os.TempDir(), "kbcli_test")
			Expect(os.WriteFile(tempConfigPath, []byte(configContent), 0666)).Should(Succeed())
		})

		AfterEach(func() {
			os.Remove(tempConfigPath)
		})

		It("test read configs and remove", func() {
			Expect(tempCLusterConfig.ReadConfigs(tempConfigPath)).Should(Succeed())
			Expect(tempCLusterConfig.Len()).Should(Equal(1))
			Expect(tempCLusterConfig.RemoveConfig("orioledb")).Should(BeTrue())
			Expect(tempCLusterConfig.Len()).Should(Equal(0))
		})

		It("test add config and write", func() {
			tempCLusterConfig.AddConfig(&TypeInstance{
				Name:  "orioledb",
				URL:   "https://fakeurl.com",
				Alias: "",
			})
			Expect(tempCLusterConfig.Len()).Should(Equal(1))
			Expect(tempCLusterConfig.WriteConfigs(tempConfigPath)).Should(Succeed())

			file, _ := os.ReadFile(tempConfigPath)
			Expect(string(file)).Should(Equal("- name: orioledb\n  helmChartUrl: https://fakeurl.com\n  alias: \"\"\n  chartName: \"\"\n"))
		})

	})
})
