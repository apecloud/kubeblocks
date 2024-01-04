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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentRestoreTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentRestoreTransformer{}

func (t *componentRestoreTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	cluster := transCtx.Cluster
	if cluster.Annotations[constant.RestoreFromBackupAnnotationKey] == "" {
		return nil
	}
	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	commitError := func(err error) error {
		if ictrlutil.IsTargetError(err, ictrlutil.ErrorTypeNeedWaiting) {
			transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeNormal, string(ictrlutil.ErrorTypeNeedWaiting), err.Error())
			return graph.ErrPrematureStop
		}
		return err
	}

	restoreMGR := plan.NewRestoreManager(reqCtx.Ctx, t.Client, cluster, rscheme, nil, transCtx.SynthesizeComponent.Replicas, 0)
	if err := restoreMGR.DoRestore(transCtx.SynthesizeComponent, transCtx.Component); err != nil {
		return commitError(err)
	}
	return nil
}
