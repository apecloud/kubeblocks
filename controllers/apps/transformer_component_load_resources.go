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
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentLoadResourcesTransformer handles referenced resources validation and load them into context
type componentLoadResourcesTransformer struct {
	client.Client
}

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

	generated := false
	generated, err = isGeneratedComponent(cluster, comp)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if generated {
		return t.transformForGeneratedComponent(transCtx)
	}
	return t.transformForNativeComponent(transCtx)
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
	compDef, err := t.getNCheckCompDef(transCtx)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	transCtx.CompDef = compDef

	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	comp := transCtx.Component
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, transCtx.Client, transCtx.Cluster, compDef, comp)
	if err != nil {
		message := fmt.Sprintf("build synthesized component for %s failed: %s", comp.Name, err.Error())
		return newRequeueError(requeueDuration, message)
	}
	transCtx.SynthesizeComponent = synthesizedComp

	return nil
}

func (t *componentLoadResourcesTransformer) getNCheckCompDef(transCtx *componentTransformContext) (*appsv1alpha1.ComponentDefinition, error) {
	compKey := types.NamespacedName{
		Namespace: transCtx.Component.Namespace,
		Name:      transCtx.Component.Spec.CompDef,
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := transCtx.Client.Get(transCtx.Context, compKey, compDef); err != nil {
		return nil, err
	}
	if compDef.Status.Phase != appsv1alpha1.AvailablePhase {
		return nil, fmt.Errorf("ComponentDefinition referenced is unavailable: %s", compDef.Name)
	}
	return compDef, nil
}

func isGeneratedComponent(cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component) (bool, error) {
	compName, err := component.ShortName(cluster.Name, comp.Name)
	if err != nil {
		return false, err
	}
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name == compName {
			if len(compSpec.ComponentDef) > 0 {
				if compSpec.ComponentDef != comp.Spec.CompDef {
					err = fmt.Errorf("component definitions referred in cluster and component are different: %s vs %s",
						compSpec.ComponentDef, comp.Spec.CompDef)
				}
				return false, err
			}
			return true, nil
		}
	}
	return true, fmt.Errorf("component %s is not found in cluster %s", compName, cluster.Name)
}
