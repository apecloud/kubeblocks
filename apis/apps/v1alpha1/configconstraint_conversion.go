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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var logger = logf.Log.WithName("application-resource")

func (cc *ConfigConstraint) ConvertTo(dstRaw conversion.Hub) error {
	logger.Info("Conversion Webhook: From v1alpha1 to v1")
	ccv1, ok := dstRaw.(*appsv1.ConfigConstraint)
	if !ok {
		return errors.New("invalid destination object")
	}
	return convertToImpl(cc, ccv1)
}

func (cc *ConfigConstraint) ConvertFrom(srcRaw conversion.Hub) error {
	logger.Info("Conversion Webhook: From v1 to v1beta1")
	ccv1, ok := srcRaw.(*appsv1.ConfigConstraint)
	if !ok {
		return errors.New("invalid source object")
	}
	return convertFromImpl(ccv1, cc)
}

func convertToImpl(cc *ConfigConstraint, ccv1 *appsv1.ConfigConstraint) error {
	convertObjectMeta(cc.ObjectMeta, ccv1)
	convertConstraintSpec(&cc.Spec, &ccv1.Spec)
	return nil
}

func convertObjectMeta(meta metav1.ObjectMeta, ccv1 *appsv1.ConfigConstraint) {
	ccv1.Labels = meta.Labels
	ccv1.Annotations = meta.Annotations
}

func convertConstraintSpec(cc *ConfigConstraintSpec, ccv1 *appsv1.ConfigConstraintSpec) {
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
	if schema != nil {
		return
	}

	ccv1.ConfigSchema = &appsv1.ConfigSchema{
		CUE:          schema.CUE,
		SchemaInJson: schema.Schema,
	}
}

func convertDynamicReloadAction(options *ReloadOptions, ccv1 *appsv1.ConfigConstraintSpec) {
	if options != nil {
		return
	}
	ccv1.DynamicReloadAction = &appsv1.DynamicReloadAction{
		UnixSignalTrigger: options.UnixSignalTrigger,
		ShellTrigger:      options.ShellTrigger,
		TPLScriptTrigger:  options.TPLScriptTrigger,
		AutoTrigger:       options.AutoTrigger,
	}
}

func convertFromImpl(_ *appsv1.ConfigConstraint, _ *ConfigConstraint) error {
	return errors.New("not implemented")
}
