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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ComponentLoadResourcesTransformer handles referenced resources validation and load them into context
type ComponentLoadResourcesTransformer struct{}

var _ graph.Transformer = &ComponentLoadResourcesTransformer{}

func (t *ComponentLoadResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ComponentTransformContext)
	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	comp := transCtx.Component

	var err error
	defer func() {
		setProvisioningStartedCondition(&comp.Status.Conditions, comp.Name, comp.Generation, err)
	}()

	// get and init component definition context
	compDef := &appsv1alpha1.ComponentDefinition{}
	err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: comp.Spec.CompDef}, compDef)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if compDef.Status.Phase != appsv1alpha1.AvailablePhase {
		message := fmt.Sprintf("ComponentDefinition referenced is unavailable: %s", compDef.Name)
		return newRequeueError(requeueDuration, message)
	}

	// get and init cluster context
	// TODO(xingran): In order to backward compatibility in KubeBlocks version 0.7.0, the cluster field is still required. However, if in the future the Component objects can be used independently, the Cluster field should be removed from the component.Spec
	cluster := &appsv1alpha1.Cluster{}
	err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: comp.Spec.Cluster, Namespace: comp.Namespace}, cluster)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	transCtx.CompDef = compDef
	transCtx.Cluster = cluster

	synthesizeComp, err := component.BuildSynthesizedComponent(reqCtx, transCtx.Client, cluster, compDef, comp)
	if err != nil {
		message := fmt.Sprintf("Component %s BuildSynthesizedComponent failed: %s", comp.Name, err.Error())
		return newRequeueError(requeueDuration, message)
	}
	transCtx.SynthesizeComponent = synthesizeComp
	return nil
}
