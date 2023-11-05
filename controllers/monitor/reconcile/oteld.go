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

	userDateSources := &v1alpha1.CollectorDataSourceList{}
	if err := k8sClient.List(ctx, userDateSources); err != nil {
		return err
	}

	instanceMap, err := BuildInstanceMapForPipeline(userDateSources, metricsExporters, logsExporters, reqCtx.OTeld, params.Client, reqCtx.Ctx)
	if err != nil {
		return err
	}

	reqCtx.OteldCfgRef.SetOteldInstance(metricsExporters, logsExporters, instanceMap)
	return nil
}

func BuildInstanceMapForPipeline(appDatasources *v1alpha1.CollectorDataSourceList,
	metricsExporters *v1alpha1.MetricsExporterSinkList,
	logsExporters *v1alpha1.LogsExporterSinkList,
	oteld *v1alpha1.OTeld,
	cli client.Client,
	ctx context.Context) (map[v1alpha1.Mode]*monitortype.OteldInstance, error) {

	instanceMap := map[v1alpha1.Mode]*monitortype.OteldInstance{
		DefaultMode: monitortype.NewOteldInstance(oteld, cli, ctx),
	}
	if err := buildSystemInstanceMap(oteld, instanceMap, metricsExporters, logsExporters, appDatasources, cli, ctx); err != nil {
		return nil, err
	}

	return instanceMap, nil
}

func buildSystemInstanceMap(oteld *v1alpha1.OTeld,
	instanceMap map[v1alpha1.Mode]*monitortype.OteldInstance,
	exporters *v1alpha1.MetricsExporterSinkList,
	logsExporters *v1alpha1.LogsExporterSinkList,
	datasources *v1alpha1.CollectorDataSourceList,
	cli client.Client,
	ctx context.Context) error {
	systemDataSource := oteld.Spec.SystemDataSource
	if systemDataSource == nil {
		return nil
	}

	return newOTeldHelper(systemDataSource, instanceMap, oteld, exporters, logsExporters, datasources, cli, ctx).
		buildAPIServicePipeline().
		buildK8sNodeStatesPipeline().
		buildNodePipeline().
		buildPodLogsPipeline().
		appendUserDataSource().
		buildFixedPipeline().
		buildSelfPipeline().
		complete()
}
