/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/apecloud/kubeblocks/internal/unstructured"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// CueType define cue type
// +enum
type CueType string

const (
	NullableType            CueType = "nullable"
	FloatType               CueType = "float"
	IntType                 CueType = "integer"
	BoolType                CueType = "boolean"
	StringType              CueType = "string"
	StructType              CueType = "object"
	ListType                CueType = "array"
	K8SQuantityType         CueType = "quantity"
	ClassicStorageType      CueType = "storage"
	ClassicTimeDurationType CueType = "timeDuration"
)

// CueValidate cue validate
func CueValidate(cueTpl string) error {
	if len(cueTpl) == 0 {
		return nil
	}

	context := cuecontext.New()
	tpl := context.CompileString(cueTpl)
	return tpl.Validate()
}

func ValidateConfigurationWithCue(cueTpl string, cfgType appsv1alpha1.CfgFileFormat, rawData string) error {
	cfg, err := loadConfigObjectFromContent(cfgType, rawData)
	if err != nil {
		return WrapError(err, "failed to load configuration. [%s]", rawData)
	}

	return unstructuredDataValidateByCue(cueTpl, cfg, cfgType == appsv1alpha1.Properties)
}

func loadConfigObjectFromContent(cfgType appsv1alpha1.CfgFileFormat, rawData string) (map[string]interface{}, error) {
	configObject, err := unstructured.LoadConfig("validate", rawData, cfgType)
	if err != nil {
		return nil, err
	}

	return configObject.GetAllParameters(), nil
}

func unstructuredDataValidateByCue(cueTpl string, data interface{}, trimString bool) error {
	defaultValidatePath := "configuration"
	context := cuecontext.New()
	tpl := context.CompileString(cueTpl)
	if err := tpl.Err(); err != nil {
		return err
	}

	if err := processCfgNotStringParam(data, context, tpl, trimString); err != nil {
		return err
	}

	var paths []string
	cueValue := tpl.LookupPath(cue.ParsePath(defaultValidatePath))
	if cueValue.Err() == nil {
		paths = []string{defaultValidatePath}
	}

	tpl = tpl.Fill(data, paths...)
	if err := tpl.Err(); err != nil {
		return WrapError(err, "failed to cue template render configure")
	}

	return tpl.Validate()
}
