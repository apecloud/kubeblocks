/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package render

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
)

type templateRenderWrapper struct {
	namespace    string
	clusterName  string
	templateName string

	// Global Var
	builtInFunctions *gotemplate.BuiltInObjectsFunc
	builtInObjects   *builtInObjects

	podSpec *corev1.PodSpec
	ctx     context.Context
	cli     client.Reader
}

const defaultTemplateName = "KbTemplate"

func NewTemplateBuilder(reconcileCtx *ReconcileCtx) TemplateRender {
	builder := &templateRenderWrapper{
		namespace:    reconcileCtx.Namespace,
		clusterName:  reconcileCtx.ClusterName,
		templateName: defaultTemplateName,
		ctx:          reconcileCtx.Context,
		cli:          reconcileCtx.Client,
	}
	builder.injectBuiltInObjectsAndFunctions(reconcileCtx.PodSpec, reconcileCtx.SynthesizedComponent, reconcileCtx.Cache, reconcileCtx.Cluster)
	return builder
}

func (c *templateRenderWrapper) setTemplateName(templateName string) {
	c.templateName = templateName
}

func (c *templateRenderWrapper) formatError(file string, err error) error {
	return fmt.Errorf("failed to render configuration template[cm:%s][key:%s], error: [%v]", c.templateName, file, err)
}

func (c *templateRenderWrapper) render(configs map[string]string) (map[string]string, error) {
	values, err := builtinObjectsAsValues(c.builtInObjects)
	if err != nil {
		return nil, err
	}

	rendered := make(map[string]string, len(configs))
	engine := gotemplate.NewTplEngine(values, c.builtInFunctions, c.templateName, c.cli, c.ctx)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, c.formatError(file, err)
		}
		rendered[file] = newContext
	}
	return rendered, nil
}

func (c *templateRenderWrapper) injectBuiltInObjectsAndFunctions(
	podSpec *corev1.PodSpec,
	component *component.SynthesizedComponent,
	localObjs []client.Object,
	cluster *appsv1.Cluster) {
	c.podSpec = podSpec
	c.builtInFunctions = BuiltInCustomFunctions(c, component, localObjs)
	c.builtInObjects = buildInComponentObjects(podSpec, component, cluster)
}

func findMatchedLocalObject(localObjs []client.Object, objKey client.ObjectKey, gvk schema.GroupVersionKind) client.Object {
	for _, obj := range localObjs {
		if obj.GetName() == objKey.Name && obj.GetNamespace() == objKey.Namespace {
			if generics.ToGVK(obj) == gvk {
				return obj
			}
		}
	}
	return nil
}

// RenderTemplate renders multiple component templates into Kubernetes ConfigMap objects.
func RenderTemplate(resourceCtx *ResourceCtx,
	cluster *appsv1.Cluster,
	synthesizedComponent *component.SynthesizedComponent,
	comp *appsv1.Component,
	localObjs []client.Object,
	tpls []appsv1.ComponentTemplateSpec) ([]*corev1.ConfigMap, error) {
	var err error
	var configMap *corev1.ConfigMap
	var configMaps []*corev1.ConfigMap

	reconcileCtx := &ReconcileCtx{
		ResourceCtx:          resourceCtx,
		Cluster:              cluster,
		Component:            comp,
		SynthesizedComponent: synthesizedComponent,
		PodSpec:              synthesizedComponent.PodSpec,
		Cache:                localObjs,
	}

	tplBuilder := NewTemplateBuilder(reconcileCtx)
	for _, tpl := range tpls {
		cmName := core.GetComponentCfgName(cluster.Name, synthesizedComponent.Name, tpl.Name)
		configMap, err = RenderComponentTemplate(cluster, synthesizedComponent, tplBuilder, cmName, tpl, resourceCtx.Context, reconcileCtx.Client, nil)
		if err != nil {
			return nil, err
		}
		if err = intctrlutil.SetOwnerReference(comp, configMap); err != nil {
			return nil, err
		}
		configMaps = append(configMaps, configMap)
	}
	return configMaps, nil
}
