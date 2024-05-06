/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var logger = logf.Log.WithName("application-resource")

func (cc *ConfigConstraint) ConvertTo(dstRaw conversion.Hub) error {
	logger.Info("Conversion Webhook: from v1alpha1 to v1", "name", cc.Name)
	ccv1, ok := dstRaw.(*appsv1beta1.ConfigConstraint)
	if !ok {
		return errors.New("invalid destination object")
	}
	return convertToImpl(cc, ccv1)
}

func (cc *ConfigConstraint) ConvertFrom(srcRaw conversion.Hub) error {
	ccv1, ok := srcRaw.(*appsv1beta1.ConfigConstraint)
	if !ok {
		return errors.New("invalid source object")
	}
	logger.Info("Conversion Webhook: from v1 to v1alpha1", "name", ccv1.Name)
	return convertFromImpl(ccv1, cc)
}

func convertToImpl(cc *ConfigConstraint, ccv1 *appsv1beta1.ConfigConstraint) error {
	ccv1.ObjectMeta = cc.ObjectMeta
	if ccv1.Annotations == nil {
		ccv1.Annotations = make(map[string]string)
	}
	ccv1.Annotations[constant.KubeblocksAPIConversionTypeAnnotationName] = constant.MigratedAPIVersion
	ccv1.Annotations[constant.SourceAPIVersionAnnotationName] = GroupVersion.Version
	convertToConstraintSpec(&cc.Spec, &ccv1.Spec)
	return nil
}

func convertToConstraintSpec(cc *ConfigConstraintSpec, ccv1 *appsv1beta1.ConfigConstraintSpec) {
	ccv1.MergeReloadAndRestart = cc.DynamicActionCanBeMerged
	ccv1.ReloadStaticParamsBeforeRestart = cc.DynamicParameterSelectedPolicy
	ccv1.ToolsSetup = cc.ToolsImageSpec
	ccv1.DownwardAPITriggeredActions = cc.DownwardAPIOptions
	ccv1.ScriptConfigs = cc.ScriptConfigs
	ccv1.ConfigSchemaTopLevelKey = cc.CfgSchemaTopLevelName
	ccv1.StaticParameters = cc.StaticParameters
	ccv1.DynamicParameters = cc.DynamicParameters
	ccv1.ImmutableParameters = cc.ImmutableParameters
	ccv1.ReloadedPodSelector = cc.Selector
	ccv1.FileFormatConfig = cc.FormatterConfig
	convertDynamicReloadAction(cc.ReloadOptions, ccv1)
	convertSchema(cc.ConfigurationSchema, ccv1)
}

func convertSchema(schema *CustomParametersValidation, ccv1 *appsv1beta1.ConfigConstraintSpec) {
	if schema == nil {
		return
	}
	ccv1.ConfigSchema = &appsv1beta1.ConfigSchema{
		CUE:          schema.CUE,
		SchemaInJSON: schema.Schema,
	}
}

func convertDynamicReloadAction(options *ReloadOptions, ccv1 *appsv1beta1.ConfigConstraintSpec) {
	if options == nil {
		return
	}
	ccv1.ReloadAction = &appsv1beta1.ReloadAction{
		UnixSignalTrigger: options.UnixSignalTrigger,
		ShellTrigger:      options.ShellTrigger,
		TPLScriptTrigger:  options.TPLScriptTrigger,
		AutoTrigger:       options.AutoTrigger,
	}
}

func convertFromImpl(ccv1 *appsv1beta1.ConfigConstraint, cc *ConfigConstraint) error {
	cc.ObjectMeta = ccv1.ObjectMeta
	if cc.Annotations == nil {
		cc.Annotations = make(map[string]string)
	}

	vType, ok := ccv1.Annotations[constant.KubeblocksAPIConversionTypeAnnotationName]
	if ok && vType == constant.MigratedAPIVersion && ccv1.Annotations[constant.SourceAPIVersionAnnotationName] == GroupVersion.Version {
		cc.Annotations[constant.KubeblocksAPIConversionTypeAnnotationName] = constant.SourceAPIVersion
	} else {
		cc.Annotations[constant.KubeblocksAPIConversionTypeAnnotationName] = constant.ReviewAPIVersion
	}

	convertFromConstraintSpec(&ccv1.Spec, &cc.Spec)
	return nil
}

func convertFromConstraintSpec(ccv1 *appsv1beta1.ConfigConstraintSpec, cc *ConfigConstraintSpec) {
	cc.DynamicActionCanBeMerged = ccv1.MergeReloadAndRestart
	cc.DynamicParameterSelectedPolicy = ccv1.ReloadStaticParamsBeforeRestart
	cc.ToolsImageSpec = ccv1.ToolsSetup
	cc.DownwardAPIOptions = ccv1.DownwardAPITriggeredActions
	cc.ScriptConfigs = ccv1.ScriptConfigs
	cc.CfgSchemaTopLevelName = ccv1.ConfigSchemaTopLevelKey
	cc.StaticParameters = ccv1.StaticParameters
	cc.DynamicParameters = ccv1.DynamicParameters
	cc.ImmutableParameters = ccv1.ImmutableParameters
	cc.Selector = ccv1.ReloadedPodSelector
	cc.FormatterConfig = ccv1.FileFormatConfig

	if ccv1.ReloadAction != nil {
		cc.ReloadOptions = &ReloadOptions{
			UnixSignalTrigger: ccv1.ReloadAction.UnixSignalTrigger,
			ShellTrigger:      ccv1.ReloadAction.ShellTrigger,
			TPLScriptTrigger:  ccv1.ReloadAction.TPLScriptTrigger,
			AutoTrigger:       ccv1.ReloadAction.AutoTrigger,
		}
	}
	if ccv1.ConfigSchema != nil {
		cc.ConfigurationSchema = &CustomParametersValidation{
			Schema: ccv1.ConfigSchema.SchemaInJSON,
			CUE:    ccv1.ConfigSchema.CUE,
		}
	}
}
