/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	"github.com/StudioSol/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ParamPairs struct {
	Key           string
	UpdatedParams map[string]interface{}
}

// MergeAndValidateConfigs merges and validates configuration files
func MergeAndValidateConfigs(configConstraint appsv1alpha1.ConfigConstraintSpec, baseConfigs map[string]string, cmKey []string, updatedParams []ParamPairs) (map[string]string, error) {
	var (
		err error
		fc  = configConstraint.FormatterConfig

		newCfg         map[string]string
		configOperator ConfigOperator
		updatedKeys    = set.NewLinkedHashSetString()
	)

	cmKeySet := FromCMKeysSelector(cmKey)
	configLoaderOption := CfgOption{
		Type:           CfgCmType,
		Log:            log.FromContext(context.TODO()),
		CfgType:        fc.Format,
		ConfigResource: FromConfigData(baseConfigs, cmKeySet),
	}
	if configOperator, err = NewConfigLoader(configLoaderOption); err != nil {
		return nil, err
	}

	// merge param to config file
	for _, params := range updatedParams {
		if err := configOperator.MergeFrom(params.UpdatedParams, NewCfgOptions(params.Key, WithFormatterConfig(fc))); err != nil {
			return nil, err
		}
		updatedKeys.Add(params.Key)
	}

	if newCfg, err = configOperator.ToCfgContent(); err != nil {
		return nil, WrapError(err, "failed to generate config file")
	}

	// The ToCfgContent interface returns the file contents of all keys, the configuration file is encoded and decoded into keys,
	// the content may be different with the original file, such as comments, blank lines, etc,
	// in order to minimize the impact on the original file, only update the changed part.
	updatedCfg := fromUpdatedConfig(newCfg, updatedKeys)
	if err = NewConfigValidator(&configConstraint, WithKeySelector(cmKey)).Validate(updatedCfg); err != nil {
		return nil, WrapError(err, "failed to validate updated config")
	}
	return MergeUpdatedConfig(baseConfigs, updatedCfg), nil
}

// MergeUpdatedConfig replaces the file content of the changed key.
// baseMap is the original configuration file,
// updatedMap is the updated configuration file
func MergeUpdatedConfig(baseMap, updatedMap map[string]string) map[string]string {
	r := make(map[string]string)
	for key, val := range baseMap {
		r[key] = val
		if v, ok := updatedMap[key]; ok {
			r[key] = v
		}
	}
	return r
}

// fromUpdatedConfig filters out changed file contents.
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

// FromStringMap converts a map[string]string to a map[string]interface{}
func FromStringMap(m map[string]string) map[string]interface{} {
	r := make(map[string]interface{}, len(m))
	for key, v := range m {
		r[key] = v
	}
	return r
}

func ApplyConfigPatch(baseCfg []byte, updatedParameters map[string]string, formatConfig *appsv1alpha1.FormatterConfig) (string, error) {
	configLoaderOption := CfgOption{
		Type:    CfgRawType,
		Log:     log.FromContext(context.TODO()),
		CfgType: formatConfig.Format,
		RawData: baseCfg,
	}
	configWrapper, err := NewConfigLoader(configLoaderOption)
	if err != nil {
		return "", err
	}

	mergedOptions := NewCfgOptions("", WithFormatterConfig(formatConfig))
	err = configWrapper.MergeFrom(FromStringMap(updatedParameters), mergedOptions)
	if err != nil {
		return "", err
	}
	mergedConfig := configWrapper.getConfigObject(mergedOptions)
	return mergedConfig.Marshal()
}

func NeedReloadVolume(config appsv1alpha1.ComponentConfigSpec) bool {
	// TODO distinguish between scripts and configuration
	return config.ConfigConstraintRef != ""
}

func GetReloadOptions(cli client.Client, ctx context.Context, configSpecs []appsv1alpha1.ComponentConfigSpec) (*appsv1alpha1.ReloadOptions, *appsv1alpha1.FormatterConfig, error) {
	for _, configSpec := range configSpecs {
		if !NeedReloadVolume(configSpec) {
			continue
		}
		ccKey := client.ObjectKey{
			Namespace: "",
			Name:      configSpec.ConfigConstraintRef,
		}
		cfgConst := &appsv1alpha1.ConfigConstraint{}
		if err := cli.Get(ctx, ccKey, cfgConst); err != nil {
			return nil, nil, WrapError(err, "failed to get ConfigConstraint, key[%v]", ccKey)
		}
		if cfgConst.Spec.ReloadOptions != nil {
			return cfgConst.Spec.ReloadOptions, cfgConst.Spec.FormatterConfig, nil
		}
	}
	return nil, nil, nil
}
