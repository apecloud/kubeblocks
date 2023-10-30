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

package reconcile

import (
	"context"
	"errors"
	"fmt"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type oteldWrapper struct {
	*v1alpha1.OTeld
	cli client.Client
	ctx context.Context

	errs []error

	source           *v1alpha1.SystemDataSource
	userSource       *v1alpha1.CollectorDataSourceList
	instanceMap      map[v1alpha1.Mode]*types.OteldInstance
	logsExporters    *v1alpha1.LogsExporterSinkList
	metricsExporters *v1alpha1.MetricsExporterSinkList
}

const (
	k8sclusterPipeline = "api-service"
	k8snodePipeline    = "datasource-metrics"
	k8spodLogsPipeline = "podlogs"

	AppMetricsCreatorName = "receiver_creator/app"
	LogsCreatorName       = "receiver_creator/logs"
)

type collectType string

const (
	collectTypeMetrics collectType = "metrics"
	collectTypeLogs    collectType = "logs"
)

func (w *oteldWrapper) buildAPIServicePipeline() *oteldWrapper {
	if !w.source.EnabledK8sClusterExporter {
		return w
	}

	pipeline := w.createPipeline(v1alpha1.ModeDeployment, k8sclusterPipeline, collectTypeMetrics)
	pipeline.ReceiverMap[constant.APIServiceReceiverTPLName] = types.Receiver{
		CollectionInterval: w.source.CollectionInterval.String(),
	}
	w.buildProcessor(pipeline)
	w.buildMetricsExporter(pipeline)
	return w
}

func (w *oteldWrapper) buildK8sNodeStatesPipeline() *oteldWrapper {
	if !w.source.EnabledK8sNodeStatesMetrics {
		return w
	}

	pipeline := w.createPipeline(v1alpha1.ModeDaemonSet, k8snodePipeline, collectTypeMetrics)
	pipeline.ReceiverMap[constant.K8SNodeStatesReceiverTPLName] = types.Receiver{
		CollectionInterval: w.source.CollectionInterval.String(),
	}
	w.buildProcessor(pipeline)
	w.buildMetricsExporter(pipeline)
	return w
}

func (w *oteldWrapper) buildNodePipeline() *oteldWrapper {
	if !w.source.EnabledK8sNodeStatesMetrics {
		return w
	}

	pipeline := w.createPipeline(v1alpha1.ModeDaemonSet, k8snodePipeline, collectTypeMetrics)
	pipeline.ReceiverMap[constant.NodeExporterReceiverTPLName] = types.Receiver{
		CollectionInterval: w.source.CollectionInterval.String(),
	}
	w.buildProcessor(pipeline)
	w.buildMetricsExporter(pipeline)
	return w
}

func (w *oteldWrapper) buildPodLogsPipeline() *oteldWrapper {
	if !w.source.EnabledPodLogs {
		return w
	}

	pipeline := w.createPipeline(v1alpha1.ModeDaemonSet, k8spodLogsPipeline, collectTypeLogs)
	pipeline.ReceiverMap[constant.PodLogsReceiverTPLName] = types.Receiver{}
	w.buildProcessor(pipeline)
	w.buildLogsExporter(pipeline)
	return w
}

func (w *oteldWrapper) createPipeline(mode v1alpha1.Mode, name string, collectType collectType) *types.Pipline {
	var instance *types.OteldInstance

	if instance = w.instanceMap[mode]; instance == nil {
		instance = types.NewOteldInstance(w.OTeld, w.cli, w.ctx)
		w.instanceMap[mode] = instance
	}
	if instance.MetricsPipline == nil {
		instance.MetricsPipline = []types.Pipline{}
	}
	return foundOrCreatePipeline(instance, name, collectType)
}

func (w *oteldWrapper) buildProcessor(pipeline *types.Pipline) {
	if w.Spec.Batch.Enabled {
		pipeline.ProcessorMap[types.BatchProcessorName] = true
	}
	if w.Spec.MemoryLimiter.Enabled {
		pipeline.ProcessorMap[types.MemoryProcessorName] = true
	}
}

func (w *oteldWrapper) buildMetricsExporter(pipeline *types.Pipline) {
	for _, exporter := range w.metricsExporters.Items {
		if exporter.Name == w.source.MetricsExporterRef {
			pipeline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
			return
		}
	}
	w.errs = append(w.errs, cfgcore.MakeError("the metrics exporter[%s] relied on by %s was not found.", w.source.MetricsExporterRef, pipeline.Name))
}

func (w *oteldWrapper) buildLogsExporter(pipeline *types.Pipline) {
	for _, exporter := range w.logsExporters.Items {
		if exporter.Name == w.source.LogsExporterRef {
			pipeline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
			return
		}
	}
	w.errs = append(w.errs, cfgcore.MakeError("the logs exporter[%s] relied on by %s was not found.", w.source.LogsExporterRef, pipeline.Name))
}

func (w *oteldWrapper) appendAllMetricsExporter(pipeline *types.Pipline) {
	for _, exporter := range w.metricsExporters.Items {
		pipeline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
	}
}

func (w *oteldWrapper) appendAllLogsExporter(pipeline *types.Pipline) {
	for _, exporter := range w.logsExporters.Items {
		pipeline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
	}
}

func (w *oteldWrapper) appendUserDataSource() *oteldWrapper {
	for _, dataSource := range w.userSource.Items {
		var instance *types.OteldInstance

		if instance = w.instanceMap[v1alpha1.ModeDaemonSet]; instance == nil {
			instance = types.NewOteldInstance(w.OTeld, w.cli, w.ctx)
			w.instanceMap[v1alpha1.ModeDaemonSet] = instance
		}
		instance.AppDataSources = append(instance.AppDataSources, dataSource)
	}
	return w
}

func (w *oteldWrapper) buildFixedPipline() *oteldWrapper {
	for _, instance := range w.instanceMap {
		logsPipline := types.NewPipeline(LogsCreatorName)
		w.buildProcessor(&logsPipline)
		w.appendAllLogsExporter(&logsPipline)
		instance.AppLogsPipline = &logsPipline

		metricsPipline := types.NewPipeline(AppMetricsCreatorName)
		w.buildProcessor(&metricsPipline)
		w.appendAllMetricsExporter(&metricsPipline)
		instance.AppMetricsPiplien = &metricsPipline
	}
	return w
}

func (w *oteldWrapper) complete() error {
	return errors.Join(w.errs...)
}

func foundOrCreatePipeline(instance *types.OteldInstance, name string, collectType collectType) *types.Pipline {
	foundPipeline := func(pipelines []types.Pipline) *types.Pipline {
		for i := range pipelines {
			pipeline := &pipelines[i]
			if pipeline.Name == name {
				return pipeline
			}
		}
		return nil
	}
	checkAndCreate := func(pipeline []types.Pipline, update func(pipeline types.Pipline) *types.Pipline) *types.Pipline {
		if p := foundPipeline(pipeline); p != nil {
			return p
		}
		p := types.NewPipeline(name)
		return update(p)
	}

	switch collectType {
	case collectTypeMetrics:
		return checkAndCreate(instance.MetricsPipline, func(pipeline types.Pipline) *types.Pipline {
			instance.MetricsPipline = append(instance.MetricsPipline, pipeline)
			return &instance.MetricsPipline[len(instance.MetricsPipline)-1]
		})
	case collectTypeLogs:
		return checkAndCreate(instance.LogPipline, func(pipeline types.Pipline) *types.Pipline {
			instance.LogPipline = append(instance.LogPipline, pipeline)
			return &instance.LogPipline[len(instance.LogPipline)-1]
		})
	default:
		return nil
	}
}

func newOTeldHelper(source *v1alpha1.SystemDataSource, instanceMap map[v1alpha1.Mode]*types.OteldInstance, oteld *v1alpha1.OTeld, metricsExporters *v1alpha1.MetricsExporterSinkList, logsExporters *v1alpha1.LogsExporterSinkList, userSources *v1alpha1.CollectorDataSourceList, cli client.Client, ctx context.Context) *oteldWrapper {
	return &oteldWrapper{
		OTeld:            oteld,
		source:           source,
		userSource:       userSources,
		instanceMap:      instanceMap,
		logsExporters:    logsExporters,
		metricsExporters: metricsExporters,
		cli:              cli,
		ctx:              ctx,
	}
}
