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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ComponentConfigurationTransformer handles component configuration render
type ComponentConfigurationTransformer struct {
	client.Client
}

var _ graph.Transformer = &ComponentConfigurationTransformer{}

func (t *ComponentConfigurationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ComponentTransformContext)

	comp := transCtx.Component
	cluster := transCtx.Cluster
	compOrig := transCtx.ComponentOrig
	synthesizeComp := transCtx.SynthesizeComponent

	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	// get rsm and dependOnObjs which will be used in configuration render
	var rsm *workloads.ReplicatedStateMachine
	var dependOnObjs []client.Object
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*model.ObjectVertex)
		if rsmV, ok := v.Obj.(*workloads.ReplicatedStateMachine); ok {
			rsm = rsmV
			continue
		}
		if cm, ok := v.Obj.(*corev1.ConfigMap); ok {
			dependOnObjs = append(dependOnObjs, cm)
			continue
		}
		if secret, ok := v.Obj.(*corev1.Secret); ok {
			dependOnObjs = append(dependOnObjs, secret)
			continue
		}
	}

	if rsm == nil {
		return errors.New("rsm workload not found")
	}

	// configuration render
	// In versions prior to KubeBlocks 0.7.0, users were able to access the ClusterVersion object in configuration template rendering to retrieve the corresponding values.
	// However, after KubeBlocks 0.7.0 version, the ClusterVersion will be deprecated. This means that the previous functionality will be affected, and this change was expected.
	// In this case, the ClusterVersion will be set to nil, and there will be a subsequent refactoring of the configuration rendering module to remove the ClusterVersion entirely.
	// TODO(xingran): remove clusterVersion in configuration rendering
	if err := plan.RenderConfigNScriptFiles(
		&intctrlutil.ResourceCtx{
			Context:       transCtx.Context,
			Client:        t.Client,
			Namespace:     comp.GetNamespace(),
			ClusterName:   synthesizeComp.ClusterName,
			ComponentName: synthesizeComp.Name,
		},
		nil,
		cluster,
		synthesizeComp,
		rsm,
		synthesizeComp.PodSpec,
		dependOnObjs); err != nil {
		return err
	}
	return nil
}
