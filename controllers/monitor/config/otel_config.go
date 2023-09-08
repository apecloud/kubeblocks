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
	"time"

	"github.com/apecloud/kubeblocks/controllers/monitor/types"

	"gopkg.in/yaml.v2"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
)

type SimpleReceiver struct {
	Name string
	v1alpha1.ExporterRef
}

type Pipline struct {
	ReceiverList []SimpleReceiver
	ProcessorMap map[string]bool
	ExporterMap  map[string]bool
}

type OteldConfigGenerater struct {
	metricsPipline *Pipline
	logsPipline    *Pipline

	config *types.Config
}

func NewConfigGenerator(config *types.Config) (*OteldConfigGenerater, error) {
	metricsPipline := &Pipline{
		ReceiverList: []SimpleReceiver{},
		ProcessorMap: map[string]bool{},
		ExporterMap:  map[string]bool{},
	}
	logsPipline := &Pipline{
		ReceiverList: []SimpleReceiver{},
		ProcessorMap: map[string]bool{},
		ExporterMap:  map[string]bool{},
	}
	return &OteldConfigGenerater{config: config, metricsPipline: metricsPipline, logsPipline: logsPipline}, nil
}

func (cg *OteldConfigGenerater) GenerateOteldConfiguration(datasourceList *v1alpha1.CollectorDataSourceList, metricsExporterList *v1alpha1.MetricsExporterSinkList, logsExporterList *v1alpha1.LogsExporterSinkList) ([]byte, error) {
	cfg := yaml.MapSlice{}
	cfg = cg.appendReceiver(cfg, datasourceList)
	cfg = cg.appendProcessor(cfg)
	cfg = cg.appendExporter(cfg, metricsExporterList, logsExporterList)
	cfg = cg.appendServices(cfg, datasourceList, metricsExporterList, logsExporterList)

	return yaml.Marshal(cfg)
}

func (cg *OteldConfigGenerater) appendReceiver(cfg yaml.MapSlice, datasourceList *v1alpha1.CollectorDataSourceList) yaml.MapSlice {
	receiverItems := yaml.MapSlice{}
	for _, datasource := range datasourceList.Items {
		switch datasource.Spec.Type {
		case v1alpha1.MetricsDatasourceType:
			// TODO: add metrics receiver
		case v1alpha1.LogsDataSourceType:
			// TODO: add logs receiver
		default:
			continue
		}
	}

	if cg.config.Datasource.MetricsDatasource != nil {
		receiverItems = append(receiverItems, cg.buildMetricsDatasourceSlice(cg.config.Datasource.MetricsDatasource)...)
	}

	if cg.config.Datasource.LogDatasource != nil {
		receiverItems = append(receiverItems, cg.buildLogsDatasourceSlice(receiverItems, cg.config.Datasource.LogDatasource)...)
	}
	return append(cfg, yaml.MapItem{Key: "receivers", Value: receiverItems})
}

func (cg *OteldConfigGenerater) appendExporter(cfg yaml.MapSlice, metricsExporters *v1alpha1.MetricsExporterSinkList, logsExporter *v1alpha1.LogsExporterSinkList) yaml.MapSlice {
	exporterItems := yaml.MapSlice{}
	for _, exporter := range metricsExporters.Items {
		var metricsExporterConfig yaml.MapSlice
		switch exporter.Spec.Type {
		case v1alpha1.PrometheusSinkType:
			exporterConfig := exporter.Spec.MetricsSinkSource.PrometheusConfig
			metricsExporterConfig = append(metricsExporterConfig,
				yaml.MapItem{Key: "endpoint", Value: exporterConfig.Endpoint},
				yaml.MapItem{Key: "send_timestamps", Value: false},
				yaml.MapItem{Key: "metric_expiration", Value: (20 * time.Second).String()},
				yaml.MapItem{Key: "enable_open_metrics", Value: false},
				yaml.MapItem{
					Key:   "resource_to_telemetry_conversion",
					Value: yaml.MapSlice{yaml.MapItem{Key: "enabled", Value: true}},
				},
			)
		default:
			continue
		}
		exporterItems = append(exporterItems, yaml.MapItem{Key: exporter.Spec.Type, Value: metricsExporterConfig})
	}

	// for _, exporter := range logsExporter.Items {
	// 	 var logsExporterConfig yaml.MapSlice
	//	 switch exporter.Spec.Type {
	//	 case v1alpha1.LokiSinkType:
	//		exporterConfig := exporter.Spec.LokiConfig
	//		logsExporterConfig = append(logsExporterConfig,
	//			yaml.MapItem{Key: "timeout", Value: (20 * time.Second).String()},
	//		)
	//		if exporterConfig.RetryPolicyOnFailure != nil {
	//			retryConfig := yaml.MapSlice{}
	//			retryConfig = append(retryConfig, yaml.MapItem{Key: "enabled", Value: true})
	//			retryConfig = append(retryConfig, yaml.MapItem{Key: "enabled"})
	//	 		logsExporterConfig = append(logsExporterConfig,
	//	 			yaml.MapItem{Key: "retry_on_failure", Value: (20 * time.Second).String()},
	// 			)
	//	 	}
	//	 default:
	// 		continue
	//	 }
	//	 exporterItems = append(exporterItems, yaml.MapItem{Key: exporter.Name, Value: logsExporterConfig})
	// }
	return append(cfg, yaml.MapItem{Key: "exporters", Value: exporterItems})
}

func (cg *OteldConfigGenerater) appendProcessor(cfg yaml.MapSlice) yaml.MapSlice {
	// TODO
	return cfg
}

func (cg *OteldConfigGenerater) appendServices(cfg yaml.MapSlice,
	datasourceList *v1alpha1.CollectorDataSourceList,
	metricsExporterList *v1alpha1.MetricsExporterSinkList,
	logsExporterList *v1alpha1.LogsExporterSinkList,
) yaml.MapSlice {
	cg.buildPipline(datasourceList, metricsExporterList, logsExporterList)
	serviceSlice := yaml.MapSlice{}
	piplneItem := cg.buildPiplineItem()
	serviceSlice = append(serviceSlice, piplneItem)
	return append(cfg, yaml.MapItem{Key: "service", Value: serviceSlice})
}

func (cg *OteldConfigGenerater) buildMetricsDatasourceSlice(datasources *types.MetricsDatasource) yaml.MapSlice {
	collectionInterval := cg.config.CollectionInterval
	if cg.config.Datasource.MetricsDatasource.CollectionInterval != nil {
		collectionInterval = *cg.config.Datasource.MetricsDatasource.CollectionInterval
	}

	receiverConfigs := yaml.MapSlice{}
	if datasources.K8sNodeConfig != nil || datasources.K8sNodeConfig.Enabled {
		k8sNodeItem := yaml.MapItem{
			Key: "apecloudnode",
			Value: yaml.MapSlice{
				yaml.MapItem{Key: "collection_interval", Value: collectionInterval},
			},
		}
		receiverConfigs = append(receiverConfigs, k8sNodeItem)
	}
	if datasources.K8sClusterConfig != nil || datasources.K8sClusterConfig.Enabled {
		k8sClusterItem := yaml.MapItem{
			Key: "k8s_cluster",
			Value: yaml.MapSlice{
				yaml.MapItem{Key: "collection_interval", Value: collectionInterval},
			},
		}
		receiverConfigs = append(receiverConfigs, k8sClusterItem)
	}
	if datasources.KubeletStateConfig != nil || datasources.KubeletStateConfig.Enabled {
		k8sStateSlice := yaml.MapSlice{}
		if datasources.KubeletStateConfig.MetricGroups != nil {
			k8sStateSlice = append(k8sStateSlice, yaml.MapItem{Key: "metric_groups", Value: datasources.KubeletStateConfig.MetricGroups})
		}
		k8sStateSlice = append(k8sStateSlice, yaml.MapItem{Key: "auth_type", Value: cg.config.AuthType})
		k8sStateSlice = append(k8sStateSlice, yaml.MapItem{Key: "collection_interval", Value: collectionInterval})
		k8sStateSlice = append(k8sStateSlice, yaml.MapItem{Key: "endpoint", Value: "${env:NODE_NAME}:10250"})
		k8sStateItem := yaml.MapItem{
			Key:   "apecloudkubeletstats",
			Value: k8sStateSlice,
		}
		receiverConfigs = append(receiverConfigs, k8sStateItem)

	}
	return receiverConfigs
}

func (cg *OteldConfigGenerater) buildLogsDatasourceSlice(receiverItems yaml.MapSlice, datasources *types.LogsDatasource) yaml.MapSlice {
	return receiverItems
}

func (cg *OteldConfigGenerater) buildPiplineItem() yaml.MapItem {

	pipline := yaml.MapSlice{}

	if cg.metricsPipline != nil {
		metricsSlice := yaml.MapSlice{}
		receiverSlice, exporterSlice := []string{}, []string{}
		for _, receiver := range cg.metricsPipline.ReceiverList {
			receiverSlice = append(receiverSlice, receiver.Name)
			for _, exporterName := range receiver.ExporterRef.ExporterNames {
				if cg.metricsPipline.ExporterMap[exporterName] {
					exporterSlice = append(exporterSlice, exporterName)
				}
			}
		}
		if len(receiverSlice) > 0 {
			metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})
		}
		if len(exporterSlice) > 0 {
			metricsSlice = append(metricsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})
		}
		pipline = append(pipline, yaml.MapItem{Key: "metrics", Value: metricsSlice})
	}
	return yaml.MapItem{Key: "pipelines", Value: pipline}
}

func (cg *OteldConfigGenerater) buildPipline(datasourceList *v1alpha1.CollectorDataSourceList, metricsExporterList *v1alpha1.MetricsExporterSinkList, logsExporterList *v1alpha1.LogsExporterSinkList) {
	for _, mExporter := range metricsExporterList.Items {
		cg.metricsPipline.ExporterMap[mExporter.Name] = true
	}
	for _, lExporter := range logsExporterList.Items {
		cg.logsPipline.ExporterMap[lExporter.Name] = true
	}
	for _, datasource := range datasourceList.Items {
		switch datasource.Spec.Type {
		case v1alpha1.MetricsDatasourceType:
			cg.addMetricsPiplineReceiver(datasource.Name, datasource.Spec.ExporterNames)
		case v1alpha1.LogsDataSourceType:
			cg.addLogsPiplineReceiver(datasource.Name, datasource.Spec.ExporterNames)
		}
	}

	datasource := cg.config.Datasource.MetricsDatasource
	if datasource != nil {
		if datasource.KubeletStateConfig != nil {
			cg.addMetricsPiplineReceiver("apecloudkubeletstats", datasource.ExporterNames)
		}
		if datasource.K8sClusterConfig != nil {
			cg.addMetricsPiplineReceiver("k8scluster", datasource.ExporterNames)
		}
		if datasource.K8sNodeConfig != nil {
			cg.addMetricsPiplineReceiver("apecloudk8snode", datasource.ExporterNames)
		}
	}

	// if cg.config.Datasource.LogDatasource != nil {
	// 	cg.buildLogsPipline(cg.config.Datasource.LogDatasource)
	// }
}

func (cg *OteldConfigGenerater) addMetricsPiplineReceiver(name string, exporterRef []string) {
	receiver := &SimpleReceiver{Name: name, ExporterRef: v1alpha1.ExporterRef{ExporterNames: exporterRef}}
	cg.metricsPipline.ReceiverList = append(cg.metricsPipline.ReceiverList, *receiver)
}

func (cg *OteldConfigGenerater) addLogsPiplineReceiver(name string, exporterRef []string) {
	receiver := SimpleReceiver{Name: name, ExporterRef: v1alpha1.ExporterRef{ExporterNames: exporterRef}}
	cg.logsPipline.ReceiverList = append(cg.logsPipline.ReceiverList, receiver)
}
