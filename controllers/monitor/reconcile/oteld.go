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
)

const OTeldName = "apecloudoteld"
const DefaultMode = v1alpha1.ModeDaemonSet

func OTeld(reqCtx monitortype.ReconcileCtx, params monitortype.OTeldParams) (err error) {
	var (
		ctx       = reqCtx.Ctx
		k8sClient = params.Client
	)

	params.ConfigGenerator = monitortype.NewConfigGenerator()

	exporter := monitortype.Exporters{}
	metricsExporters := &v1alpha1.MetricsExporterSinkList{}
	if err = k8sClient.List(ctx, metricsExporters); err != nil {
		return
	}
	exporter.Metricsexporter = metricsExporters.Items

	logsExporters := &v1alpha1.LogsExporterSinkList{}
	if err = k8sClient.List(ctx, logsExporters); err != nil {
		return
	}
	exporter.Logsexporter = logsExporters.Items

	reqCtx.SetExporters(&exporter)

	datasources := &v1alpha1.CollectorDataSourceList{}
	if err = k8sClient.List(ctx, datasources); err != nil {
		return
	}

	oteldTemplates := &v1alpha1.OTeldCollectorTemplateList{}
	if err = k8sClient.List(ctx, oteldTemplates); err != nil {
		return
	}
	buildOteldInstance(reqCtx, metricsExporters, logsExporters, datasources, oteldTemplates)
	return
}

func buildOteldInstance(
	reqCtx monitortype.ReconcileCtx,
	metricsExporterList *v1alpha1.MetricsExporterSinkList,
	logsExporterList *v1alpha1.LogsExporterSinkList,
	datasources *v1alpha1.CollectorDataSourceList,
	templates *v1alpha1.OTeldCollectorTemplateList,
) error {
	instanceMap, err := BuildInstanceMapForPipline(datasources)
	if err != nil {
		return err
	}
	reqCtx.SetOteldInstanceMap(instanceMap)

	for _, template := range templates.Items {
		instance := reqCtx.GetOteldInstance(template.Spec.Mode)
		if instance == nil {
			continue
		}
		instance.OteldTemplate = &template
	}

	if err := reqCtx.VerifyOteldInstance(metricsExporterList, logsExporterList); err != nil {
		return err
	}
	return nil
}

func BuildInstanceMapForPipline(datasources *v1alpha1.CollectorDataSourceList) (map[v1alpha1.Mode]*monitortype.OteldInstance, error) {
	instanceMap := map[v1alpha1.Mode]*monitortype.OteldInstance{}
	for _, dataSource := range datasources.Items {
		mode := dataSource.Spec.Mode
		if mode == "" {
			mode = DefaultMode
		}
		oteldInstance, ok := instanceMap[mode]
		if !ok {
			oteldInstance = monitortype.NewOteldInstance()
		}
		switch dataSource.Spec.Type {
		case v1alpha1.MetricsDatasourceType:
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
			oteldInstance.MetricsPipline = append(oteldInstance.MetricsPipline, pipline)

		case v1alpha1.LogsDataSourceType:
			if oteldInstance.LogsPipline == nil {
				oteldInstance.LogsPipline = []monitortype.Pipline{}
			}
			pipline := monitortype.NewPipline()
			pipline.Name = dataSource.Name
			for _, data := range dataSource.Spec.DataSourceList {
				pipline.ReceiverMap[data.Name] = monitortype.Receiver{Parameter: data.Parameter}
			}
			oteldInstance.LogsPipline = append(oteldInstance.LogsPipline, pipline)
		}
		instanceMap[dataSource.Spec.Mode] = oteldInstance
	}
	return instanceMap, nil
}
