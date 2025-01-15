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

package component

import (
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
			transCtx.EventRecorder.Event(transCtx.Component, corev1.EventTypeNormal, string(intctrlutil.ErrorTypeNeedWaiting), err.Error())
			return graph.ErrPrematureStop
		}
		return err
	}

	clusterKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      synthesizedComp.ClusterName,
	}
	cluster := &appsv1.Cluster{}
	if err := t.Client.Get(reqCtx.Ctx, clusterKey, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "obtain the cluster object error for restore")
	}

	restoreMGR := plan.NewRestoreManager(reqCtx.Ctx, t.Client, cluster, model.GetScheme(), nil, synthesizedComp.Replicas, 0)

	postProvisionDone := checkPostProvisionDone(transCtx)
	if err := restoreMGR.DoRestore(synthesizedComp, transCtx.Component, postProvisionDone); err != nil {
		return commitError(err)
	}

	if !reflect.DeepEqual(transCtx.ComponentOrig.Annotations, transCtx.Component.Annotations) {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		graphCli.Update(dag, transCtx.ComponentOrig, transCtx.Component, &model.ReplaceIfExistingOption{})
	}
	return nil
}
