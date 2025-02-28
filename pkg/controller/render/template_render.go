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
		if client.ObjectKeyFromObject(obj) == objKey && generics.ToGVK(obj) == gvk {
			return obj
		}
	}
	return nil
}
