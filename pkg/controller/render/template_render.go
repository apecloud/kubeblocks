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
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
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

	cluster          *appsv1.Cluster
	component        *appsv1.Component
	builtinComponent *component.SynthesizedComponent
	podSpec          *corev1.PodSpec

	ctx context.Context
	cli client.Client
}

const defaultTemplateName = "KbTemplate"

func NewTemplateBuilder(reconcileCtx *ReconcileCtx) TemplateRender {
	builder := &templateRenderWrapper{
		namespace:        reconcileCtx.Namespace,
		clusterName:      reconcileCtx.ClusterName,
		templateName:     defaultTemplateName,
		cluster:          reconcileCtx.Cluster,
		component:        reconcileCtx.Component,
		podSpec:          reconcileCtx.PodSpec,
		builtinComponent: reconcileCtx.SynthesizedComponent,
		ctx:              reconcileCtx.Context,
		cli:              reconcileCtx.Client,
	}
	builder.injectBuiltInObjectsAndFunctions(reconcileCtx.Cache)
	return builder
}

// RenderComponentTemplate renders config file by config template provided by provider.
func (r *templateRenderWrapper) RenderComponentTemplate(
	templateSpec appsv1.ComponentTemplateSpec,
	cmName string,
	dataValidator RenderedValidator) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := r.RenderConfigMapTemplate(templateSpec)
	if err != nil {
		return nil, err
	}

	if dataValidator != nil {
		if err = dataValidator(configs); err != nil {
			return nil, err
		}
	}

	// Using ConfigMap cue template render to configmap of config
	return factory.BuildConfigMapWithTemplate(r.cluster, r.builtinComponent, configs, cmName, templateSpec), nil
}

// RenderConfigMapTemplate renders config file using template engine
func (r *templateRenderWrapper) RenderConfigMapTemplate(templateSpec appsv1.ComponentTemplateSpec) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := r.cli.Get(r.ctx, client.ObjectKey{
		Namespace: templateSpec.Namespace,
		Name:      templateSpec.TemplateRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	r.setTemplateName(templateSpec.TemplateRef)
	renderedData, err := r.render(cmObj.Data)
	if err != nil {
		return nil, core.WrapError(err, "failed to render configmap")
	}
	return renderedData, nil
}

func (r *templateRenderWrapper) setTemplateName(templateName string) {
	r.templateName = templateName
}

func (r *templateRenderWrapper) formatError(file string, err error) error {
	return fmt.Errorf("failed to render configuration template[cm:%s][key:%s], error: [%v]", r.templateName, file, err)
}

func (r *templateRenderWrapper) render(configs map[string]string) (map[string]string, error) {
	values, err := builtinObjectsAsValues(r.builtInObjects)
	if err != nil {
		return nil, err
	}

	rendered := make(map[string]string, len(configs))
	engine := gotemplate.NewTplEngine(values, r.builtInFunctions, r.templateName, r.cli, r.ctx)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, r.formatError(file, err)
		}
		rendered[file] = newContext
	}
	return rendered, nil
}

func (r *templateRenderWrapper) injectBuiltInObjectsAndFunctions(localObjs []client.Object) {
	r.builtInFunctions = BuiltInCustomFunctions(r, r.builtinComponent, localObjs)
	r.builtInObjects = buildInComponentObjects(r.podSpec, r.builtinComponent, r.cluster)
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
