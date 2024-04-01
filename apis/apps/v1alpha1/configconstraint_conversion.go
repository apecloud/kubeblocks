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

package v1alpha1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var logger = logf.Log.WithName("application-resource")

func (cc *ConfigConstraint) ConvertTo(dstRaw conversion.Hub) error {
	logger.Info("Conversion Webhook: from v1alpha1 to v1", "name", cc.Name)
	ccv1, ok := dstRaw.(*appsv1.ConfigConstraint)
	if !ok {
		return errors.New("invalid destination object")
	}
	return convertToImpl(cc, ccv1)
}

func (cc *ConfigConstraint) ConvertFrom(srcRaw conversion.Hub) error {
	ccv1, ok := srcRaw.(*appsv1.ConfigConstraint)
	if !ok {
		return errors.New("invalid source object")
	}
	logger.Info("Conversion Webhook: from v1 to v1alpha1", "name", ccv1.Name)
	return convertFromImpl(ccv1, cc)
}

func convertToImpl(cc *ConfigConstraint, ccv1 *appsv1.ConfigConstraint) error {
	ccv1.ObjectMeta = cc.ObjectMeta
	if ccv1.Annotations == nil {
		ccv1.Annotations = make(map[string]string)
	}
	ccv1.Annotations[constant.KubeblocksAPIConversionTypeAnnotationName] = constant.MigratedAPIVersion
	ccv1.Annotations[constant.SourceAPIVersionAnnotationName] = GroupVersion.Version
	convertToConstraintSpec(&cc.Spec, &ccv1.Spec)
	return nil
}

func convertToConstraintSpec(cc *ConfigConstraintSpec, ccv1 *appsv1.ConfigConstraintSpec) {
	ccv1.DynamicActionCanBeMerged = cc.DynamicActionCanBeMerged
	ccv1.DynamicParameterSelectedPolicy = cc.DynamicParameterSelectedPolicy
	ccv1.ReloadToolsImage = cc.ToolsImageSpec
	ccv1.DownwardActions = cc.DownwardAPIOptions
	ccv1.ScriptConfigs = cc.ScriptConfigs
	ccv1.ConfigSchemaTopLevelKey = cc.CfgSchemaTopLevelName
	ccv1.StaticParameters = cc.StaticParameters
	ccv1.DynamicParameters = cc.DynamicParameters
	ccv1.ImmutableParameters = cc.ImmutableParameters
	ccv1.DynamicReloadSelector = cc.Selector
	ccv1.FormatterConfig = cc.FormatterConfig
	convertDynamicReloadAction(cc.ReloadOptions, ccv1)
	convertSchema(cc.ConfigurationSchema, ccv1)
}

func convertSchema(schema *CustomParametersValidation, ccv1 *appsv1.ConfigConstraintSpec) {
	if schema == nil {
		return
	}
	ccv1.ConfigSchema = &appsv1.ConfigSchema{
		CUE:          schema.CUE,
		SchemaInJSON: schema.Schema,
	}
}

func convertDynamicReloadAction(options *ReloadOptions, ccv1 *appsv1.ConfigConstraintSpec) {
	if options == nil {
		return
	}
	ccv1.DynamicReloadAction = &appsv1.DynamicReloadAction{
		UnixSignalTrigger: options.UnixSignalTrigger,
		ShellTrigger:      options.ShellTrigger,
		TPLScriptTrigger:  options.TPLScriptTrigger,
		AutoTrigger:       options.AutoTrigger,
	}
}

func convertFromImpl(ccv1 *appsv1.ConfigConstraint, cc *ConfigConstraint) error {
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

func convertFromConstraintSpec(ccv1 *appsv1.ConfigConstraintSpec, cc *ConfigConstraintSpec) {
	cc.DynamicActionCanBeMerged = ccv1.DynamicActionCanBeMerged
	cc.DynamicParameterSelectedPolicy = ccv1.DynamicParameterSelectedPolicy
	cc.ToolsImageSpec = ccv1.ReloadToolsImage
	cc.DownwardAPIOptions = ccv1.DownwardActions
	cc.ScriptConfigs = ccv1.ScriptConfigs
	cc.CfgSchemaTopLevelName = ccv1.ConfigSchemaTopLevelKey
	cc.StaticParameters = ccv1.StaticParameters
	cc.DynamicParameters = ccv1.DynamicParameters
	cc.ImmutableParameters = ccv1.ImmutableParameters
	cc.Selector = ccv1.DynamicReloadSelector
	cc.FormatterConfig = ccv1.FormatterConfig

	if ccv1.DynamicReloadAction != nil {
		cc.ReloadOptions = &ReloadOptions{
			UnixSignalTrigger: ccv1.DynamicReloadAction.UnixSignalTrigger,
			ShellTrigger:      ccv1.DynamicReloadAction.ShellTrigger,
			TPLScriptTrigger:  ccv1.DynamicReloadAction.TPLScriptTrigger,
			AutoTrigger:       ccv1.DynamicReloadAction.AutoTrigger,
		}
	}
	if ccv1.ConfigSchema != nil {
		cc.ConfigurationSchema = &CustomParametersValidation{
			Schema: ccv1.ConfigSchema.SchemaInJSON,
			CUE:    ccv1.ConfigSchema.CUE,
		}
	}
}
