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
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
)

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
