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

package controllerutil

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
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
			continue
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

// IsApplyConfigChanged checks if the configuration is changed
func IsApplyConfigChanged(configMap *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail) bool {
	if configMap == nil {
		return false
	}

	lastAppliedVersion, ok := configMap.Annotations[constant.ConfigAppliedVersionAnnotationKey]
	if !ok {
		return false
	}
	b, err := json.Marshal(item)
	return err == nil && string(b) == lastAppliedVersion
}

// IsRerender checks if the configuration template is changed
func IsRerender(configMap *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail) bool {
	if configMap == nil {
		return true
	}
	if item.Payload.Data == nil && item.CustomTemplates == nil {
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

// GetConfigSpecReconcilePhase gets the configuration phase
func GetConfigSpecReconcilePhase(configMap *corev1.ConfigMap,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	status *parametersv1alpha1.ConfigTemplateItemDetailStatus) parametersv1alpha1.ParameterPhase {
	if status == nil || status.Phase == "" {
		return parametersv1alpha1.CCreatingPhase
	}
	if !IsApplyConfigChanged(configMap, item) {
		return parametersv1alpha1.CPendingPhase
	}
	return status.Phase
}

func CheckAndPatchPayload(item *parametersv1alpha1.ConfigTemplateItemDetail, payloadID string, payload interface{}) (bool, error) {
	if item == nil {
		return false, nil
	}
	if item.Payload.Data == nil {
		item.Payload.Data = make(map[string]interface{})
	}
	oldPayload, ok := item.Payload.Data[payloadID]
	if !ok && payload == nil {
		return false, nil
	}
	if payload == nil {
		delete(item.Payload.Data, payloadID)
		return true, nil
	}
	newPayload, err := buildPayloadAsUnstructuredObject(payload)
	if err != nil {
		return false, err
	}
	if oldPayload != nil && reflect.DeepEqual(oldPayload, newPayload) {
		return false, nil
	}
	item.Payload.Data[payloadID] = newPayload
	return true, nil
}

func buildPayloadAsUnstructuredObject(payload interface{}) (interface{}, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var unstructuredObj any
	if err = json.Unmarshal(b, &unstructuredObj); err != nil {
		return nil, err
	}
	return unstructuredObj, nil
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

func ResolveCmpdParametersDefs(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (*parametersv1alpha1.ParameterDrivenConfigRender, []*parametersv1alpha1.ParametersDefinition, error) {
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
		paramsDefs = append(paramsDefs, paramsDef)
	}
	return configRender, paramsDefs, nil
}

func ResolveComponentConfigRender(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (*parametersv1alpha1.ParameterDrivenConfigRender, error) {
	configDefList := &parametersv1alpha1.ParameterDrivenConfigRenderList{}
	if err := reader.List(ctx, configDefList); err != nil {
		return nil, err
	}

	for i, item := range configDefList.Items {
		if item.Spec.ComponentDef != cmpd.Name {
			continue
		}
		if item.Spec.ServiceVersion == "" || item.Spec.ServiceVersion == cmpd.Spec.ServiceVersion {
			return &configDefList.Items[i], nil
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
