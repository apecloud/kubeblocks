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

package configuration

import (
	"context"

	"github.com/StudioSol/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ParamPairs struct {
	Key           string
	UpdatedParams map[string]interface{}
}

// MergeAndValidateConfigs does merge configuration files and validate
func MergeAndValidateConfigs(configConstraint appsv1alpha1.ConfigConstraintSpec, baseConfigs map[string]string, cmKey []string, updatedParams []ParamPairs) (map[string]string, error) {
	var (
		err error
		fc  = configConstraint.FormatterConfig

		newCfg         map[string]string
		configOperator ConfigOperator
		updatedKeys    = set.NewLinkedHashSetString()
	)

	cmKeySet := FromCMKeysSelector(cmKey)
	configOption := CfgOption{
		Type:           CfgCmType,
		Log:            log.FromContext(context.TODO()),
		CfgType:        fc.Format,
		ConfigResource: FromConfigData(baseConfigs, cmKeySet),
	}
	if configOperator, err = NewConfigLoader(configOption); err != nil {
		return nil, err
	}

	// process special formatter options
	mergedOptions := func(ctx *CfgOpOption) {
		// process special formatter
		if fc.Format == appsv1alpha1.Ini && fc.IniConfig != nil {
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
		updatedKeys.Add(params.Key)
	}

	if newCfg, err = configOperator.ToCfgContent(); err != nil {
		return nil, WrapError(err, "failed to generate config file")
	}

	// The ToCfgContent interface returns the file contents of all keys, and after the configuration file is encoded and decoded,
	// the content may be inconsistent, such as comments, blank lines, etc,
	// in order to minimize the impact on the original configuration file, only update the changed file content.
	updatedCfg := fromUpdatedConfig(newCfg, updatedKeys)
	if err = NewConfigValidator(&configConstraint, WithKeySelector(cmKey)).Validate(updatedCfg); err != nil {
		return nil, WrapError(err, "failed to validate updated config")
	}
	return mergeUpdatedConfig(baseConfigs, updatedCfg), nil
}

// mergeUpdatedConfig replaces the file content of the changed key.
// baseMap is the original configuration file,
// updatedMap is the updated configuration file
func mergeUpdatedConfig(baseMap, updatedMap map[string]string) map[string]string {
	r := make(map[string]string)
	for key, val := range baseMap {
		r[key] = val
		if v, ok := updatedMap[key]; ok {
			r[key] = v
		}
	}
	return r
}

// fromUpdatedConfig function is to filter out changed file contents.
func fromUpdatedConfig(m map[string]string, sets *set.LinkedHashSetString) map[string]string {
	if sets.Length() == 0 {
		return map[string]string{}
	}

	r := make(map[string]string, sets.Length())
	for key, v := range m {
		if sets.InArray(key) {
			r[key] = v
		}
	}
	return r
}
