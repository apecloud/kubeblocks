/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentRestoreTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentRestoreTransformer{}

func (t *componentRestoreTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	synthesizedComp := transCtx.SynthesizeComponent
	if synthesizedComp.Annotations[constant.RestoreFromBackupAnnotationKey] == "" {
		return nil
	}
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	commitError := func(err error) error {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting) {
			transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeNormal, string(intctrlutil.ErrorTypeNeedWaiting), err.Error())
			return errPrematureStopWithSetCompOwnership(transCtx.Component, dag, graphCli)
		}
		return err
	}

	cluster := transCtx.Cluster
	restoreMGR := plan.NewRestoreManager(reqCtx.Ctx, t.Client, cluster, rscheme, nil, synthesizedComp.Replicas, 0)

	actionCtx, err := component.NewActionContext(cluster, transCtx.Component, transCtx.RunningWorkload,
		synthesizedComp.LifecycleActions, synthesizedComp.ScriptTemplates, component.PostProvisionAction)
	if err != nil {
		return err
	}
	needDoPostProvision, _ := component.NeedDoPostProvision(transCtx.Context, transCtx.Client, actionCtx)
	if err := restoreMGR.DoRestore(synthesizedComp, transCtx.Component, needDoPostProvision); err != nil {
		return commitError(err)
	}

	if !reflect.DeepEqual(transCtx.ComponentOrig.Annotations, transCtx.Component.Annotations) {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		graphCli.Update(dag, transCtx.ComponentOrig, transCtx.Component, &model.ReplaceIfExistingOption{})
	}
	return nil
}
