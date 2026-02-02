/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

	return t.transformForNativeComponent(transCtx)
}

func (t *componentLoadResourcesTransformer) transformForNativeComponent(transCtx *componentTransformContext) error {
	var (
		ctx  = transCtx.Context
		cli  = transCtx.Client
		comp = transCtx.Component
	)
	compDef, err := getNCheckCompDefinition(ctx, cli, comp.Spec.CompDef)
	if err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}
	if err = component.UpdateCompDefinitionImages4ServiceVersion(ctx, cli, compDef, comp.Spec.ServiceVersion); err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}
	transCtx.CompDef = compDef

	synthesizedComp, err := component.BuildSynthesizedComponent(ctx, transCtx.Client, compDef, comp)
	if err != nil {
		message := fmt.Sprintf("build synthesized component for %s failed: %s", comp.Name, err.Error())
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, message)
	}
	transCtx.SynthesizeComponent = synthesizedComp

	runningITS, err := t.runningInstanceSetObject(transCtx, synthesizedComp)
	if err != nil {
		return err
	}
	transCtx.RunningWorkload = runningITS

	return nil
}

func getNCheckCompDefinition(ctx context.Context, cli client.Reader, name string) (*appsv1.ComponentDefinition, error) {
	compKey := types.NamespacedName{
		Name: name,
	}
	compDef := &appsv1.ComponentDefinition{}
	if err := cli.Get(ctx, compKey, compDef); err != nil {
		return nil, err
	}
	if compDef.Generation != compDef.Status.ObservedGeneration {
		return nil, fmt.Errorf("the referenced ComponentDefinition is not up to date: %s", compDef.Name)
	}
	if compDef.Status.Phase != appsv1.AvailablePhase {
		return nil, fmt.Errorf("the referenced ComponentDefinition is unavailable: %s", compDef.Name)
	}
	return compDef, nil
}

func (t *componentLoadResourcesTransformer) runningInstanceSetObject(ctx *componentTransformContext,
	synthesizeComp *component.SynthesizedComponent) (*workloads.InstanceSet, error) {
	objs, err := component.ListOwnedWorkloads(ctx.GetContext(), ctx.GetClient(),
		synthesizeComp.Namespace, synthesizeComp.ClusterName, synthesizeComp.Name)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, nil
	}
	return objs[0], nil
}
