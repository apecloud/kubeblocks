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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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

	templateVars, envVars, err := component.ResolveTemplateNEnvVars(transCtx.Context, reader,
		synthesizedComp, transCtx.CompDef.Spec.Vars)
	if err != nil {
		return err
	}

	// pass all direct value env vars through CM
	envVars2, envData := buildEnvVarsNData(envVars)
	setTemplateNEnvVars(synthesizedComp, templateVars, envVars2)

	if err := createOrUpdateEnvConfigMap(ctx, dag, envData); err != nil {
		return err
	}
	return nil
}

func buildEnvVarsNData(vars []corev1.EnvVar) ([]corev1.EnvVar, map[string]string) {
	hasReference := func(v corev1.EnvVar) bool {
		return len(component.VarReferenceRegExp().FindAllStringSubmatchIndex(v.Value, -1)) > 0
	}

	envVars := make([]corev1.EnvVar, 0)
	envData := make(map[string]string)
	for i, v := range vars {
		if v.ValueFrom != nil || hasReference(v) {
			envVars = append(envVars, vars[i])
		} else {
			envData[v.Name] = v.Value
		}
	}
	return envVars, envData
}

func setTemplateNEnvVars(synthesizedComp *component.SynthesizedComponent, templateVars map[string]any, envVars []corev1.EnvVar) {
	envSource := envConfigMapSource(synthesizedComp.ClusterName, synthesizedComp.Name)
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
	err := transCtx.Client.Get(transCtx.Context, envKey, envObj, inDataContext4C())
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if err != nil { // not-found
		obj := builder.NewConfigMapBuilder(envKey.Namespace, envKey.Name).
			AddLabelsInMap(constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
			SetData(data).
			GetObject()
		if err := setCompOwnershipNFinalizer(transCtx.Component, obj); err != nil {
			return err
		}
		graphCli.Create(dag, obj, inDataContext4G())
	} else if !reflect.DeepEqual(envObj.Data, data) {
		envObjCopy := envObj.DeepCopy()
		envObjCopy.Data = data
		graphCli.Update(dag, envObj, envObjCopy, inDataContext4G())
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
	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !items.IsValid() {
		return fmt.Errorf("ObjectList has no Items field: %s", list.GetObjectKind().GroupVersionKind().String())
	}

	if err := r.cli.List(ctx, list, opts...); err != nil {
		return err
	}

	objects := r.listFromGraph(items.Type(), opts...)

	// remove duplicated items
	names := sets.New[string]()
	for i := 0; i < objects.Len(); i++ {
		names.Insert(objects.Index(i).FieldByName("Name").String())
	}
	for i := 0; i < items.Len(); i++ {
		obj := items.Index(i)
		name := obj.FieldByName("Name").String()
		if !names.Has(name) {
			names.Insert(name)
			objects = reflect.Append(objects, obj)
		}
	}
	items.Set(objects)
	return nil
}

func (r *varsReader) listFromGraph(objectListType reflect.Type, opts ...client.ListOption) reflect.Value {
	objects := reflect.MakeSlice(objectListType, 0, 0)
	graphObjs := r.graphCli.FindAll(r.dag, reflect.New(objectListType.Elem()).Interface())
	if len(graphObjs) > 0 {
		listOpts := &client.ListOptions{}
		for _, opt := range opts {
			opt.ApplyToList(listOpts)
		}
		for i, obj := range graphObjs {
			if listOpts.LabelSelector != nil {
				if !listOpts.LabelSelector.Matches(labels.Set(obj.GetLabels())) {
					continue
				}
			}
			objects = reflect.Append(objects, reflect.ValueOf(graphObjs[i]).Elem())
		}
	}
	return objects
}
