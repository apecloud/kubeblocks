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

package configuration

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type templateRenderValidator = func(map[string]string) error

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

// generateConfigMapFromTemplate renders config file by config template provided by provider.
func generateConfigMapFromTemplate(cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	tplBuilder *configTemplateBuilder,
	cmName string,
	templateSpec appsv1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client, dataValidator templateRenderValidator) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := renderConfigMapTemplate(tplBuilder, templateSpec, ctx, cli)
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

// renderConfigMapTemplate renders config file using template engine
func renderConfigMapTemplate(
	templateBuilder *configTemplateBuilder,
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

	templateBuilder.setTemplateName(templateSpec.TemplateRef)
	renderedData, err := templateBuilder.render(cmObj.Data)
	if err != nil {
		return nil, core.WrapError(err, "failed to render configmap")
	}
	return renderedData, nil
}

// validateRenderedData validates config file against constraint
func validateRenderedData(renderedData map[string]string, paramsDefs []*parametersv1alpha1.ParametersDefinition, configRender *parametersv1alpha1.ParameterDrivenConfigRender) error {
	if len(paramsDefs) == 0 || configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil
	}
	for _, paramsDef := range paramsDefs {
		fileName := paramsDef.Spec.FileName
		if paramsDef.Spec.ParametersSchema == nil {
			continue
		}
		if _, ok := renderedData[fileName]; !ok {
			continue
		}
		if fileConfig := resolveFileFormatConfig(configRender.Spec.Configs, fileName); fileConfig != nil {
			if err := validateConfigContent(renderedData[fileName], &paramsDef.Spec, fileConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveFileFormatConfig(configDescs []parametersv1alpha1.ComponentConfigDescription, fileName string) *parametersv1alpha1.FileFormatConfig {
	for i, configDesc := range configDescs {
		if fileName == configDesc.Name {
			return configDescs[i].FileFormatConfig
		}
	}
	return nil
}

func validateConfigContent(renderedData string, paramsDef *parametersv1alpha1.ParametersDefinitionSpec, fileFormat *parametersv1alpha1.FileFormatConfig) error {
	configChecker := validate.NewConfigValidator(paramsDef.ParametersSchema, fileFormat)
	// NOTE: It is necessary to verify the correctness of the data
	if err := configChecker.Validate(renderedData); err != nil {
		return core.WrapError(err, "failed to validate configmap")
	}
	return nil
}
