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
	"github.com/StudioSol/set"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/kube-openapi/pkg/validation/errors"
	kubeopenapispec "k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ValidatorOptions = func(key string) bool

type ConfigValidator interface {
	Validate(data map[string]string) error
}

type cmKeySelector struct {
	// A ConfigMap object may contain multiple configuration files and only some configuration files can recognize their format and verify their by kubeblocks,
	// such as pg, there are two files, pg_hba.conf and postgresql.conf in the ConfigMap, we can only validate postgresql.conf,
	// thus pg_hba.conf file needs to be ignored during the verification.
	// keySelector is used to filter the keys in the configmap.
	keySelector []ValidatorOptions
}

type configCueValidator struct {
	cmKeySelector

	// cue describe configuration template
	cueScript string
	cfgType   appsv1alpha1.CfgFileFormat
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

func (c *configCueValidator) Validate(data map[string]string) error {
	if c.cueScript == "" {
		return nil
	}
	for key, content := range data {
		if c.filter(key) {
			continue
		}
		if err := ValidateConfigurationWithCue(c.cueScript, c.cfgType, content); err != nil {
			return err
		}
	}
	return nil
}

type schemaValidator struct {
	cmKeySelector

	typeName string
	schema   *apiext.JSONSchemaProps
	cfgType  appsv1alpha1.CfgFileFormat
}

func (s *schemaValidator) Validate(data map[string]string) error {
	openAPITypes := &kubeopenapispec.Schema{}
	validator := validate.NewSchemaValidator(openAPITypes, nil, "", strfmt.Default)
	for key, data := range data {
		if s.filter(key) {
			continue
		}
		cfg, err := loadConfigObjectFromContent(s.cfgType, data)
		if err != nil {
			return err
		}
		res := validator.Validate(cfg)
		if res.HasErrors() {
			return WrapError(errors.CompositeValidationError(res.Errors...), "failed to schema validate for cfg: %s", key)
		}
	}
	return nil
}

type emptyValidator struct {
}

func (e emptyValidator) Validate(_ map[string]string) error {
	return nil
}

func WithKeySelector(keys []string) ValidatorOptions {
	var sets *set.LinkedHashSetString
	if len(keys) > 0 {
		sets = FromCMKeysSelector(keys)
	}
	return func(key string) bool {
		return sets == nil || sets.InArray(key)
	}
}

func NewConfigValidator(configConstraint *appsv1alpha1.ConfigConstraintSpec, options ...ValidatorOptions) ConfigValidator {
	var (
		validator    ConfigValidator
		configSchema = configConstraint.ConfigurationSchema
	)

	switch {
	case configSchema == nil:
		validator = &emptyValidator{}
	case len(configSchema.CUE) != 0:
		validator = &configCueValidator{
			cmKeySelector: cmKeySelector{
				keySelector: options,
			},
			cfgType:   configConstraint.FormatterConfig.Format,
			cueScript: configSchema.CUE,
		}
	case configSchema.Schema != nil:
		validator = &schemaValidator{
			cmKeySelector: cmKeySelector{
				keySelector: options,
			},
			typeName: configConstraint.CfgSchemaTopLevelName,
			cfgType:  configConstraint.FormatterConfig.Format,
			schema:   configSchema.Schema,
		}
	default:
		validator = &emptyValidator{}
	}
	return validator
}
