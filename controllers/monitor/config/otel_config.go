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
	"fmt"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
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

func NewConfigGenerator(config *types.Config) *OteldConfigGenerater {
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
	return &OteldConfigGenerater{config: config, metricsPipline: metricsPipline, logsPipline: logsPipline}
}

func (cg *OteldConfigGenerater) GenerateOteldConfiguration(datasourceList *v1alpha1.CollectorDataSourceList, metricsExporterList *v1alpha1.MetricsExporterSinkList, logsExporterList *v1alpha1.LogsExporterSinkList) yaml.MapSlice {
	cfg := yaml.MapSlice{}
	cfg = cg.appendExtentions(cfg)
	cfg = cg.appendReceiver(cfg, datasourceList)
	cfg = cg.appendProcessor(cfg)
	cfg = cg.appendExporter(cfg, metricsExporterList, logsExporterList)
	cfg = cg.appendServices(cfg, datasourceList, metricsExporterList, logsExporterList)

	return cfg
}

func (cg *OteldConfigGenerater) appendReceiver(cfg yaml.MapSlice, datasourceList *v1alpha1.CollectorDataSourceList) yaml.MapSlice {
	receiverSlice := yaml.MapSlice{}
	receiverSlice = append(receiverSlice, newReceiverCreator(v1alpha1.MetricsDatasourceType, datasourceList.Items))
	receiverSlice = append(receiverSlice, newReceiverCreator(v1alpha1.LogsDataSourceType, datasourceList.Items))
	return append(cfg, yaml.MapItem{Key: "receivers", Value: receiverSlice})
}

func newReceiverCreator(datasourceType v1alpha1.DataSourceType, dataSources []v1alpha1.CollectorDataSource) yaml.MapItem {
	if len(dataSources) == 0 {
		return yaml.MapItem{}
	}
	creator := yaml.MapSlice{}
	creator = append(creator, yaml.MapItem{Key: "watch_observers", Value: []string{"apecloud_engine_observer"}})
	receiverSlice := yaml.MapSlice{}
	for _, dataSource := range dataSources {
		if dataSource.Spec.Type != datasourceType {
			continue
		}
		for _, data := range dataSource.Spec.DataSourceList {
			tplName := fmt.Sprintf("receiver/%s/%s.cue", datasourceType, data.Name)
			receiverSlice = append(receiverSlice, buildSliceFromCUE(tplName)...)
		}
	}
	creator = append(creator, yaml.MapItem{Key: "receivers", Value: receiverSlice})
	return yaml.MapItem{Key: fmt.Sprintf("receiver_creator/%s", datasourceType), Value: creator}
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
	// TODO: add logs exporter
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
	extensionItem := yaml.MapItem{Key: "extensions", Value: []string{"runtime_container", "apecloud_engine_observer"}}
	serviceSlice = append(serviceSlice, extensionItem)
	return append(cfg, yaml.MapItem{Key: "service", Value: serviceSlice})
}

func (cg *OteldConfigGenerater) buildLogsDatasourceSlice(receiverItems yaml.MapSlice, datasources *types.LogsDatasource) yaml.MapSlice {
	return receiverItems
}

func (cg *OteldConfigGenerater) buildPiplineItem() yaml.MapItem {

	pipline := yaml.MapSlice{}

	if cg.metricsPipline != nil {
		metricsSlice := yaml.MapSlice{}
		receiverMap, exporterMap := map[string]bool{}, map[string]bool{}
		for _, receiver := range cg.metricsPipline.ReceiverList {
			receiverMap[receiver.Name] = true
			for _, exporterName := range receiver.ExporterRef.ExporterNames {
				if cg.metricsPipline.ExporterMap[exporterName] {
					exporterMap[exporterName] = true
				}
			}
		}
		if len(receiverMap) > 0 {
			receiverSlice := []string{}
			for receiverName := range receiverMap {
				receiverSlice = append(receiverSlice, receiverName)
			}
			metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})
		}
		if len(exporterMap) > 0 {
			exporterSlice := []string{}
			for exporterName := range exporterMap {
				exporterSlice = append(exporterSlice, exporterName)
			}
			metricsSlice = append(metricsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})
		}
		pipline = append(pipline, yaml.MapItem{Key: "metrics", Value: metricsSlice})
	}
	return yaml.MapItem{Key: "pipelines", Value: pipline}
}

func (cg *OteldConfigGenerater) buildPipline(datasourceList *v1alpha1.CollectorDataSourceList, metricsExporterList *v1alpha1.MetricsExporterSinkList, logsExporterList *v1alpha1.LogsExporterSinkList) {
	for _, mExporter := range metricsExporterList.Items {
		cg.metricsPipline.ExporterMap[string(mExporter.Spec.Type)] = true
	}
	for _, lExporter := range logsExporterList.Items {
		cg.logsPipline.ExporterMap[string(lExporter.Spec.Type)] = true
	}
	for _, datasource := range datasourceList.Items {
		switch datasource.Spec.Type {
		case v1alpha1.MetricsDatasourceType:
			cg.addMetricsPiplineFromDataSource(datasource)
		case v1alpha1.LogsDataSourceType:
			cg.addLogsPiplineFromDataSource(datasource)
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

func (cg *OteldConfigGenerater) addMetricsPiplineFromDataSource(datasource v1alpha1.CollectorDataSource) {
	cg.addMetricsPiplineReceiver(fmt.Sprintf("receiver_creator/%s", datasource.Spec.Type), datasource.Spec.ExporterNames)
}

func (cg *OteldConfigGenerater) addLogsPiplineFromDataSource(datasource v1alpha1.CollectorDataSource) {
	cg.addLogsPiplineReceiver(fmt.Sprintf("receiver_creator/%s", datasource.Spec.Type), datasource.Spec.ExporterNames)
}

func (cg *OteldConfigGenerater) appendExtentions(cfg yaml.MapSlice) yaml.MapSlice {
	extensionSlice := yaml.MapSlice{}
	extensionSlice = append(extensionSlice, buildSliceFromCUE(newExtensionTplPath("apecloud_engine_observer"))...)
	extensionSlice = append(extensionSlice, buildSliceFromCUE(newExtensionTplPath("runtime_container"))...)
	return append(cfg, yaml.MapItem{Key: "extensions", Value: extensionSlice})
}

func newExtensionTplPath(name string) string {
	return fmt.Sprintf("extension/%s.cue", name)
}

func buildSliceFromCUE(tplName string) yaml.MapSlice {
	bytes, err := buildFromCUEForOTel(tplName, map[string]any{}, "output")
	if err != nil {
		return nil
	}
	extensionSlice := yaml.MapSlice{}
	err = yaml.Unmarshal(bytes, &extensionSlice)
	if err != nil {
		return nil
	}
	return extensionSlice
}
