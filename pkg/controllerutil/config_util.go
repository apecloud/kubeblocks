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

package controllerutil

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type Result struct {
	Phase      v1alpha1.ConfigurationPhase `json:"phase"`
	Revision   string                      `json:"revision"`
	Policy     string                      `json:"policy"`
	ExecResult string                      `json:"execResult"`

	SucceedCount  int32 `json:"succeedCount"`
	ExpectedCount int32 `json:"expectedCount"`

	Retry   bool   `json:"retry"`
	Failed  bool   `json:"failed"`
	Message string `json:"message"`
}

// MergeAndValidateConfigs merges and validates configuration files
func MergeAndValidateConfigs(configConstraint v1alpha1.ConfigConstraintSpec, baseConfigs map[string]string, cmKey []string, updatedParams []core.ParamPairs) (map[string]string, error) {
	var (
		err error
		fc  = configConstraint.FormatterConfig

		newCfg         map[string]string
		configOperator core.ConfigOperator
		updatedKeys    = util.NewSet()
	)

	cmKeySet := core.FromCMKeysSelector(cmKey)
	configLoaderOption := core.CfgOption{
		Type:           core.CfgCmType,
		Log:            log.FromContext(context.TODO()),
		CfgType:        fc.Format,
		ConfigResource: core.FromConfigData(baseConfigs, cmKeySet),
	}
	if configOperator, err = core.NewConfigLoader(configLoaderOption); err != nil {
		return nil, err
	}

	// merge param to config file
	for _, params := range updatedParams {
		if err := configOperator.MergeFrom(params.UpdatedParams, core.NewCfgOptions(params.Key, core.WithFormatterConfig(fc))); err != nil {
			return nil, err
		}
		updatedKeys.Add(params.Key)
	}

	if newCfg, err = configOperator.ToCfgContent(); err != nil {
		return nil, core.WrapError(err, "failed to generate config file")
	}

	// The ToCfgContent interface returns the file contents of all keys, the configuration file is encoded and decoded into keys,
	// the content may be different with the original file, such as comments, blank lines, etc,
	// in order to minimize the impact on the original file, only update the changed part.
	updatedCfg := fromUpdatedConfig(newCfg, updatedKeys)
	if err = validate.NewConfigValidator(&configConstraint, validate.WithKeySelector(cmKey)).Validate(updatedCfg); err != nil {
		return nil, core.WrapError(err, "failed to validate updated config")
	}
	return core.MergeUpdatedConfig(baseConfigs, updatedCfg), nil
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

// IsApplyConfigChanged checks if the configuration is changed
func IsApplyConfigChanged(configMap *corev1.ConfigMap, item v1alpha1.ConfigurationItemDetail) bool {
	if configMap == nil {
		return false
	}

	lastAppliedVersion, ok := configMap.Annotations[constant.ConfigAppliedVersionAnnotationKey]
	if !ok {
		return false
	}
	var target v1alpha1.ConfigurationItemDetail
	if err := json.Unmarshal([]byte(lastAppliedVersion), &target); err != nil {
		return false
	}

	return reflect.DeepEqual(target, item)
}

// IsRerender checks if the configuration template is changed
func IsRerender(configMap *corev1.ConfigMap, item v1alpha1.ConfigurationItemDetail) bool {
	if configMap == nil {
		return true
	}
	if item.Version == "" {
		return false
	}

	version, ok := configMap.Annotations[constant.CMConfigurationTemplateVersion]
	if !ok || version != item.Version {
		return true
	}
	return false
}

// GetConfigSpecReconcilePhase gets the configuration phase
func GetConfigSpecReconcilePhase(configMap *corev1.ConfigMap,
	item v1alpha1.ConfigurationItemDetail,
	status *v1alpha1.ConfigurationItemDetailStatus) v1alpha1.ConfigurationPhase {
	if status == nil || status.Phase == "" {
		return v1alpha1.CCreatingPhase
	}
	if !IsApplyConfigChanged(configMap, item) {
		return v1alpha1.CPendingPhase
	}
	return status.Phase
}
