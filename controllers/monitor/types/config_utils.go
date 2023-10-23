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
	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

func VerifyOteldInstance(metricsExporterList *v1alpha1.MetricsExporterSinkList, logsExporterList *v1alpha1.LogsExporterSinkList, instanceMap map[v1alpha1.Mode]*OteldInstance) error {
	metricsMap := make(map[string]bool)

	for _, mExporter := range metricsExporterList.Items {
		metricsMap[string(mExporter.Spec.Type)] = true
	}
	logMap := make(map[string]bool)
	for _, lExporter := range logsExporterList.Items {
		logMap[string(lExporter.Spec.Type)] = true
	}
	for _, instance := range instanceMap {
		if instance.MetricsPipline != nil {
			for _, pipline := range instance.MetricsPipline {
				for key := range pipline.ExporterMap {
					if _, ok := metricsMap[key]; !ok {
						return cfgcore.MakeError("not found exporter %s", key)
					}
				}
			}
		}
		if instance.LogsPipline != nil {
			for _, pipline := range instance.LogsPipline {
				for key := range pipline.ExporterMap {
					if _, ok := logMap[key]; !ok {
						return cfgcore.MakeError("not found exporter %s", key)
					}
				}
			}
		}
	}
	return nil
}
