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
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentVarsTransformer resolves and builds vars for template and Env.
type componentVarsTransformer struct{}

var _ graph.Transformer = &componentVarsTransformer{}

func (t *componentVarsTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create var related objects", "component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	reader := &varsReader{transCtx.Client, graphCli, dag}
	synthesizedComp := transCtx.SynthesizeComponent

	generated, err := isGeneratedComponent(transCtx.Cluster, transCtx.ComponentOrig)
	if err != nil {
		return err
	}

	var templateVars map[string]any
	var envVars []corev1.EnvVar
	if generated {
		templateVars, envVars, err = component.ResolveEnvVars4LegacyCluster(transCtx.Context, reader,
			synthesizedComp, transCtx.Cluster.Annotations, transCtx.CompDef.Spec.Vars)
	} else {
		templateVars, envVars, err = component.ResolveTemplateNEnvVars(transCtx.Context, reader,
			synthesizedComp, transCtx.Cluster.Annotations, transCtx.CompDef.Spec.Vars)
	}
	if err != nil {
		return err
	}

	// pass all direct value env vars through CM
	envVars2, envData := buildEnvVarsNData(synthesizedComp, envVars, generated)
	setTemplateNEnvVars(synthesizedComp, templateVars, envVars2, generated)

	return createOrUpdateEnvConfigMap(ctx, dag, envData)
}

func buildEnvVarsNData(synthesizedComp *component.SynthesizedComponent, vars []corev1.EnvVar, legacy bool) ([]corev1.EnvVar, map[string]string) {
	envData := make(map[string]string)
	if synthesizedComp != nil && synthesizedComp.ComponentRefEnvs != nil {
		for _, env := range synthesizedComp.ComponentRefEnvs {
			envData[env.Name] = env.Value
		}
	}

	// for legacy cluster, don't move direct values into ConfigMap
	if legacy {
		return vars, envData
	}

	envVars := make([]corev1.EnvVar, 0)
	for i, v := range vars {
		if v.ValueFrom == nil {
			envData[v.Name] = v.Value
		} else {
			envVars = append(envVars, vars[i])
		}
	}
	return envVars, envData
}

func setTemplateNEnvVars(synthesizedComp *component.SynthesizedComponent, templateVars map[string]any, envVars []corev1.EnvVar, legacy bool) {
	envSource := envConfigMapSource(synthesizedComp.ClusterName, synthesizedComp.Name)
	if legacy {
		envSource.ConfigMapRef.Optional = nil
	}

	synthesizedComp.TemplateVars = templateVars
	synthesizedComp.EnvVars = envVars
	synthesizedComp.EnvFromSources = []corev1.EnvFromSource{envSource}

	component.InjectEnvVars(synthesizedComp, envVars, []corev1.EnvFromSource{envSource})
}

func envConfigMapSource(clusterName, compName string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: constant.GenerateClusterComponentEnvPattern(clusterName, compName),
			},
			Optional: func() *bool { optional := false; return &optional }(),
		},
	}
}

func createOrUpdateEnvConfigMap(ctx graph.TransformContext, dag *graph.DAG, data map[string]string) error {
	var (
		transCtx, _     = ctx.(*componentTransformContext)
		synthesizedComp = transCtx.SynthesizeComponent
		envKey          = types.NamespacedName{
			Namespace: synthesizedComp.Namespace,
			Name:      constant.GenerateClusterComponentEnvPattern(synthesizedComp.ClusterName, synthesizedComp.Name),
		}
	)
	envObj := &corev1.ConfigMap{}
	err := transCtx.Client.Get(transCtx.Context, envKey, envObj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if err != nil { // not-found
		obj := builder.NewConfigMapBuilder(envKey.Namespace, envKey.Name).
			AddLabelsInMap(constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
			SetData(data).
			GetObject()
		graphCli.Create(dag, obj)
	} else if !reflect.DeepEqual(envObj.Data, data) {
		envObjCopy := envObj.DeepCopy()
		envObjCopy.Data = data
		graphCli.Update(dag, envObj, envObjCopy)
	}
	return nil
}

type varsReader struct {
	cli      client.Reader
	graphCli model.GraphClient
	dag      *graph.DAG
}

func (r *varsReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	for _, val := range r.graphCli.FindAll(r.dag, obj) {
		if client.ObjectKeyFromObject(val) == key {
			reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(val).Elem())
			return nil
		}
	}
	return r.cli.Get(ctx, key, obj, opts...)
}

func (r *varsReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return r.cli.List(ctx, list, opts...)
}
