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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	monitortype "github.com/apecloud/kubeblocks/controllers/monitor/types"
)

const (
	OTeldName           = "apecloudoteld"
	DefaultMode         = v1alpha1.ModeDaemonSet
	ExporterNamePattern = "%s/%s"
)

func OTeld(reqCtx monitortype.ReconcileCtx, params monitortype.OTeldParams) error {
	var (
		ctx       = reqCtx.Ctx
		k8sClient = params.Client
	)

	metricsExporters := &v1alpha1.MetricsExporterSinkList{}
	if err := k8sClient.List(ctx, metricsExporters); err != nil {
		return err
	}
	logsExporters := &v1alpha1.LogsExporterSinkList{}
	if err := k8sClient.List(ctx, logsExporters); err != nil {
		return err
	}

	// systemDatasources := &v1alpha1.CollectorDataSourceList{}
	// if err := k8sClient.List(ctx, systemDatasources, client.InNamespace(reqCtx.OTeld.GetNamespace())); err != nil {
	//	return err
	// }

	appDatasources := &v1alpha1.AppDataSourceList{}
	if err := k8sClient.List(ctx, appDatasources, client.InNamespace(reqCtx.OTeld.GetNamespace())); err != nil {
		return err
	}

	instanceMap, err := BuildInstanceMapForPipline(appDatasources, metricsExporters, logsExporters, reqCtx.OTeld)
	if err != nil {
		return err
	}

	reqCtx.OteldCfgRef.SetOteldInstance(metricsExporters, logsExporters, instanceMap)
	return nil
}

func BuildInstanceMapForPipline(appDatasources *v1alpha1.AppDataSourceList,
	metricsExporters *v1alpha1.MetricsExporterSinkList,
	logsExporters *v1alpha1.LogsExporterSinkList,
	oteld *v1alpha1.OTeld) (map[v1alpha1.Mode]*monitortype.OteldInstance, error) {

	instanceMap := map[v1alpha1.Mode]*monitortype.OteldInstance{
		DefaultMode: monitortype.NewOteldInstance(oteld),
	}
	if err := buildSystemInstanceMap(oteld, instanceMap, metricsExporters, logsExporters); err != nil {
		return nil, err
	}

	// for _, dataSource := range datasources.Items {
	//	mode := dataSource.Spec.Mode
	//	if mode == "" {
	//		mode = DefaultMode
	//	}
	//	oteldInstance, ok := instanceMap[mode]
	//	if !ok {
	//		oteldInstance = monitortype.NewOteldInstance(oteld)
	//	}
	//	if oteldInstance.MetricsPipline == nil {
	//		oteldInstance.MetricsPipline = []monitortype.Pipline{}
	//	}
	//	pipline := monitortype.NewPipeline(dataSource.Name)
	//	for _, data := range dataSource.Spec.DataSourceList {
	//		pipline.ReceiverMap[data.Name] = monitortype.Receiver{
	//			Parameter:          data.Parameter,
	//			CollectionInterval: dataSource.Spec.CollectionInterval,
	//		}
	//	}
	//
	//	if oteldInstance.Oteld.Spec.Batch.Enabled {
	//		pipline.ProcessorMap[monitortype.BatchProcessorName] = true
	//	}
	//	if oteldInstance.Oteld.Spec.MemoryLimiter.Enabled {
	//		pipline.ProcessorMap[monitortype.MemoryProcessorName] = true
	//	}
	//	switch dataSource.Spec.Type {
	//	case v1alpha1.MetricsDatasourceType:
	//		for _, exporterRef := range dataSource.Spec.ExporterNames {
	//			for _, exporter := range metricsExporters.Items {
	//				if exporter.Name == exporterRef {
	//					pipline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
	//				}
	//			}
	//		}
	//		oteldInstance.MetricsPipline = append(oteldInstance.MetricsPipline, pipline)
	//	case v1alpha1.LogsDataSourceType:
	//		for _, exporterRef := range dataSource.Spec.ExporterNames {
	//			for _, exporter := range logsExporters.Items {
	//				if exporter.Name == exporterRef {
	//					pipline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
	//				}
	//			}
	//		}
	//		oteldInstance.LogPipline = append(oteldInstance.LogPipline, pipline)
	//	default:
	//		return nil, fmt.Errorf("unknown data source type %s", dataSource.Spec.Type)
	//	}
	//	instanceMap[mode] = oteldInstance
	// }

	for _, dataSource := range appDatasources.Items {
		mode := dataSource.Spec.Mode
		if mode == "" {
			mode = DefaultMode
		}
		oteldInstance, ok := instanceMap[mode]
		if !ok {
			oteldInstance = monitortype.NewOteldInstance(oteld)
		}
		if oteldInstance.AppDataSources == nil {
			oteldInstance.AppDataSources = []v1alpha1.AppDataSource{}
		}
		oteldInstance.AppDataSources = append(oteldInstance.AppDataSources, dataSource)
		instanceMap[mode] = oteldInstance
	}

	for _, instance := range instanceMap {
		systemMetricsPipline := monitortype.NewPipeline(monitortype.AppMetricsCreatorName)
		if instance.Oteld.Spec.Batch.Enabled {
			systemMetricsPipline.ProcessorMap[monitortype.BatchProcessorName] = true
		}
		if instance.Oteld.Spec.MemoryLimiter.Enabled {
			systemMetricsPipline.ProcessorMap[monitortype.MemoryProcessorName] = true
		}
		for _, exporter := range metricsExporters.Items {
			systemMetricsPipline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
		}
		instance.AppMetricsPiplien = systemMetricsPipline

		logPipline := monitortype.NewPipeline(monitortype.LogCreatorName)
		logPipline.ReceiverMap[monitortype.LogCreatorName] = monitortype.Receiver{}
		if instance.Oteld.Spec.Batch.Enabled {
			logPipline.ProcessorMap[monitortype.BatchProcessorName] = true
		}
		if instance.Oteld.Spec.MemoryLimiter.Enabled {
			logPipline.ProcessorMap[monitortype.MemoryProcessorName] = true
		}
		for _, exporter := range logsExporters.Items {
			logPipline.ExporterMap[fmt.Sprintf(ExporterNamePattern, exporter.Spec.Type, exporter.Name)] = true
		}
		instance.AppLogsPipline = logPipline
	}

	return instanceMap, nil
}

func buildSystemInstanceMap(oteld *v1alpha1.OTeld,
	instanceMap map[v1alpha1.Mode]*monitortype.OteldInstance,
	exporters *v1alpha1.MetricsExporterSinkList,
	logsExporters *v1alpha1.LogsExporterSinkList) error {
	if oteld.Spec.SystemDataSource == nil {
		return nil
	}

	systemDataSource := oteld.Spec.SystemDataSource
	return newOTeldHelper(systemDataSource, instanceMap, oteld, exporters, logsExporters).
		buildAPIServicePipeline().
		buildK8sNodeStatesPipeline().
		buildNodePipeline().
		buildPodLogsPipeline().
		complete()
}
