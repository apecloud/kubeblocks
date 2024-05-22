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
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentLoadResourcesTransformer handles referenced resources validation and load them into context
type componentLoadResourcesTransformer struct{}

var _ graph.Transformer = &componentLoadResourcesTransformer{}

func (t *componentLoadResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	comp := transCtx.Component

	var err error
	defer func() {
		setProvisioningStartedCondition(&comp.Status.Conditions, comp.Name, comp.Generation, err)
	}()

	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	cluster := &appsv1alpha1.Cluster{}
	err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: clusterName, Namespace: comp.Namespace}, cluster)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	transCtx.Cluster = cluster

	if component.IsGenerated(transCtx.ComponentOrig) {
		err = t.transformForGeneratedComponent(transCtx)
		if err != nil {
			return err
		}
		return nil
	}

	err = t.transformForNativeComponent(transCtx)
	if err != nil {
		return err
	}
	return nil
}

func (t *componentLoadResourcesTransformer) transformForGeneratedComponent(transCtx *componentTransformContext) error {
	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	comp := transCtx.Component

	compDef, synthesizedComp, err := component.BuildSynthesizedComponent4Generated(reqCtx, transCtx.Client, transCtx.Cluster, comp)
	if err != nil {
		message := fmt.Sprintf("build synthesized component for %s failed: %s", comp.Name, err.Error())
		return newRequeueError(requeueDuration, message)
	}
	transCtx.CompDef = compDef
	transCtx.SynthesizeComponent = synthesizedComp

	return nil
}

func (t *componentLoadResourcesTransformer) transformForNativeComponent(transCtx *componentTransformContext) error {
	var (
		ctx  = transCtx.Context
		cli  = transCtx.Client
		comp = transCtx.Component
	)
	compDef, err := getNCheckCompDefinition(ctx, cli, comp.Spec.CompDef)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	if err = component.UpdateCompDefinitionImages4ServiceVersion(ctx, cli, compDef, comp.Spec.ServiceVersion); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	transCtx.CompDef = compDef

	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, transCtx.Client, transCtx.Cluster, compDef, comp)
	if err != nil {
		message := fmt.Sprintf("build synthesized component for %s failed: %s", comp.Name, err.Error())
		return newRequeueError(requeueDuration, message)
	}
	transCtx.SynthesizeComponent = synthesizedComp

	return nil
}
