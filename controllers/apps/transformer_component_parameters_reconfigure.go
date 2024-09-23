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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
)

// componentParametersReloadSidecarTransformer handles component configuration render
type componentParametersReloadSidecarTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentParametersReloadSidecarTransformer{}

func (t *componentParametersReloadSidecarTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
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
	if len(synthesizeComp.ConfigTemplates) == 0 {
		return nil
	}

	// get dependOnObjs which will be used in configuration render
	var dependOnObjs []client.Object
	var configObj *configurationv1alpha1.ComponentParameter
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
		if config, ok := v.Obj.(*configurationv1alpha1.ComponentParameter); ok {
			configObj = config
		}
	}

	if configObj == nil {
		log.Log.Info("not found ComponentParameter resource and pass")
		return nil
	}

	// configuration render
	if err := plan.BuildReloadActionContainer(
		&configctrl.ResourceCtx{
			Context:       transCtx.Context,
			Client:        t.Client,
			Namespace:     comp.GetNamespace(),
			ClusterName:   synthesizeComp.ClusterName,
			ComponentName: synthesizeComp.Name,
		},
		cluster,
		comp,
		synthesizeComp,
		synthesizeComp.PodSpec,
		configObj,
		dependOnObjs); err != nil {
		return err
	}
	return nil
}
