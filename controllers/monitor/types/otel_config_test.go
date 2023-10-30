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

package types

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gopkg.in/yaml.v2"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
)

var _ = Describe("monitor_controller", func() {
	var (
		logsExporterList    v1alpha1.LogsExporterSinkList
		metricsExporterList v1alpha1.MetricsExporterSinkList
	)

	BeforeEach(func() {
		logsExporterList = fakeLogsExporterSinkList()
		metricsExporterList = fakeMetricsExporterSinkList()
	})

	It("should generate oteld correctly", func() {
		instance := fakeInstance()

		cg := NewConfigGenerator()
		cfg, err := cg.GenerateOteldConfiguration(instance, metricsExporterList.Items, logsExporterList.Items, v1alpha1.ModeDaemonSet)
		Expect(err).ShouldNot(HaveOccurred())
		bytes, err := yaml.Marshal(cfg)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(bytes) > 0).Should(BeTrue())

		engineCfg, err := cg.GenerateEngineConfiguration(instance, v1alpha1.ModeDaemonSet)
		Expect(err).Should(HaveOccurred())
		// TODO fix this ut
		_ = engineCfg
		// bytes, err = yaml.Marshal(engineCfg)
		// Expect(err).ShouldNot(HaveOccurred())
		// Expect(len(bytes) > 0).Should(BeTrue())
	})

})

func fakeInstance() *OteldInstance {
	return &OteldInstance{
		MetricsPipeline: []Pipeline{
			{
				Name: "metrics",
				ReceiverMap: map[string]Receiver{
					"kubeletstats": {},
					"node":         {},
				},
				ProcessorMap: map[string]bool{},
				ExporterMap: map[string]bool{
					"prometheus": true,
				},
			},
		},
		OTeld: &v1alpha1.OTeld{
			Spec: v1alpha1.OTeldSpec{
				Mode:               v1alpha1.ModeDaemonSet,
				CollectionInterval: "15s",
			},
		},
		AppDataSources: []v1alpha1.CollectorDataSource{
			{
				Spec: v1alpha1.CollectorDataSourceSpec{
					ClusterRef: "test",
					CollectorSpecs: []v1alpha1.CollectorSpec{{
						ComponentName: "test",
						ScrapeConfigs: []v1alpha1.ScrapeConfig{{
							ContainerName: "test",
							Metrics: &v1alpha1.MetricsCollector{
								CollectionInterval: "15s",
								MetricsSelector: []string{
									"test_metrics",
								},
							},
							Logs: &v1alpha1.LogsCollector{
								LogTypes: []string{
									"test",
								},
							},
						}},
					}},
				},
			},
		},
	}
}

// func fakeCollectorDataSourceList() v1alpha1.CollectorDataSourceList {
//	return v1alpha1.CollectorDataSourceList{
//		Items: []v1alpha1.CollectorDataSource{
//			{
//				Spec: v1alpha1.CollectorDataSourceSpec{
//					Type:        v1alpha1.MetricsDatasourceType,
//					ExporterRef: v1alpha1.ExporterRef{ExporterNames: []string{"prometheus"}},
//					DataSourceList: []v1alpha1.DataSource{
//						{Name: "apecloudmysql"},
//						{Name: "apecloudkubeletstats"},
//						{Name: "apecloudnode"},
//					},
//				},
//			},
//		},
//	}
// }

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
		Items: []v1alpha1.LogsExporterSink{},
	}
}
