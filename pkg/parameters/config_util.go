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

package parameters

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/validate"
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
func IsApplyUpdatedParameters(configMap *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail, componentGeneration int64) bool {
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
	if !reflect.DeepEqual(lastItem, item) {
		return false
	}
	if componentGeneration == 0 {
		return true
	}
	appliedGeneration, ok := configMap.Annotations[constant.ConfigAppliedComponentGenerationKey]
	if !ok || appliedGeneration == "" {
		return false
	}
	return appliedGeneration == strconv.FormatInt(componentGeneration, 10)
}

// GetUpdatedParametersReconciledPhase gets the configuration phase
func GetUpdatedParametersReconciledPhase(configMap *corev1.ConfigMap,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	status *parametersv1alpha1.ConfigTemplateItemDetailStatus,
	componentGeneration int64) parametersv1alpha1.ParameterPhase {
	if status == nil || status.Phase == "" {
		return parametersv1alpha1.CCreatingPhase
	}
	if !IsApplyUpdatedParameters(configMap, item, componentGeneration) {
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
	paramsDefList := &parametersv1alpha1.ParametersDefinitionList{}
	if err := reader.List(ctx, paramsDefList); err != nil {
		return nil, nil, err
	}

	slices.SortFunc(paramsDefList.Items, func(a, b parametersv1alpha1.ParametersDefinition) int {
		if cmp := strings.Compare(b.Spec.ComponentDef, a.Spec.ComponentDef); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Spec.TemplateName, b.Spec.TemplateName); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Spec.FileName, b.Spec.FileName); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Name, b.Name)
	})

	var paramsDefs []*parametersv1alpha1.ParametersDefinition
	seenFiles := make(map[string]string)
	for i := range paramsDefList.Items {
		paramsDef := &paramsDefList.Items[i]
		matched, err := matchParametersDefinition(cmpd, paramsDef)
		if err != nil {
			return nil, nil, err
		}
		if !matched {
			continue
		}
		if paramsDef.Status.Phase != parametersv1alpha1.PDAvailablePhase {
			return nil, nil, fmt.Errorf("the referenced ParametersDefinition is unavailable: %s", paramsDef.Name)
		}
		if err := validateMatchedParametersDefinition(paramsDef); err != nil {
			return nil, nil, err
		}
		if existing, ok := seenFiles[paramsDef.Spec.FileName]; ok {
			return nil, nil, fmt.Errorf("config file[%s] has been defined in other parametersdefinition[%s]", paramsDef.Spec.FileName, existing)
		}
		seenFiles[paramsDef.Spec.FileName] = paramsDef.Name
		paramsDefs = append(paramsDefs, paramsDef)
	}
	if len(paramsDefs) == 0 {
		return nil, nil, nil
	}
	return BuildConfigRenderFromParametersDefs(cmpd, paramsDefs), paramsDefs, nil
}

func matchParametersDefinition(cmpd *appsv1.ComponentDefinition, paramsDef *parametersv1alpha1.ParametersDefinition) (bool, error) {
	if cmpd == nil || paramsDef == nil {
		return false, nil
	}
	pattern := paramsDef.Spec.ComponentDef
	if pattern == "" {
		return false, nil
	}
	if err := component.ValidateDefNameRegexp(pattern); err != nil {
		return false, fmt.Errorf("invalid parametersdefinition[%s] componentDef pattern %q: %w", paramsDef.Name, pattern, err)
	}
	if !component.PrefixOrRegexMatched(cmpd.Name, pattern) {
		return false, nil
	}
	return paramsDef.Spec.ServiceVersion == "" || paramsDef.Spec.ServiceVersion == cmpd.Spec.ServiceVersion, nil
}

func validateMatchedParametersDefinition(paramsDef *parametersv1alpha1.ParametersDefinition) error {
	if paramsDef.Spec.TemplateName == "" {
		return fmt.Errorf("parametersdefinition[%s] misses templateName", paramsDef.Name)
	}
	if paramsDef.Spec.FileName == "" {
		return fmt.Errorf("parametersdefinition[%s] misses fileName", paramsDef.Name)
	}
	if paramsDef.Spec.FileFormatConfig == nil {
		return fmt.Errorf("parametersdefinition[%s] misses fileFormatConfig", paramsDef.Name)
	}
	return nil
}

func BuildConfigRenderFromParametersDefs(cmpd *appsv1.ComponentDefinition, paramsDefs []*parametersv1alpha1.ParametersDefinition) *parametersv1alpha1.ParamConfigRenderer {
	if len(paramsDefs) == 0 {
		return nil
	}
	spec := parametersv1alpha1.ParamConfigRendererSpec{}
	if cmpd != nil {
		spec.ComponentDef = cmpd.Name
		spec.ServiceVersion = cmpd.Spec.ServiceVersion
	}
	spec.ParametersDefs = make([]string, 0, len(paramsDefs))
	spec.Configs = make([]parametersv1alpha1.ComponentConfigDescription, 0, len(paramsDefs))
	for _, paramsDef := range paramsDefs {
		spec.ParametersDefs = append(spec.ParametersDefs, paramsDef.Name)
		spec.Configs = append(spec.Configs, parametersv1alpha1.ComponentConfigDescription{
			Name:             paramsDef.Spec.FileName,
			TemplateName:     paramsDef.Spec.TemplateName,
			FileFormatConfig: paramsDef.Spec.FileFormatConfig.DeepCopy(),
		})
	}
	return &parametersv1alpha1.ParamConfigRenderer{
		Spec: spec,
	}
}

func ResolveComponentConfigRender(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (*parametersv1alpha1.ParamConfigRenderer, error) {
	configDefList := &parametersv1alpha1.ParamConfigRendererList{}
	if err := reader.List(ctx, configDefList); err != nil {
		return nil, err
	}
	slices.SortFunc(configDefList.Items, func(a, b parametersv1alpha1.ParamConfigRenderer) int {
		return strings.Compare(b.Spec.ComponentDef, a.Spec.ComponentDef)
	})

	checkAvailable := func(configDef parametersv1alpha1.ParamConfigRenderer) error {
		if configDef.Status.Phase != parametersv1alpha1.PDAvailablePhase {
			return fmt.Errorf("the referenced ParamConfigRenderer is unavailable: %s", configDef.Name)
		}
		return nil
	}

	for i, item := range configDefList.Items {
		if !component.PrefixOrRegexMatched(cmpd.Name, item.Spec.ComponentDef) {
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

func legacyConfigManagerRequired(pd *parametersv1alpha1.ParametersDefinitionSpec) bool {
	if pd == nil {
		return false
	}
	// Legacy config-manager retention is keyed off whether the parameters
	// definition still relies on the old reload-action mechanism at all.
	// The concrete reload execution path (for example, whether it is a
	// ShellTrigger that can be proxied through the cluster API reconfigure
	// action) is decided later in the reconfigure flow.
	return pd.ReloadAction != nil
}

func LegacyConfigManagerRequiredForParamsDefs(paramsDefs []*parametersv1alpha1.ParametersDefinition) bool {
	for _, pd := range paramsDefs {
		if pd != nil && legacyConfigManagerRequired(&pd.Spec) {
			return true
		}
	}
	return false
}

type LegacyConfigManagerRequirementState string

const (
	LegacyConfigManagerRequirementUnknown LegacyConfigManagerRequirementState = "unknown"
	LegacyConfigManagerRequirementKeep    LegacyConfigManagerRequirementState = "keep"
	LegacyConfigManagerRequirementCleanup LegacyConfigManagerRequirementState = "cleanup"
)

func LegacyConfigManagerRequirementStateForCluster(cluster *appsv1.Cluster) (LegacyConfigManagerRequirementState, error) {
	if cluster == nil || len(cluster.Annotations) == 0 {
		return LegacyConfigManagerRequirementUnknown, nil
	}
	raw, ok := cluster.Annotations[constant.LegacyConfigManagerRequiredAnnotationKey]
	if !ok || raw == "" {
		return LegacyConfigManagerRequirementUnknown, nil
	}
	required, err := strconv.ParseBool(raw)
	if err != nil {
		return LegacyConfigManagerRequirementUnknown, fmt.Errorf("failed to parse %s: %w", constant.LegacyConfigManagerRequiredAnnotationKey, err)
	}
	if required {
		return LegacyConfigManagerRequirementKeep, nil
	}
	return LegacyConfigManagerRequirementCleanup, nil
}

func LegacyConfigManagerRequiredForCluster(cluster *appsv1.Cluster) (bool, error) {
	state, err := LegacyConfigManagerRequirementStateForCluster(cluster)
	if err != nil {
		return false, err
	}
	if state == LegacyConfigManagerRequirementUnknown {
		return false, nil
	}
	return state == LegacyConfigManagerRequirementKeep, nil
}

func ResolveComponentTemplate(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (map[string]*corev1.ConfigMap, error) {
	tpls := make(map[string]*corev1.ConfigMap, len(cmpd.Spec.Configs))
	for _, config := range cmpd.Spec.Configs {
		cm := &corev1.ConfigMap{}
		if err := reader.Get(ctx, client.ObjectKey{Name: config.Template, Namespace: config.Namespace}, cm); err != nil {
			return nil, err
		}
		tpls[config.Name] = cm
	}
	return tpls, nil
}
