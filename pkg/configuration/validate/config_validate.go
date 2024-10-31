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
	"github.com/StudioSol/set"
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

type cmKeySelector struct {
	// A ConfigMap object may contain multiple configuration files and only some of them can be parsed and verified by kubeblocks,
	// such as postgresql, there are two files pg_hba.conf & postgresql.conf in the ConfigMap, and we can only validate postgresql.conf,
	// so pg_hba.conf file needs to be ignored during when doing verification.
	// keySelector filters the keys in the configmap.
	keySelector []ValidatorOptions
}

type configCueValidator struct {
	cmKeySelector

	// cue describes configuration template
	cueScript string
	cfgType   parametersv1alpha1.CfgFileFormat
}

func (s *cmKeySelector) filter(key string) bool {
	if len(s.keySelector) == 0 {
		return false
	}

	for _, option := range s.keySelector {
		if !option(key) {
			return true
		}
	}
	return false
}

func (c *configCueValidator) Validate(content string) error {
	if c.cueScript == "" {
		return nil
	}
	return ValidateConfigWithCue(c.cueScript, c.cfgType, content)
}

type schemaValidator struct {
	cmKeySelector

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

func WithKeySelector(keys []string) ValidatorOptions {
	var sets *set.LinkedHashSetString
	if len(keys) > 0 {
		sets = core.FromCMKeysSelector(keys)
	}
	return func(key string) bool {
		return sets == nil || sets.InArray(key)
	}
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
