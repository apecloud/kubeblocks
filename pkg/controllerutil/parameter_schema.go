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

package controllerutil

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
)

type ParameterMeta struct {
	FileName           string
	ConfigTemplateName string
}

func GetConfigParameterMeta(ctx context.Context, cli client.Reader, parameterDesc appsv1.ComponentParametersDescription, configTemplate *appsv1.ComponentConfigSpec) (map[string]*ParameterMeta, error) {
	paramDefKey := client.ObjectKey{
		Name: parameterDesc.ParametersDefName,
	}

	paramDef := &configurationv1alpha1.ParametersDefinition{}
	if err := cli.Get(ctx, paramDefKey, paramDef); err != nil {
		return nil, err
	}

	if paramDef.Spec.ParametersSchema == nil || paramDef.Spec.ParametersSchema.SchemaInJSON == nil {
		return nil, nil
	}

	schema := paramDef.Spec.ParametersSchema.SchemaInJSON
	if _, ok := schema.Properties[openapi.DefaultSchemaName]; !ok {
		return nil, nil
	}

	paramMeta := &ParameterMeta{
		FileName:           parameterDesc.Name,
		ConfigTemplateName: configTemplate.Name,
	}
	props := openapi.FlattenSchema(schema.Properties[openapi.DefaultSchemaName]).Properties
	params := make(map[string]*ParameterMeta, len(props))
	for key := range props {
		params[key] = paramMeta
	}

	return params, nil
}
