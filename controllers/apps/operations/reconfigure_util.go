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

package operations

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type reconfiguringResult struct {
	failed               bool
	noFormatFilesUpdated bool
	configPatch          *core.ConfigPatchInfo
	lastAppliedConfigs   map[string]string
	err                  error
}

// type updateReconfigureStatus func(params []core.ParamPairs, orinalData map[string]string, formatter *appsv1alpha1.FormatterConfig) error

// Deprecated: use NewPipeline instead
// updateConfigConfigmapResource merges parameters of the config into the configmap, and verifies final configuration file.
// func updateConfigConfigmapResource(config appsv1alpha1.ConfigurationItem,
//	configSpec appsv1alpha1.ComponentConfigSpec,
//	cmKey client.ObjectKey,
//	ctx context.Context,
//	cli client.Client,
//	opsCrName string,
//	updater updateReconfigureStatus) reconfiguringResult {
//	var (
//		cm = &corev1.ConfigMap{}
//		cc = &appsv1alpha1.ConfigConstraint{}
//
//		err    error
//		newCfg map[string]string
//	)
//
//	if err := cli.Get(ctx, cmKey, cm); err != nil {
//		return makeReconfiguringResult(err)
//	}
//	if err := cli.Get(ctx, client.ObjectKey{
//		Namespace: configSpec.Namespace,
//		Name:      configSpec.ConfigConstraintRef,
//	}, cc); err != nil {
//		return makeReconfiguringResult(err)
//	}
//
//	updatedFiles := make(map[string]string, len(config.Keys))
//	updatedParams := make([]core.ParamPairs, 0, len(config.Keys))
//	for _, key := range config.Keys {
//		if key.FileContent != "" {
//			updatedFiles[key.Key] = key.FileContent
//		}
//		if len(key.Parameters) > 0 {
//			updatedParams = append(updatedParams, core.ParamPairs{
//				Key:           key.Key,
//				UpdatedParams: fromKeyValuePair(key.Parameters),
//			})
//		}
//	}
//
//	if newCfg, err = mergeUpdatedParams(cm.Data, updatedFiles, updatedParams, cc, configSpec); err != nil {
//		return makeReconfiguringResult(err, withFailed(true))
//	}
//	configPatch, restart, err := core.CreateConfigPatch(cm.Data, newCfg, cc.Spec.FormatterConfig.Format, configSpec.Keys, len(updatedFiles) != 0)
//	if err != nil {
//		return makeReconfiguringResult(err)
//	}
//	if !restart && !configPatch.IsModify {
//		return makeReconfiguringResult(nil, withReturned(newCfg, configPatch))
//	}
//	if updater != nil {
//		if err := updater(updatedParams, cm.Data, cc.Spec.FormatterConfig); err != nil {
//			return makeReconfiguringResult(err)
//		}
//	}
//
//	return makeReconfiguringResult(
//		syncConfigmap(cm, newCfg, cli, ctx, opsCrName, configSpec, &cc.Spec, config.Policy),
//		withReturned(newCfg, configPatch),
//		withNoFormatFilesUpdated(restart))
// }

// func mergeUpdatedParams(base map[string]string,
//	updatedFiles map[string]string,
//	updatedParams []core.ParamPairs,
//	cc *appsv1alpha1.ConfigConstraint,
//	tpl appsv1alpha1.ComponentConfigSpec) (map[string]string, error) {
//	updatedConfig := base
//
//	// merge updated files into configmap
//	if len(updatedFiles) != 0 {
//		return core.MergeUpdatedConfig(base, updatedFiles), nil
//	}
//	if cc == nil {
//		return updatedConfig, nil
//	}
//	return intctrlutil.MergeAndValidateConfigs(cc.Spec, updatedConfig, tpl.Keys, updatedParams)
// }

// func syncConfigmap(
//	cmObj *corev1.ConfigMap,
//	newCfg map[string]string,
//	cli client.Client,
//	ctx context.Context,
//	opsCrName string,
//	configSpec appsv1alpha1.ComponentConfigSpec,
//	cc *appsv1alpha1.ConfigConstraintSpec,
//	policy *appsv1alpha1.UpgradePolicy) error {
//
//	patch := client.MergeFrom(cmObj.DeepCopy())
//	cmObj.Data = newCfg
//	if cmObj.Annotations == nil {
//		cmObj.Annotations = make(map[string]string)
//	}
//	if policy != nil {
//		cmObj.Annotations[constant.UpgradePolicyAnnotationKey] = string(*policy)
//	}
//	cmObj.Annotations[constant.LastAppliedOpsCRAnnotationKey] = opsCrName
//	core.SetParametersUpdateSource(cmObj, constant.ReconfigureUserSource)
//	if err := configuration.SyncEnvConfigmap(configSpec, cmObj, cc, cli, ctx); err != nil {
//		return err
//	}
//	return cli.Patch(ctx, cmObj, patch)
// }

func updateOpsLabelWithReconfigure(obj *appsv1alpha1.OpsRequest, params []core.ParamPairs, orinalData map[string]string, formatter *appsv1alpha1.FormatterConfig) {
	var maxLabelCount = 16
	updateLabel := func(param map[string]interface{}) {
		if obj.Labels == nil {
			obj.Labels = make(map[string]string)
		}
		for key, val := range param {
			if maxLabelCount <= 0 {
				return
			}
			maxLabelCount--
			obj.Labels[key] = core.FromValueToString(val)
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

func fetchOriginalValue(keyFile, data string, params map[string]interface{}, formatter *appsv1alpha1.FormatterConfig) (string, error) {
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
	return &FastFailError{message: err.Error()}
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
		item.ConfigFileParams[key] = appsv1alpha1.ConfigParams{
			Content: &content,
		}
		return
	}
	item.ConfigFileParams[key] = appsv1alpha1.ConfigParams{
		Parameters: params.Parameters,
		Content:    &content,
	}
}

func updateParameters(item *appsv1alpha1.ConfigurationItemDetail, key string, parameters []appsv1alpha1.ParameterPair) {
	updatedParams := make(map[string]*string, len(parameters))
	for _, parameter := range parameters {
		updatedParams[parameter.Key] = parameter.Value
	}

	params, ok := item.ConfigFileParams[key]
	if !ok {
		item.ConfigFileParams[key] = appsv1alpha1.ConfigParams{
			Parameters: updatedParams,
		}
		return
	}

	item.ConfigFileParams[key] = appsv1alpha1.ConfigParams{
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
