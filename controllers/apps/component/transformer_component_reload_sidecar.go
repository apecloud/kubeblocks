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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentReloadActionSidecarTransformer handles component configuration render
type componentReloadActionSidecarTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentReloadActionSidecarTransformer{}

func (t *componentReloadActionSidecarTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	comp := transCtx.Component
	compOrig := transCtx.ComponentOrig
	builtinComp := transCtx.SynthesizeComponent

	if model.IsObjectDeleting(compOrig) {
		return nil
	}
	if common.IsCompactMode(compOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create configuration related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}
	if len(builtinComp.ConfigTemplates) == 0 {
		return nil
	}

	if err := updatePodVolumes(builtinComp.PodSpec, builtinComp); err != nil {
		return err
	}

	clusterKey := types.NamespacedName{
		Namespace: builtinComp.Namespace,
		Name:      builtinComp.ClusterName,
	}
	cluster := &appsv1.Cluster{}
	if err := t.Client.Get(transCtx, clusterKey, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "obtain the cluster object error for restore")
	}

	reconcileCtx := &render.ResourceCtx{
		Context:       transCtx.Context,
		Client:        t.Client,
		Namespace:     comp.GetNamespace(),
		ClusterName:   builtinComp.ClusterName,
		ComponentName: builtinComp.Name,
	}
	return configctrl.BuildReloadActionContainer(reconcileCtx, cluster, builtinComp, transCtx.CompDef)
}

func updatePodVolumes(podSpec *corev1.PodSpec, component *component.SynthesizedComponent) error {
	volumes := make(map[string]appsv1.ComponentTemplateSpec, len(component.ConfigTemplates))
	for _, tpls := range [][]appsv1.ComponentTemplateSpec{component.ConfigTemplates} {
		for _, tpl := range tpls {
			cmName := core.GetComponentCfgName(component.ClusterName, component.Name, tpl.Name)
			volumes[cmName] = tpl
		}
	}
	return intctrlutil.CreateOrUpdatePodVolumes(podSpec, volumes, configSetsFromComponent(component.ConfigTemplates))
}

func configSetsFromComponent(templates []appsv1.ComponentTemplateSpec) []string {
	configSet := make([]string, 0, len(templates))
	for _, template := range templates {
		configSet = append(configSet, template.Name)
	}
	return configSet
}
