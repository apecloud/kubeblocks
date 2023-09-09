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

package monitor

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
)

var _ = Describe("monitor_controller", func() {
	var (
		logsExporterList    v1alpha1.LogsExporterSinkList
		metricsExporterList v1alpha1.MetricsExporterSinkList
		datasourceList      v1alpha1.CollectorDataSourceList
	)

	BeforeEach(func() {
		logsExporterList = fakeLogsExporterSinkList()
		metricsExporterList = fakeMetricsExporterSinkList()
		datasourceList = fakeCollectorDataSourceList()
	})

	It("should generate config correctly from config yaml", func() {
		Eventually(func(g Gomega) {
			config, err := types.LoadConfig("./data/config_test.yaml")
			if err != nil {
				return
			}
			g.Expect(config).ShouldNot(BeNil())
		}).Should(Succeed())
	})

	It("should generate oteld correctly", func() {

		config, err := types.LoadConfig("./data/config_test.yaml")
		Expect(err).ShouldNot(HaveOccurred())

		By("create cluster & clusterDef")
		cg := NewConfigGenerator(config)
		cfg := cg.GenerateOteldConfiguration(&datasourceList, &metricsExporterList, &logsExporterList)
		bytes, err := yaml.Marshal(cfg)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(bytes)).Should(Equal(""))
		Expect(true).Should(BeTrue())
	})

})

func fakeCollectorDataSourceList() v1alpha1.CollectorDataSourceList {
	return v1alpha1.CollectorDataSourceList{
		Items: []v1alpha1.CollectorDataSource{
			{
				Spec: v1alpha1.CollectorDataSourceSpec{Type: v1alpha1.MetricsDatasourceType,
					ExporterRef: v1alpha1.ExporterRef{ExporterNames: []string{"prometheus"}}},
			},
		},
	}
}

func fakeMetricsExporterSinkList() v1alpha1.MetricsExporterSinkList {
	return v1alpha1.MetricsExporterSinkList{
		Items: []v1alpha1.MetricsExporterSink{
			{
				Spec: v1alpha1.MetricsExporterSinkSpec{
					Type: v1alpha1.PrometheusSinkType, MetricsSinkSource: v1alpha1.MetricsSinkSource{
						PrometheusConfig: &v1alpha1.PrometheusConfig{ServiceRef: v1alpha1.ServiceRef{Endpoint: "test"}},
					}},
			},
		},
	}
}

func fakeLogsExporterSinkList() v1alpha1.LogsExporterSinkList {
	return v1alpha1.LogsExporterSinkList{
		Items: []v1alpha1.LogsExporterSink{
			{
				Spec: v1alpha1.LogsExporterSinkSpec{
					Type: v1alpha1.LokiSinkType, SinkSource: v1alpha1.SinkSource{
						LokiConfig: &v1alpha1.LokiConfig{ServiceRef: v1alpha1.ServiceRef{Endpoint: "test"}},
					}},
			},
			{
				Spec: v1alpha1.LogsExporterSinkSpec{
					Type: v1alpha1.S3SinkType, SinkSource: v1alpha1.SinkSource{
						LokiConfig: &v1alpha1.LokiConfig{ServiceRef: v1alpha1.ServiceRef{Endpoint: "test"}},
					}},
			},
		},
	}
}
