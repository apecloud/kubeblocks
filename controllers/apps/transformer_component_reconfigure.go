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
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

	reconcileCtx := &configctrl.ResourceCtx{
		Context:       transCtx.Context,
		Client:        t.Client,
		Namespace:     comp.GetNamespace(),
		ClusterName:   synthesizeComp.ClusterName,
		ComponentName: synthesizeComp.Name,
	}

	var configmaps []*corev1.ConfigMap
	cachedObjs := resolveRerenderDependOnObjects(dag)
	for _, tpls := range [][]appsv1.ComponentTemplateSpec{synthesizeComp.ScriptTemplates, synthesizeComp.ConfigTemplates} {
		objects, err := configctrl.RenderTemplate(reconcileCtx, cluster, synthesizeComp, comp, cachedObjs, tpls)
		if err != nil {
			return err
		}
		configmaps = append(configmaps, objects...)
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if err := checkAndCreateConfigRelatedObjs(transCtx, graphCli, dag, configmaps...); err != nil {
		return err
	}
	if err := updatePodVolumes(synthesizeComp.PodSpec, synthesizeComp); err != nil {
		return err
	}
	if len(synthesizeComp.ConfigTemplates) == 0 {
		return nil
	}

	configRender, paramsDefs, err := intctrlutil.ResolveCmpdParametersDefs(transCtx, transCtx.Client, transCtx.CompDef)
	if err != nil {
		return err
	}
	if err = handleInjectEnv(transCtx, graphCli, dag, synthesizeComp, configRender, configmaps); err != nil {
		return err
	}
	return configctrl.BuildReloadActionContainer(reconcileCtx, cluster, synthesizeComp, configRender, paramsDefs)
}

func handleInjectEnv(ctx context.Context,
	graphCli model.GraphClient,
	dag *graph.DAG,
	comp *component.SynthesizedComponent,
	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
	configmaps []*corev1.ConfigMap) error {
	envObjs, err := configctrl.InjectTemplateEnvFrom(comp, comp.PodSpec, configRender, configmaps)
	if err != nil {
		return err
	}
	if len(envObjs) == 0 {
		return nil
	}
	return checkAndCreateConfigRelatedObjs(ctx, graphCli, dag, envObjs...)
}

func checkAndCreateConfigRelatedObjs(ctx context.Context, cli model.GraphClient, dag *graph.DAG, configmaps ...*corev1.ConfigMap) error {
	for _, configmap := range configmaps {
		var cm = &corev1.ConfigMap{}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(configmap), cm); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			cli.Create(dag, configmap, inDataContext4G())
		}
	}
	return nil
}

func resolveRerenderDependOnObjects(dag *graph.DAG) []client.Object {
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
	return dependOnObjs
}

func updatePodVolumes(podSpec *corev1.PodSpec, component *component.SynthesizedComponent) error {
	volumes := make(map[string]appsv1.ComponentTemplateSpec, len(component.ConfigTemplates))
	for _, tpls := range [][]appsv1.ComponentTemplateSpec{component.ConfigTemplates, component.ScriptTemplates} {
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
