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

package apps

import (
	graph2 "github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type RestoreTransformer struct {
	client.Client
}

var _ graph2.Transformer = &RestoreTransformer{}

func (t *RestoreTransformer) Transform(ctx graph2.TransformContext, dag *graph2.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	clusterDef := transCtx.ClusterDef
	clusterVer := transCtx.ClusterVer
	reqCtx := controllerutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	commitError := func(err error) error {
		if controllerutil.IsTargetError(err, controllerutil.ErrorTypeNeedWaiting) {
			transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeNormal, string(controllerutil.ErrorTypeNeedWaiting), err.Error())
			return graph2.ErrPrematureStop
		}
		return err
	}
	for _, spec := range cluster.Spec.ComponentSpecs {
		comp, err := components.NewComponent(reqCtx, t.Client, clusterDef, clusterVer, cluster, spec.Name, nil)
		if err != nil {
			return err
		}
		syncComp := comp.GetSynthesizedComponent()
		if cluster.Annotations[constant.RestoreFromBackUpAnnotationKey] != "" {
			if err = plan.DoRestore(reqCtx.Ctx, t.Client, cluster, syncComp, rscheme); err != nil {
				return commitError(err)
			}
		} else if cluster.Annotations[constant.RestoreFromTimeAnnotationKey] != "" {
			if err = plan.DoPITR(reqCtx.Ctx, t.Client, cluster, syncComp, rscheme); err != nil {
				return commitError(err)
			}
		}
	}
	return nil
}
