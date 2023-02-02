/*
Copyright ApeCloud, Inc.

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

/*
Copyright ApeCloud Inc.

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

package configuration

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type ParamPairs struct {
	Key           string
	UpdatedParams map[string]interface{}
}

// MergeAndValidateConfiguration does merge configuration files and validate
func MergeAndValidateConfiguration(configConstraint dbaasv1alpha1.ConfigConstraintSpec, baseCfg map[string]string, updatedParams []ParamPairs) (map[string]string, error) {
	var (
		err            error
		newCfg         map[string]string
		configOperator ConfigOperator

		fc = configConstraint.FormatterConfig
	)

	if configOperator, err = NewConfigLoader(CfgOption{
		Type:    CfgCmType,
		Log:     log.FromContext(context.TODO()),
		CfgType: fc.Formatter,
		K8sKey: &K8sConfig{
			CfgKey: client.ObjectKey{},
			ResourceFn: func(key client.ObjectKey) (map[string]string, error) {
				return baseCfg, nil
			},
		}}); err != nil {
		return nil, err
	}

	// process special formatter options
	mergedOptions := func(ctx *CfgOpOption) {
		// process special formatter
		if fc.Formatter == dbaasv1alpha1.INI && fc.IniConfig != nil {
			ctx.IniContext = &IniContext{
				SectionName: fc.IniConfig.SectionName,
			}
		}
	}

	// merge param to config file
	for _, params := range updatedParams {
		if err := configOperator.MergeFrom(params.UpdatedParams, NewCfgOptions(params.Key, mergedOptions)); err != nil {
			return nil, err
		}
	}

	if newCfg, err = configOperator.ToCfgContent(); err != nil {
		return nil, WrapError(err, "failed to generate config file")
	}
	if err = NewConfigValidator(&configConstraint).Validate(newCfg); err != nil {
		return nil, WrapError(err, "failed to validate updated config")
	}
	return newCfg, nil
}
