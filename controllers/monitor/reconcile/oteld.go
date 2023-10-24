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
	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	monitortype "github.com/apecloud/kubeblocks/controllers/monitor/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OTeldName = "apecloudoteld"
const DefaultMode = v1alpha1.ModeDaemonSet

func OTeld(reqCtx monitortype.ReconcileCtx, params monitortype.OTeldParams) error {
	var (
		ctx       = reqCtx.Ctx
		k8sClient = params.Client
	)

	exporter := monitortype.Exporters{}
	metricsExporters := &v1alpha1.MetricsExporterSinkList{}
	if err := k8sClient.List(ctx, metricsExporters); err != nil {
		return err
	}
	exporter.MetricsExporter = metricsExporters.Items

	logsExporters := &v1alpha1.LogsExporterSinkList{}
	if err := k8sClient.List(ctx, logsExporters); err != nil {
		return err
	}
	exporter.LogsExporter = logsExporters.Items

	systemDatasources := &v1alpha1.CollectorDataSourceList{}
	if err := k8sClient.List(ctx, systemDatasources, client.InNamespace(reqCtx.OTeld.GetNamespace())); err != nil {
		return err
	}

	appDatasources := &v1alpha1.AppDataSourceList{}
	if err := k8sClient.List(ctx, appDatasources, client.InNamespace(reqCtx.OTeld.GetNamespace())); err != nil {
		return err
	}

	instanceMap, err := BuildInstanceMapForPipline(systemDatasources, appDatasources, reqCtx.OTeld)
	if err != nil {
		return err
	}

	reqCtx.OteldCfgRef.Exporters = &exporter
	reqCtx.OteldCfgRef.OteldInstanceMap = instanceMap
	if err = monitortype.VerifyOteldInstance(metricsExporters, logsExporters, instanceMap); err != nil {
		return err
	}
	return err
}

func BuildInstanceMapForPipline(datasources *v1alpha1.CollectorDataSourceList, appDatasources *v1alpha1.AppDataSourceList, oteld *v1alpha1.OTeld) (map[v1alpha1.Mode]*monitortype.OteldInstance, error) {
	instanceMap := map[v1alpha1.Mode]*monitortype.OteldInstance{}
	for _, dataSource := range datasources.Items {
		mode := dataSource.Spec.Mode
		if mode == "" {
			mode = DefaultMode
		}
		oteldInstance, ok := instanceMap[mode]
		if !ok {
			oteldInstance = monitortype.NewOteldInstance(oteld)
		}
		if oteldInstance.MetricsPipline == nil {
			oteldInstance.MetricsPipline = []monitortype.Pipline{}
		}
		pipline := monitortype.NewPipline()
		pipline.Name = dataSource.Name
		for _, data := range dataSource.Spec.DataSourceList {
			pipline.ReceiverMap[data.Name] = monitortype.Receiver{
				Parameter:          data.Parameter,
				CollectionInterval: dataSource.Spec.CollectionInterval,
			}
		}

		for _, exporter := range dataSource.Spec.ExporterNames {
			pipline.ExporterMap[exporter] = true
		}
		oteldInstance.MetricsPipline = append(oteldInstance.MetricsPipline, pipline)

		instanceMap[dataSource.Spec.Mode] = oteldInstance
	}

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
		instanceMap[dataSource.Spec.Mode] = oteldInstance
	}
	return instanceMap, nil
}
