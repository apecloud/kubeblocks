/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"fmt"
	"reflect"
	"slices"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type Result struct {
	Phase      parametersv1alpha1.ParameterPhase `json:"phase"`
	Revision   string                            `json:"revision"`
	Policy     string                            `json:"policy"`
	ExecResult string                            `json:"execResult"`

	SucceedCount  int32 `json:"succeedCount"`
	ExpectedCount int32 `json:"expectedCount"`

	Retry   bool   `json:"retry"`
	Failed  bool   `json:"failed"`
	Message string `json:"message"`
}

// MergeAndValidateConfigs merges and validates configuration files
func MergeAndValidateConfigs(baseConfigs map[string]string,
	updatedParams []core.ParamPairs,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configDescs []parametersv1alpha1.ComponentConfigDescription) (map[string]string, error) {
	var (
		err             error
		newCfg          map[string]string
		configOperator  core.ConfigOperator
		updatedFileList []string
	)

	configLoaderOption := core.CfgOption{
		Type:           core.CfgCmType,
		Log:            log.FromContext(context.TODO()),
		FileFormatFn:   core.WithConfigFileFormat(configDescs),
		ConfigResource: core.FromConfigData(baseConfigs, core.NewConfigFileFilter(configDescs)),
	}
	if configOperator, err = core.NewConfigLoader(configLoaderOption); err != nil {
		return nil, err
	}

	// merge param to config file
	for _, params := range updatedParams {
		validUpdatedParameters := filterImmutableParameters(params.UpdatedParams, params.Key, paramsDefs)
		if len(validUpdatedParameters) == 0 {
			continue
		}
		fc := core.ResolveConfigFormat(configDescs, params.Key)
		if fc == nil {
			return nil, fmt.Errorf("not support the config updated: %s", params.Key)
		}
		if err = configOperator.MergeFrom(validUpdatedParameters, core.NewCfgOptions(params.Key, core.WithFormatterConfig(fc))); err != nil {
			return nil, err
		}
		updatedFileList = append(updatedFileList, params.Key)
	}

	if newCfg, err = configOperator.ToCfgContent(); err != nil {
		return nil, core.WrapError(err, "failed to generate config file")
	}

	// The ToCfgContent interface returns the file contents of all keys, the configuration file is encoded and decoded into keys,
	// the content may be different with the original file, such as comments, blank lines, etc,
	// in order to minimize the impact on the original file, only update the changed part.
	updatedCfgFiles := make(map[string]string, len(updatedFileList))
	for _, key := range updatedFileList {
		updatedCfgFiles[key] = newCfg[key]
		paramsDef := resolveParametersDef(paramsDefs, key)
		if paramsDef == nil {
			continue
		}
		fc := core.ResolveConfigFormat(configDescs, key)
		if fc == nil {
			continue
		}
		if err = validate.NewConfigValidator(paramsDef.Spec.ParametersSchema, fc).Validate(updatedCfgFiles[key]); err != nil {
			return nil, core.WrapError(err, "failed to validate updated config")
		}
	}

	return core.MergeUpdatedConfig(baseConfigs, updatedCfgFiles), nil
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

// IsApplyUpdatedParameters checks if the configuration is changed
func IsApplyUpdatedParameters(configMap *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail) bool {
	if configMap == nil {
		return false
	}

	lastAppliedVersion, ok := configMap.Annotations[constant.ConfigAppliedVersionAnnotationKey]
	if !ok {
		return false
	}
	lastItem := parametersv1alpha1.ConfigTemplateItemDetail{}
	if err := json.Unmarshal([]byte(lastAppliedVersion), &lastItem); err != nil {
		return false
	}
	return reflect.DeepEqual(lastItem, item)
}

// IsRerender checks if the configuration template is changed
func IsRerender(configMap *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail) bool {
	if configMap == nil {
		return true
	}
	if len(item.Payload) == 0 && item.CustomTemplates == nil {
		return false
	}

	var updatedVersion parametersv1alpha1.ConfigTemplateItemDetail
	updatedVersionStr, ok := configMap.Annotations[constant.ConfigAppliedVersionAnnotationKey]
	if ok && updatedVersionStr != "" {
		if err := json.Unmarshal([]byte(updatedVersionStr), &updatedVersion); err != nil {
			return false
		}
	}
	return !reflect.DeepEqual(updatedVersion.Payload, item.Payload) ||
		!reflect.DeepEqual(updatedVersion.CustomTemplates, item.CustomTemplates)
}

// GetUpdatedParametersReconciledPhase gets the configuration phase
func GetUpdatedParametersReconciledPhase(configMap *corev1.ConfigMap,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	status *parametersv1alpha1.ConfigTemplateItemDetailStatus) parametersv1alpha1.ParameterPhase {
	if status == nil || status.Phase == "" {
		return parametersv1alpha1.CCreatingPhase
	}
	if !IsApplyUpdatedParameters(configMap, item) {
		return parametersv1alpha1.CPendingPhase
	}
	if status.Phase == parametersv1alpha1.CFinishedPhase {
		// Check if the cr subresource (status) is the last version.
		lastRevision, ok := configMap.Annotations[constant.ConfigurationRevision]
		if !ok || status.UpdateRevision != lastRevision {
			return parametersv1alpha1.CRunningPhase
		}
	}
	return status.Phase
}

func CheckAndPatchPayload(item *parametersv1alpha1.ConfigTemplateItemDetail, payloadID string, payload interface{}) (bool, error) {
	if item == nil {
		return false, nil
	}
	if item.Payload == nil {
		item.Payload = make(map[string]json.RawMessage)
	}
	oldPayload, ok := item.Payload[payloadID]
	if !ok && payload == nil {
		return false, nil
	}
	if payload == nil {
		delete(item.Payload, payloadID)
		return true, nil
	}
	newPayload, err := buildPayloadAsUnstructuredObject(payload)
	if err != nil {
		return false, err
	}
	if oldPayload != nil && reflect.DeepEqual(oldPayload, newPayload) {
		return false, nil
	}
	item.Payload[payloadID] = newPayload
	return true, nil
}

func buildPayloadAsUnstructuredObject(payload interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ResourcesPayloadForComponent(resources corev1.ResourceRequirements) any {
	if len(resources.Requests) == 0 && len(resources.Limits) == 0 {
		return nil
	}

	return map[string]any{
		"limits":   resources.Limits,
		"requests": resources.Requests,
	}
}

func resolveParametersDef(paramsDefs []*parametersv1alpha1.ParametersDefinition, fileName string) *parametersv1alpha1.ParametersDefinition {
	pos := generics.FindFirstFunc(paramsDefs, func(paramsDef *parametersv1alpha1.ParametersDefinition) bool {
		return paramsDef.Spec.FileName == fileName
	})
	if pos >= 0 {
		return paramsDefs[pos]
	}
	return nil
}

func filterImmutableParameters(parameters map[string]any, fileName string, paramsDefs []*parametersv1alpha1.ParametersDefinition) map[string]any {
	paramsDef := resolveParametersDef(paramsDefs, fileName)
	if paramsDef == nil || len(paramsDef.Spec.ImmutableParameters) == 0 {
		return parameters
	}

	immutableParams := paramsDef.Spec.ImmutableParameters
	validParameters := make(map[string]any, len(parameters))
	for key, val := range parameters {
		if !slices.Contains(immutableParams, key) {
			validParameters[key] = val
		}
	}
	return validParameters
}

func ResolveCmpdParametersDefs(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (*parametersv1alpha1.ParamConfigRenderer, []*parametersv1alpha1.ParametersDefinition, error) {
	var paramsDefs []*parametersv1alpha1.ParametersDefinition

	configRender, err := ResolveComponentConfigRender(ctx, reader, cmpd)
	if err != nil {
		return nil, nil, err
	}
	if configRender == nil || len(configRender.Spec.ParametersDefs) == 0 {
		return configRender, nil, nil
	}
	for _, defName := range configRender.Spec.ParametersDefs {
		paramsDef := &parametersv1alpha1.ParametersDefinition{}
		if err = reader.Get(ctx, client.ObjectKey{Name: defName}, paramsDef); err != nil {
			return nil, nil, err
		}
		if paramsDef.Status.Phase != parametersv1alpha1.PDAvailablePhase {
			return nil, nil, fmt.Errorf("the referenced ParametersDefinition is unavailable: %s", paramsDef.Name)
		}
		paramsDefs = append(paramsDefs, paramsDef)
	}
	return configRender, paramsDefs, nil
}

func ResolveComponentConfigRender(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (*parametersv1alpha1.ParamConfigRenderer, error) {
	configDefList := &parametersv1alpha1.ParamConfigRendererList{}
	if err := reader.List(ctx, configDefList); err != nil {
		return nil, err
	}

	checkAvailable := func(configDef parametersv1alpha1.ParamConfigRenderer) error {
		if configDef.Status.Phase != parametersv1alpha1.PDAvailablePhase {
			return fmt.Errorf("the referenced ParamConfigRenderer is unavailable: %s", configDef.Name)
		}
		return nil
	}

	for i, item := range configDefList.Items {
		if item.Spec.ComponentDef != cmpd.Name {
			continue
		}
		if item.Spec.ServiceVersion == "" || item.Spec.ServiceVersion == cmpd.Spec.ServiceVersion {
			return &configDefList.Items[i], checkAvailable(item)
		}
	}
	return nil, nil
}

func NeedDynamicReloadAction(pd *parametersv1alpha1.ParametersDefinitionSpec) bool {
	if pd.MergeReloadAndRestart != nil {
		return !*pd.MergeReloadAndRestart
	}
	return false
}

func ReloadStaticParameters(pd *parametersv1alpha1.ParametersDefinitionSpec) bool {
	if pd.ReloadStaticParamsBeforeRestart != nil {
		return *pd.ReloadStaticParamsBeforeRestart
	}
	return false
}

func TransformComponentParameters(params []opsv1alpha1.ParameterPair) parametersv1alpha1.ComponentParameters {
	ret := make(parametersv1alpha1.ComponentParameters, len(params))
	for _, param := range params {
		ret[param.Key] = param.Value
	}
	return ret
}
