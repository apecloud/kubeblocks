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

package validate

import (
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/kube-openapi/pkg/validation/errors"
	kubeopenapispec "k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type ValidatorOptions = func(key string) bool

type ConfigValidator interface {
	Validate(data string) error
}

type configCueValidator struct {
	// cue describes configuration template
	cueScript string
	cfgType   parametersv1alpha1.CfgFileFormat
}

func (c *configCueValidator) Validate(content string) error {
	if c.cueScript == "" {
		return nil
	}
	return ValidateConfigWithCue(c.cueScript, c.cfgType, content)
}

type schemaValidator struct {
	typeName string
	schema   *apiext.JSONSchemaProps
	cfgType  parametersv1alpha1.CfgFileFormat
}

func (s *schemaValidator) Validate(content string) error {
	var err error
	var parameters map[string]interface{}

	openAPITypes := &kubeopenapispec.Schema{}
	validator := validate.NewSchemaValidator(openAPITypes, nil, "", strfmt.Default)
	if parameters, err = LoadConfigObjectFromContent(s.cfgType, content); err != nil {
		return err
	}
	if res := validator.Validate(parameters); res.HasErrors() {
		return core.WrapError(errors.CompositeValidationError(res.Errors...), "failed to schema validate for config file")
	}
	return nil
}

type emptyValidator struct {
}

func (e emptyValidator) Validate(_ string) error {
	return nil
}

func NewConfigValidator(paramsSchema *parametersv1alpha1.ParametersSchema, fileFormat *parametersv1alpha1.FileFormatConfig) ConfigValidator {
	if fileFormat == nil {
		return &emptyValidator{}
	}

	var validator ConfigValidator
	switch {
	case paramsSchema == nil:
		validator = &emptyValidator{}
	case len(paramsSchema.CUE) != 0:
		validator = &configCueValidator{
			cfgType:   fileFormat.Format,
			cueScript: paramsSchema.CUE,
		}
	case paramsSchema.SchemaInJSON != nil:
		validator = &schemaValidator{
			typeName: paramsSchema.TopLevelKey,
			cfgType:  fileFormat.Format,
			schema:   paramsSchema.SchemaInJSON,
		}
	default:
		validator = &emptyValidator{}
	}
	return validator
}
