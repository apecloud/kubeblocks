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
)

type OTeldAgentConfig struct {
	OteldInstanceMap map[v1alpha1.Mode]*OteldInstance
	Exporters        *Exporters
}

type OteldCfgRef struct {
	*OTeldAgentConfig
}

func (c *OteldCfgRef) GetOteldInstance(mode v1alpha1.Mode) *OteldInstance {
	if c == nil || c.OteldInstanceMap == nil {
		return nil
	}
	return c.OteldInstanceMap[mode]
}

func (c *OteldCfgRef) SetOteldInstance(metricsExporters *v1alpha1.MetricsExporterSinkList, logsExporters *v1alpha1.LogsExporterSinkList, instanceMap map[v1alpha1.Mode]*OteldInstance) {
	c.OTeldAgentConfig = &OTeldAgentConfig{
		Exporters: &Exporters{
			MetricsExporter: metricsExporters.Items,
			LogsExporter:    logsExporters.Items,
		},
		OteldInstanceMap: instanceMap,
	}
}
