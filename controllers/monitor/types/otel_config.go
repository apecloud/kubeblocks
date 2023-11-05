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
	"strings"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"gopkg.in/yaml.v2"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/builder"
)

const (
	SystemMetricsCUEPattern = "receiver/metrics/system/%s.cue"

	MetricsPattern = "metrics/%s"
	LogsPattern    = "logs/%s"

	ExporterTplPattern  = "exporter/%s.cue"
	ReceiverCreatorType = "receiver_creator"
	ServicePath         = "service/service.cue"

	ExtensionPath = "extension/extensions.cue"

	MetricsInfraTplName = "receiver/metrics_creator_infra.cue"
	LogsInfraTplName    = "receiver/logs_creator_infra.cue"

	AppMetricsCreatorName = "receiver_creator/app"
	LogCreatorName        = "receiver_creator/logs"
	EngineTplPath         = "engine/engine_template.cue"

	BatchProcessorName        = "batch"
	MemoryProcessorName       = "memory_limiter"
	GlobalLabelsProcessorName = "resource"
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

func (cg *OteldConfigGenerater) GenerateOteldConfiguration(instance *OteldInstance, metricsExporterList []v1alpha1.MetricsExporterSink, logsExporterList []v1alpha1.LogsExporterSink, mode v1alpha1.Mode) (yaml.MapSlice, error) {
	var err error
	var cfg = yaml.MapSlice{}

	if instance == nil || instance.OTeld == nil {
		return nil, nil
	}
	if cg.cache != nil && cg.cache[mode] != nil {
		return cg.cache[mode], nil
	}
	if cfg, err = cg.appendExtentions(cfg); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendReceiver(cfg, instance); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendProcessor(cfg, instance); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendExporter(cfg, metricsExporterList, logsExporterList); err != nil {
		return nil, err
	}
	if cfg, err = cg.appendServices(cfg, instance); err != nil {
		return nil, err
	}

	cg.cache[mode] = cfg
	return cfg, nil
}

func (cg *OteldConfigGenerater) appendReceiver(cfg yaml.MapSlice, instance *OteldInstance) (yaml.MapSlice, error) {
	receiverSlice := yaml.MapSlice{}
	creatorSlice, err := newReceiverCreatorSlice(instance)
	if err != nil {
		return nil, err
	}
	receiverSlice = append(receiverSlice, creatorSlice...)

	systemLogSlice, err := newSystemLogSlice(instance)
	if err != nil {
		return nil, err
	}
	receiverSlice = append(receiverSlice, systemLogSlice...)
	return append(cfg, yaml.MapItem{Key: "receivers", Value: receiverSlice}), nil
}

func newSystemLogSlice(instance *OteldInstance) (yaml.MapSlice, error) {
	systemLogSlice := yaml.MapSlice{}
	for _, pipeline := range instance.LogPipeline {
		for name, receiver := range pipeline.ReceiverMap {
			valMap := map[string]any{}
			if err := yaml.Unmarshal([]byte(receiver.Parameter), &valMap); err != nil {
				return nil, err
			}
			receiver, err := buildSliceFromCUE(fmt.Sprintf("receiver/logs/%s.cue", name), valMap)
			if err != nil {
				return nil, err
			}
			systemLogSlice = append(systemLogSlice, receiver...)
		}
	}
	return systemLogSlice, nil
}

func newReceiverCreatorSlice(instance *OteldInstance) (yaml.MapSlice, error) {
	creators := yaml.MapSlice{}

	for _, pipeline := range instance.MetricsPipeline {
		creator, err := newMetricsReceiverCreator(pipeline.GetReceiverName(), pipeline.ReceiverMap)
		if err != nil {
			return nil, err
		}
		creators = append(creators, creator)
	}

	appMetricsCreator, err := newAppReceiverCreator()
	if err != nil {
		return nil, err
	}
	creators = append(creators, appMetricsCreator)

	logsInfraName := LogsInfraTplName
	logCreatorSlice, err := buildSliceFromCUE(logsInfraName, map[string]any{})
	if err != nil {
		return nil, err
	}
	creators = append(creators, yaml.MapItem{Key: LogCreatorName, Value: logCreatorSlice})
	return creators, nil
}

func newAppReceiverCreator() (yaml.MapItem, error) {
	infraTplName := MetricsInfraTplName
	metricsSlice, err := buildSliceFromCUE(infraTplName, map[string]any{})
	if err != nil {
		return yaml.MapItem{}, err
	}
	receiverSlice := yaml.MapSlice{}

	appMetricsFileNames, err := builder.GetSubDirFileNames("receiver/metrics/app")
	if err != nil {
		return yaml.MapItem{}, err
	}
	for _, fileName := range appMetricsFileNames {
		tplName := fmt.Sprintf("receiver/metrics/app/%s", fileName)
		receivers, err := buildSliceFromCUE(tplName, map[string]any{})
		if err != nil {
			return yaml.MapItem{}, err
		}
		receiverSlice = append(receiverSlice, receivers...)
	}
	metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})
	return yaml.MapItem{Key: AppMetricsCreatorName, Value: metricsSlice}, nil
}

func newMetricsReceiverCreator(name string, receiverMap map[string]Receiver) (yaml.MapItem, error) {
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

	if !isReceiverCreatorType(name) {
		if len(receiverSlice) == 1 {
			return receiverSlice[0], nil
		}
		return yaml.MapItem{}, core.MakeError("receiver creator name[%s] is invalid", name)
	}

	// build receiver creator attribute template
	metricsSlice := yaml.MapSlice{yaml.MapItem{Key: "receivers", Value: receiverSlice}}
	slice, err := buildSliceFromCUE(MetricsInfraTplName, map[string]any{})
	if err != nil {
		return yaml.MapItem{}, err
	}
	metricsSlice = append(metricsSlice, slice...)
	return yaml.MapItem{Key: name, Value: metricsSlice}, nil
}

func isReceiverCreatorType(name string) bool {
	return strings.HasPrefix(name, ReceiverCreatorType+"/")
}

func (cg *OteldConfigGenerater) appendExporter(cfg yaml.MapSlice, metricsExporters []v1alpha1.MetricsExporterSink, logsExporter []v1alpha1.LogsExporterSink) (yaml.MapSlice, error) {
	exporterSlice := yaml.MapSlice{}
	for _, exporter := range metricsExporters {
		switch exporter.Spec.Type {
		case v1alpha1.PrometheusSinkType:
			exporterConfig := exporter.Spec.MetricsSinkSource.PrometheusConfig
			valueMap := map[string]any{"name": exporter.Name}
			if exporterConfig.Endpoint != "" {
				valueMap["endpoint"] = exporterConfig.Endpoint
			}
			tplName := fmt.Sprintf(ExporterTplPattern, v1alpha1.PrometheusSinkType)
			metricsExporter, err := buildSliceFromCUE(tplName, valueMap)
			if err != nil {
				return nil, err
			}

			exporterSlice = append(exporterSlice, metricsExporter...)
		case v1alpha1.PrometheusRemoteWriteSinkType:
			exporterConfig := exporter.Spec.MetricsSinkSource.PrometheusRemoteWriteConfig
			valueMap := map[string]any{"name": exporter.Name}
			if exporterConfig.Endpoint != "" {
				valueMap["endpoint"] = exporterConfig.Endpoint
			}
			tplName := fmt.Sprintf(ExporterTplPattern, v1alpha1.PrometheusRemoteWriteSinkType)
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
			valueMap := map[string]any{"name": exporter.Name}
			if exporterConfig.Endpoint != "" {
				valueMap["endpoint"] = exporterConfig.Endpoint
			}
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

func (cg *OteldConfigGenerater) appendProcessor(cfg yaml.MapSlice, instance *OteldInstance) (yaml.MapSlice, error) {
	processorSlice := yaml.MapSlice{}
	oteld := instance.OTeld
	if oteld.Spec.Batch.Enabled == true {
		batchConfig := fromBatchConfig(oteld.Spec.Batch.Config)
		batchProcessor, err := buildSliceFromCUE("processor/batch.cue", batchConfig)
		if err != nil {
			return nil, err
		}
		processorSlice = append(processorSlice, batchProcessor...)
	}
	if oteld.Spec.MemoryLimiter.Enabled == true {
		memoryLimiterConfig := fromMemoryLimiterConfig(oteld.Spec.MemoryLimiter.Config)
		memoryProcessor, err := buildSliceFromCUE("processor/memory_limiter.cue", memoryLimiterConfig)
		if err != nil {
			return nil, err
		}
		processorSlice = append(processorSlice, memoryProcessor...)
	}
	if len(oteld.Spec.GlobalLabels) > 0 {
		resourceConfig := fromGlobalLabels(oteld.Spec.GlobalLabels)
		globalLabelsProcessor, err := buildSliceFromCUE("processor/resource.cue", resourceConfig)
		if err != nil {
			return nil, err
		}
		processorSlice = append(processorSlice, globalLabelsProcessor...)
	}
	return append(cfg, yaml.MapItem{Key: "processors", Value: processorSlice}), nil
}

func fromGlobalLabels(labels map[string]string) map[string]any {
	return map[string]any{"global_labels": labels}
}

func fromMemoryLimiterConfig(limiter *v1alpha1.MemoryLimiterConfig) map[string]any {
	valMap := map[string]any{}
	if limiter == nil {
		return valMap
	}
	valMap["limit_mib"] = limiter.MemoryLimit
	valMap["spike_limit_mib"] = limiter.MemorySpikeLimit
	valMap["check_interval"] = limiter.CheckInterval
	return valMap
}

func fromBatchConfig(batch *v1alpha1.BatchConfig) map[string]any {
	valMap := map[string]any{}
	if batch == nil {
		return valMap
	}
	valMap["timeout"] = batch.Timeout
	valMap["send_batch_size"] = batch.SendBatchSize
	valMap["send_batch_max_size"] = batch.SendBatchMaxSize
	return valMap
}

func (cg *OteldConfigGenerater) appendServices(cfg yaml.MapSlice, instance *OteldInstance) (yaml.MapSlice, error) {
	serviceSlice := yaml.MapSlice{}
	piplneItem := cg.buildPipelineItem(instance)
	serviceSlice = append(serviceSlice, piplneItem)
	valmap := buildServiceValMap(instance)
	extensionSlice, err := buildSliceFromCUE(ServicePath, valmap)
	if err != nil {
		return nil, err
	}
	serviceSlice = append(serviceSlice, extensionSlice...)
	return append(cfg, yaml.MapItem{Key: "service", Value: serviceSlice}), nil
}

func buildServiceValMap(instance *OteldInstance) map[string]any {
	valMap := map[string]any{}
	if instance.OTeld.Spec.LogsLevel != "" {
		valMap["log_level"] = instance.OTeld.Spec.LogsLevel
	}
	if instance.OTeld.Spec.MetricsPort != 0 {
		valMap["metrics_port"] = instance.OTeld.Spec.MetricsPort
	}
	if len(instance.OTeld.Spec.GlobalLabels) != 0 {
		valMap["global_labels"] = instance.OTeld.Spec.GlobalLabels
	}
	return valMap
}

func (cg *OteldConfigGenerater) buildPipelineItem(instance *OteldInstance) yaml.MapItem {
	pipeline := yaml.MapSlice{}

	for _, mPipeline := range instance.MetricsPipeline {
		metricsSlice := yaml.MapSlice{}
		receiverSlice := []string{}
		receiverSlice = append(receiverSlice, mPipeline.GetReceiverName())
		metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})

		processorSlice := []string{}
		for name := range mPipeline.ProcessorMap {
			processorSlice = append(processorSlice, name)
		}
		metricsSlice = append(metricsSlice, yaml.MapItem{Key: "processors", Value: processorSlice})

		exporterSlice := []string{}
		for name := range mPipeline.ExporterMap {
			exporterSlice = append(exporterSlice, name)
		}
		metricsSlice = append(metricsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})
		if len(metricsSlice) > 0 {
			pipeline = append(pipeline, yaml.MapItem{Key: fmt.Sprintf(MetricsPattern, mPipeline.Name), Value: metricsSlice})
		}
	}

	for _, lPipeline := range instance.LogPipeline {
		logsSlice := yaml.MapSlice{}
		receiverSlice := []string{}
		for receiverName := range lPipeline.ReceiverMap {
			receiverSlice = append(receiverSlice, fmt.Sprintf("filelog/%s", receiverName))
		}
		logsSlice = append(logsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})

		processorSlice := []string{}
		for name := range lPipeline.ProcessorMap {
			processorSlice = append(processorSlice, name)
		}
		logsSlice = append(logsSlice, yaml.MapItem{Key: "processors", Value: processorSlice})

		exporterSlice := []string{}
		for name := range lPipeline.ExporterMap {
			exporterSlice = append(exporterSlice, name)
		}
		logsSlice = append(logsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})
		if len(logsSlice) > 0 {
			pipeline = append(pipeline, yaml.MapItem{Key: fmt.Sprintf(LogsPattern, lPipeline.Name), Value: logsSlice})
		}
	}

	if instance.AppMetricsPipelien != nil && instance.AppMetricsPipelien.Name != "" {
		metricsPipeline := instance.AppMetricsPipelien

		metricsSlice := yaml.MapSlice{}
		var receiverSlice []string
		receiverSlice = append(receiverSlice, metricsPipeline.Name)
		metricsSlice = append(metricsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})

		processorSlice := []string{}
		for name := range metricsPipeline.ProcessorMap {
			processorSlice = append(processorSlice, name)
		}
		metricsSlice = append(metricsSlice, yaml.MapItem{Key: "processors", Value: processorSlice})

		var exporterSlice []string
		for exporter := range metricsPipeline.ExporterMap {
			exporterSlice = append(exporterSlice, exporter)
		}
		metricsSlice = append(metricsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})

		pipeline = append(pipeline, yaml.MapItem{Key: "metrics/app", Value: metricsSlice})
	}

	if instance.AppLogsPipeline != nil && instance.AppLogsPipeline.Name != "" {
		logPipeline := instance.AppLogsPipeline

		logsSlice := yaml.MapSlice{}
		var receiverSlice []string
		receiverSlice = append(receiverSlice, logPipeline.Name)
		logsSlice = append(logsSlice, yaml.MapItem{Key: "receivers", Value: receiverSlice})

		processorSlice := []string{}
		for name := range logPipeline.ProcessorMap {
			processorSlice = append(processorSlice, name)
		}
		logsSlice = append(logsSlice, yaml.MapItem{Key: "processors", Value: processorSlice})

		var exporterSlice []string
		for exporter := range logPipeline.ExporterMap {
			exporterSlice = append(exporterSlice, exporter)
		}
		logsSlice = append(logsSlice, yaml.MapItem{Key: "exporters", Value: exporterSlice})

		if len(logsSlice) > 0 {
			pipeline = append(pipeline, yaml.MapItem{Key: "logs", Value: logsSlice})
		}
	}
	return yaml.MapItem{Key: "pipelines", Value: pipeline}
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

func (cg *OteldConfigGenerater) GenerateEngineConfiguration(instance *OteldInstance, mode v1alpha1.Mode) (yaml.MapSlice, error) {
	var err error
	var valMap map[string]any
	var cfg = yaml.MapSlice{}

	if instance == nil || instance.OTeld == nil {
		return nil, nil
	}
	if cg.engineCache != nil && cg.engineCache[mode] != nil {
		return cg.engineCache[mode], nil
	}

	valMap = buildEngineInfraValMap(instance)

	infraSlice, err := buildSliceFromCUE("engine/infra.cue", valMap)
	if err != nil {
		return nil, err
	}
	cfg = append(cfg, infraSlice...)
	defaultConfigSlice := yaml.MapSlice{}
	for _, clusterDatasource := range instance.AppDataSources {
		scrapConfigs, err := fromCollectorDataSource(clusterDatasource.Name, clusterDatasource.Spec, instance.Cli, instance.Ctx, clusterDatasource.Namespace)
		if err != nil {
			return nil, err
		}
		for _, config := range scrapConfigs {
			configSlice, err := buildSliceFromCUE(EngineTplPath, config)
			if err != nil {
				return nil, err
			}
			defaultConfigSlice = append(defaultConfigSlice, configSlice...)
		}
		// for _, componentDatasource := range clusterDatasource.Spec.Components {
		//	for _, datasource := range componentDatasource.Containers {
		//		valMap = buildEngineValMap(clusterDatasource.Spec.ClusterName, componentDatasource.ComponentName, datasource)
		//		configSlice, err := buildSliceFromCUE(EngineTplPath, valMap)
		//		if err != nil {
		//			return nil, err
		//		}
		//		defaultConfigSlice = append(defaultConfigSlice, configSlice...)
		//	}
		// }
	}
	cfg = append(cfg, yaml.MapItem{Key: "scrape_configs", Value: defaultConfigSlice})
	cg.engineCache[mode] = cfg
	return cfg, nil
}

func buildEngineInfraValMap(instance *OteldInstance) map[string]any {
	return map[string]any{
		"collection_interval": instance.OTeld.Spec.CollectionInterval,
	}
}
