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

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/component"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentConfigurationTransformer handles component configuration render
type componentConfigurationTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentConfigurationTransformer{}

func (t *componentConfigurationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	comp := transCtx.Component
	cluster := transCtx.Cluster
	compOrig := transCtx.ComponentOrig
	synthesizeComp := transCtx.SynthesizeComponent

	if model.IsObjectDeleting(compOrig) {
		return nil
	}
	if common.IsCompactMode(compOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create configuration related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	// wait for the completion of relevant conditions
	components, err := t.needWaiting(transCtx)
	if err != nil {
		return err
	}

	// get dependOnObjs which will be used in configuration render
	var dependOnObjs []client.Object
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*model.ObjectVertex)
		if cm, ok := v.Obj.(*corev1.ConfigMap); ok {
			dependOnObjs = append(dependOnObjs, cm)
			continue
		}
		if secret, ok := v.Obj.(*corev1.Secret); ok {
			dependOnObjs = append(dependOnObjs, secret)
			continue
		}
	}
	if components != nil {
		dependOnObjs = append(dependOnObjs, components...)
	}

	// configuration render
	if err := plan.RenderConfigNScriptFiles(
		&intctrlutil.ResourceCtx{
			Context:       transCtx.Context,
			Client:        t.Client,
			Namespace:     comp.GetNamespace(),
			ClusterName:   synthesizeComp.ClusterName,
			ComponentName: synthesizeComp.Name,
		},
		cluster,
		synthesizeComp,
		synthesizeComp.PodSpec,
		dependOnObjs); err != nil {
		return err
	}
	return nil
}

// needWaiting checks if it is necessary to wait for the completion of relevant conditions.
func (t *componentConfigurationTransformer) needWaiting(ctx *componentTransformContext) ([]client.Object, error) {
	if !ctx.CompDef.Spec.Runtime.HostNetwork {
		// if the component not uses hostNetwork, ignore it.
		return nil, nil
	}
	// HACK for hostNetwork
	// TODO: define the api to inject dynamic info of the cluster components
	return component.CheckAndGetClusterComponents(ctx.Context, t.Client, ctx.Cluster)
}
