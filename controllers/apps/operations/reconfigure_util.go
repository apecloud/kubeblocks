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

package operations

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type reconfiguringResult struct {
	failed               bool
	noFormatFilesUpdated bool
	configPatch          *core.ConfigPatchInfo
	lastAppliedConfigs   map[string]string
	err                  error
}

func updateOpsLabelWithReconfigure(obj *appsv1alpha1.OpsRequest, params []core.ParamPairs, orinalData map[string]string, formatter *appsv1beta1.FileFormatConfig) {
	var maxLabelCount = 16
	updateLabel := func(param map[string]interface{}) {
		if obj.Labels == nil {
			obj.Labels = make(map[string]string)
		}
		for key, val := range param {
			if maxLabelCount <= 0 {
				return
			}
			paramName := core.GetValidFieldName(key)
			if core.IsValidLabelKeyOrValue(paramName) {
				obj.Labels[paramName] = core.FromValueToString(val)
			}
			maxLabelCount--
		}
	}
	updateAnnotation := func(keyFile string, param map[string]interface{}) {
		data, ok := orinalData[keyFile]
		if !ok {
			return
		}
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		oldValue, err := fetchOriginalValue(keyFile, data, param, formatter)
		if err != nil {
			log.Log.Error(err, "failed to fetch original value")
			return
		}
		obj.Annotations[keyFile] = oldValue
	}

	for _, param := range params {
		updateLabel(param.UpdatedParams)
		if maxLabelCount <= 0 {
			return
		}
		updateAnnotation(param.Key, param.UpdatedParams)
	}
}

func fetchOriginalValue(keyFile, data string, params map[string]interface{}, formatter *appsv1beta1.FileFormatConfig) (string, error) {
	baseConfigObj, err := core.FromConfigObject(keyFile, data, formatter)
	if err != nil {
		return "", err
	}
	r := make(map[string]string, len(params))
	for key := range params {
		oldVal := baseConfigObj.Get(key)
		if oldVal != nil {
			r[key] = cast.ToString(oldVal)
		}
	}
	b, err := json.Marshal(r)
	return string(b), err
}

func fromKeyValuePair(parameters []appsv1alpha1.ParameterPair) map[string]interface{} {
	m := make(map[string]interface{}, len(parameters))
	for _, param := range parameters {
		if param.Value != nil {
			m[param.Key] = *param.Value
		} else {
			m[param.Key] = nil
		}
	}
	return m
}

func withFailed(failed bool) func(result *reconfiguringResult) {
	return func(result *reconfiguringResult) {
		result.failed = failed
	}
}

func withReturned(configs map[string]string, patch *core.ConfigPatchInfo) func(result *reconfiguringResult) {
	return func(result *reconfiguringResult) {
		result.lastAppliedConfigs = configs
		result.configPatch = patch
	}
}

func withNoFormatFilesUpdated(changed bool) func(result *reconfiguringResult) {
	return func(result *reconfiguringResult) {
		result.noFormatFilesUpdated = changed
	}
}

func makeReconfiguringResult(err error, ops ...func(*reconfiguringResult)) reconfiguringResult {
	result := reconfiguringResult{
		failed: false,
		err:    err,
	}
	for _, o := range ops {
		o(&result)
	}
	return result
}

func constructReconfiguringConditions(result reconfiguringResult, resource *OpsResource, configSpec *appsv1alpha1.ComponentConfigSpec) *metav1.Condition {
	if result.noFormatFilesUpdated || (result.configPatch != nil && result.configPatch.IsModify) {
		return appsv1alpha1.NewReconfigureRunningCondition(
			resource.OpsRequest,
			appsv1alpha1.ReasonReconfigurePersisted,
			configSpec.Name,
			formatConfigPatchToMessage(result.configPatch, nil))
	}
	return appsv1alpha1.NewReconfigureRunningCondition(
		resource.OpsRequest,
		appsv1alpha1.ReasonReconfigureNoChanged,
		configSpec.Name,
		formatConfigPatchToMessage(result.configPatch, nil))
}

func i2sMap(config map[string]interface{}) map[string]string {
	if len(config) == 0 {
		return nil
	}
	m := make(map[string]string, len(config))
	for key, value := range config {
		data, _ := json.Marshal(value)
		m[key] = string(data)
	}
	return m
}

func b2sMap(config map[string][]byte) map[string]string {
	if len(config) == 0 {
		return nil
	}
	m := make(map[string]string, len(config))
	for key, value := range config {
		m[key] = string(value)
	}
	return m
}

func processMergedFailed(resource *OpsResource, isInvalid bool, err error) error {
	if !isInvalid {
		return core.WrapError(err, "failed to update param!")
	}

	// if failed to validate configure, set opsRequest to failed and return
	failedCondition := appsv1alpha1.NewReconfigureFailedCondition(resource.OpsRequest, err)
	resource.OpsRequest.SetStatusCondition(*failedCondition)
	return intctrlutil.NewFatalError(err.Error())
}

func formatConfigPatchToMessage(configPatch *core.ConfigPatchInfo, execStatus *core.PolicyExecStatus) string {
	policyName := ""
	if execStatus != nil {
		policyName = fmt.Sprintf("updated policy: <%s>, ", execStatus.PolicyName)
	}
	if configPatch == nil {
		return fmt.Sprintf("%supdated full config files.", policyName)
	}
	return fmt.Sprintf("%supdated: %s, added: %s, deleted:%s",
		policyName,
		configPatch.UpdateConfig,
		configPatch.AddConfig,
		configPatch.DeleteConfig)
}

func updateFileContent(item *appsv1alpha1.ConfigurationItemDetail, key string, content string) {
	params, ok := item.ConfigFileParams[key]
	if !ok {
		item.ConfigFileParams[key] = appsv1alpha1.ParametersInFile{
			Content: &content,
		}
		return
	}
	item.ConfigFileParams[key] = appsv1alpha1.ParametersInFile{
		Parameters: params.Parameters,
		Content:    &content,
	}
}

func updateParameters(item *appsv1alpha1.ConfigurationItemDetail, key string, parameters []appsv1alpha1.ParameterPair, filter validate.ValidatorOptions) {
	updatedParams := make(map[string]*string, len(parameters))
	for _, parameter := range parameters {
		if filter(parameter.Key) {
			updatedParams[parameter.Key] = parameter.Value
		}
	}

	params, ok := item.ConfigFileParams[key]
	if !ok {
		item.ConfigFileParams[key] = appsv1alpha1.ParametersInFile{
			Parameters: updatedParams,
		}
		return
	}

	item.ConfigFileParams[key] = appsv1alpha1.ParametersInFile{
		Content:    params.Content,
		Parameters: mergeMaps(params.Parameters, updatedParams),
	}
}

func mergeMaps(m1 map[string]*string, m2 map[string]*string) map[string]*string {
	merged := make(map[string]*string)
	for key, value := range m1 {
		merged[key] = value
	}
	for key, value := range m2 {
		merged[key] = value
	}
	return merged
}

func hasFileUpdate(config appsv1alpha1.ConfigurationItem) bool {
	for _, key := range config.Keys {
		if key.FileContent != "" {
			return true
		}
	}
	return false
}
