/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

# This file is part of KubeBlocks project

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
	"fmt"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/builder"
	"gopkg.in/yaml.v2"
)

const (
	SystemMetricsCUEPattern = "receiver/metrics/system/%s.cue"

	ExporterTplPattern  = "exporter/%s.cue"
	ReceiverNamePattern = "receiver_creator/%s"
	ServicePath         = "service/service.cue"

	ExtensionPath = "extension/extensions.cue"

	MetricsInfraTplName = "receiver/metrics_creator_infra.cue"
	LogsInfraTplName    = "receiver/logs_creator_infra.cue"

	logCreatorName = "receiver_creator/logs"
	engineTplPath  = "engine/engine_template.cue"
)

type OteldConfigGenerater struct {
	cache map[v1alpha1.Mode]yaml.MapSlice

	engineCache map[v1alpha1.Mode]yaml.MapSlice
}

func NewConfigGenerator() *OteldConfigGenerater {
	return &OteldConfigGenerater{
		cache:       map[v1alpha1.Mode]yaml.MapSlice{},
		engineCache: map[v1alpha1.Mode]yaml.MapSlice{},
	}
}

func (cg *OteldConfigGenerater) GenerateOteldConfiguration(instance *OteldInstance, metricsExporterList []v1alpha1.MetricsExporterSink, logsExporterList []v1alpha1.LogsExporterSink) (yaml.MapSlice, error) {
	var err error
	var cfg = yaml.MapSlice{}

	if instance == nil || instance.Oteld == nil {
		return nil, nil
	}
	if cg.cache != nil && cg.cache[instance.Oteld.Spec.Mode] != nil {
		return cg.cache[instance.Oteld.Spec.Mode], nil
	}
	if cfg, err = cg.appendExtentions(cfg); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendReceiver(cfg, instance); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendProcessor(cfg); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendExporter(cfg, metricsExporterList, logsExporterList); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendServices(cfg, instance); err != nil {
		return nil, err
	}

	cg.cache[instance.Oteld.Spec.Mode] = cfg
	return cfg, nil
}

func (cg *OteldConfigGenerater) appendReceiver(cfg yaml.MapSlice, instance *OteldInstance) (yaml.MapSlice, error) {
	receiverSlice := yaml.MapSlice{}
	creatorSlice, err := newReceiverCreatorSlice(instance)
	if err != nil {
		return nil, err
	}
	receiverSlice = append(receiverSlice, creatorSlice...)
	return append(cfg, yaml.MapItem{Key: "receivers", Value: receiverSlice}), nil
}

func newReceiverCreatorSlice(instance *OteldInstance) (yaml.MapSlice, error) {
	creators := yaml.MapSlice{}

	for _, pipline := range instance.MetricsPipline {
		creator, err := newMetricsReceiverCreator(pipline.Name, pipline.ReceiverMap)
		if err != nil {
			return nil, err
		}
		creators = append(creators, creator)
	}

	logsInfraName := LogsInfraTplName
	logCreatorSlice, err := buildSliceFromCUE(logsInfraName, map[string]any{})
	if err != nil {
		return nil, err
	}
	creators = append(creators, yaml.MapItem{Key: logCreatorName, Value: logCreatorSlice})
	return creators, nil
}

func newMetricsReceiverCreator(name string, receiverMap map[string]Receiver) (yaml.MapItem, error) {
	infraTplName := MetricsInfraTplName
	metricsSlice, err := buildSliceFromCUE(infraTplName, map[string]any{})
	if err != nil {
		return yaml.MapItem{}, err
	}
	receiverSlice := yaml.MapSlice{}
	for name, params := range receiverMap {
		tplName := fmt.Sprintf(SystemMetricsCUEPattern, name)
		valueMap := map[string]any{}
		if params.CollectionInterval != "" {
			valueMap["collection_interval"] = params.CollectionInterval
		}
		builder.MergeValMapFromYamlStr(valueMap, params.Parameter)
		receivers, err := buildSliceFromCUE(tplName, valueMap)
		if err != nil {
			return yaml.MapItem{}, err
		}
		receiverSlice = append(receiverSlice, receivers...)
	}

	appMetricsFileNames, err := builder.GetSubDirFileNames("cue/receiver/metrics/app")
	for _, fileName := range appMetricsFileNames {
		tplName := fmt.Sprintf("receiver/metrics/app/%s", fileName)
		receivers, err := buildSliceFromCUE(tplName, map[string]any{})
		if err != nil {
			return yaml.MapItem{}, err
		}
		receiverSlice = append(receiverSlice, receivers...)
	}

	metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})
	return yaml.MapItem{Key: fmt.Sprintf(ReceiverNamePattern, name), Value: metricsSlice}, nil
}

func (cg *OteldConfigGenerater) appendExporter(cfg yaml.MapSlice, metricsExporters []v1alpha1.MetricsExporterSink, logsExporter []v1alpha1.LogsExporterSink) (yaml.MapSlice, error) {
	exporterSlice := yaml.MapSlice{}
	for _, exporter := range metricsExporters {
		switch exporter.Spec.Type {
		case v1alpha1.PrometheusSinkType:
			exporterConfig := exporter.Spec.MetricsSinkSource.PrometheusConfig
			valueMap := map[string]any{"endpoint": exporterConfig.Endpoint}
			tplName := fmt.Sprintf(ExporterTplPattern, v1alpha1.PrometheusSinkType)
			metricsExporter, err := buildSliceFromCUE(tplName, valueMap)
			if err != nil {
				return nil, err
			}

			exporterSlice = append(exporterSlice, metricsExporter...)
		default:
			continue
		}
	}
	for _, exporter := range logsExporter {
		switch exporter.Spec.Type {
		case v1alpha1.LokiSinkType:
			exporterConfig := exporter.Spec.LokiConfig
			valueMap := map[string]any{"endpoint": exporterConfig.Endpoint}
			tplName := fmt.Sprintf(ExporterTplPattern, v1alpha1.LokiSinkType)
			logsExporter, err := buildSliceFromCUE(tplName, valueMap)
			if err != nil {
				return nil, err
			}
			exporterSlice = append(exporterSlice, logsExporter...)
		default:
			continue
		}
	}
	return append(cfg, yaml.MapItem{Key: "exporters", Value: exporterSlice}), nil
}

func (cg *OteldConfigGenerater) appendProcessor(cfg yaml.MapSlice) (yaml.MapSlice, error) {
	processorSlice, err := buildSliceFromCUE("processor/processors.cue", map[string]any{})
	if err != nil {
		return nil, err
	}
	return append(cfg, yaml.MapItem{Key: "processors", Value: processorSlice}), nil
}

func (cg *OteldConfigGenerater) appendServices(cfg yaml.MapSlice, instance *OteldInstance) (yaml.MapSlice, error) {
	serviceSlice := yaml.MapSlice{}
	piplneItem := cg.buildPiplineItem(instance)
	serviceSlice = append(serviceSlice, piplneItem)
	extensionSlice, err := buildSliceFromCUE(ServicePath, map[string]any{})
	if err != nil {
		return nil, err
	}
	serviceSlice = append(serviceSlice, extensionSlice...)
	return append(cfg, yaml.MapItem{Key: "service", Value: serviceSlice}), nil
}

func (cg *OteldConfigGenerater) buildPiplineItem(instance *OteldInstance) yaml.MapItem {

	pipline := yaml.MapSlice{}

	if instance.MetricsPipline != nil {
		metricsSlice := yaml.MapSlice{}
		for _, mPipline := range instance.MetricsPipline {
			receiverSlice := []string{}
			receiverSlice = append(receiverSlice, fmt.Sprintf(ReceiverNamePattern, mPipline.Name))
			metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})
			exporterSlice := []string{}
			for name := range mPipline.ExporterMap {
				exporterSlice = append(exporterSlice, name)
			}
			metricsSlice = append(metricsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})
		}
		if len(metricsSlice) > 0 {
			pipline = append(pipline, yaml.MapItem{Key: "metrics", Value: metricsSlice})
		}
	}

	if instance.LogsPipline != nil {
		logsSlice := yaml.MapSlice{}
		for _, lPipline := range instance.LogsPipline {
			receiverSlice := []string{}
			receiverSlice = append(receiverSlice, fmt.Sprintf(ReceiverNamePattern, lPipline.Name))
			logsSlice = append(logsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})
			exporterSlice := []string{}
			for name := range lPipline.ExporterMap {
				exporterSlice = append(exporterSlice, name)
			}

			logsSlice = append(logsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})
		}
		if len(logsSlice) > 0 {
			pipline = append(pipline, yaml.MapItem{Key: "logs", Value: logsSlice})
		}
	}
	return yaml.MapItem{Key: "pipelines", Value: pipline}
}

func (cg *OteldConfigGenerater) appendExtentions(cfg yaml.MapSlice) (yaml.MapSlice, error) {
	extensionSlice := yaml.MapSlice{}
	extension, err := buildSliceFromCUE(ExtensionPath, map[string]any{})
	if err != nil {
		return nil, err
	}
	extensionSlice = append(extensionSlice, extension...)
	return append(cfg, extensionSlice...), nil
}

func buildSliceFromCUE(tplName string, valMap map[string]any) (yaml.MapSlice, error) {
	bytes, err := builder.BuildFromCUEForOTel(tplName, valMap, "output")
	if err != nil {
		return nil, err
	}
	extensionSlice := yaml.MapSlice{}
	err = yaml.Unmarshal(bytes, &extensionSlice)
	if err != nil {
		return nil, err
	}
	return extensionSlice, nil
}

func (cg *OteldConfigGenerater) GenerateEngineConfiguration(instance *OteldInstance) (yaml.MapSlice, error) {
	var err error
	var valMap map[string]any
	var cfg = yaml.MapSlice{}

	if instance == nil || instance.Oteld == nil {
		return nil, nil
	}
	if cg.engineCache != nil && cg.engineCache[instance.Oteld.Spec.Mode] != nil {
		return cg.engineCache[instance.Oteld.Spec.Mode], nil
	}

	valMap = buildEngineInfraValMap(instance)

	infraSlice, err := buildSliceFromCUE("engine/infra.cue", valMap)
	if err != nil {
		return nil, err
	}
	cfg = append(cfg, infraSlice...)
	defaultConfigSlice := yaml.MapSlice{}
	for _, dataSource := range instance.AppDataSources {
		valMap = buildEngineValMap(dataSource, instance)
		configSlice, err := buildSliceFromCUE(engineTplPath, valMap)
		if err != nil {
			return nil, err
		}
		defaultConfigSlice = append(defaultConfigSlice, configSlice...)
	}
	cfg = append(cfg, yaml.MapItem{Key: "default_config", Value: defaultConfigSlice})
	cg.engineCache[instance.Oteld.Spec.Mode] = cfg
	return cfg, nil
}

func buildEngineValMap(source v1alpha1.AppDataSource, instance *OteldInstance) map[string]any {
	collectorInterval := instance.Oteld.Spec.CollectionInterval
	if source.Spec.CollectionInterval != "" {
		collectorInterval = source.Spec.CollectionInterval
	}
	return map[string]any{
		"cluster_name":                source.Spec.ClusterName,
		"component_name":              source.Spec.ComponentName,
		"container_name":              source.Spec.ContainerName,
		"metrics.enabled":             source.Spec.Metrics.Enable,
		"metrics.collection_interval": collectorInterval,
		"logs.enabled":                source.Spec.Logs.Enable,
	}
}

func buildEngineInfraValMap(instance *OteldInstance) map[string]any {
	return map[string]any{
		"collection_interval": instance.Oteld.Spec.CollectionInterval,
	}
}
