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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
)

// RenderComponentTemplate renders config file by config template provided by provider.
func RenderComponentTemplate(cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	templateRender TemplateRender,
	cmName string,
	templateSpec appsv1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client, dataValidator RenderedValidator) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := RenderConfigMapTemplate(templateRender, templateSpec, ctx, cli)
	if err != nil {
		return nil, err
	}

	if dataValidator != nil {
		if err = dataValidator(configs); err != nil {
			return nil, err
		}
	}

	// Using ConfigMap cue template render to configmap of config
	return factory.BuildConfigMapWithTemplate(cluster, component, configs, cmName, templateSpec), nil
}

// RenderConfigMapTemplate renders config file using template engine
func RenderConfigMapTemplate(
	templateRender TemplateRender,
	templateSpec appsv1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: templateSpec.Namespace,
		Name:      templateSpec.TemplateRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	templateRender.setTemplateName(templateSpec.TemplateRef)
	renderedData, err := templateRender.render(cmObj.Data)
	if err != nil {
		return nil, core.WrapError(err, "failed to render configmap")
	}
	return renderedData, nil
}
