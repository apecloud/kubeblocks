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
	monitor "github.com/apecloud/kubeblocks/controllers/monitor/config"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
)

const OTeldName = "apecloudoteld"

func OTeld(reqCtx types.ReconcileCtx, params types.OTeldParams) (err error) {
	var (
		ctx       = reqCtx.Ctx
		config    = reqCtx.Config
		k8sClient = params.Client
	)

	metricsExporters := &v1alpha1.MetricsExporterSinkList{}
	if err = k8sClient.List(ctx, metricsExporters); err != nil {
		return
	}

	logsExporters := &v1alpha1.LogsExporterSinkList{}
	if err = k8sClient.List(ctx, logsExporters); err != nil {
		return
	}

	datasources := &v1alpha1.CollectorDataSourceList{}
	if err = k8sClient.List(ctx, datasources); err != nil {
		return
	}

	gc := monitor.NewConfigGenerator(config)
	reqCtx.SetOteldConfig(gc.GenerateOteldConfiguration(datasources, metricsExporters, logsExporters))
	return
}
