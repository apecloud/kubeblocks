/*
Copyright ApeCloud, Inc.

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

type ConfigValidator interface {
	Validate(cfg map[string]string) error
}

type ValidatorOptions = func(key string) bool

type configCueValidator struct {
	// cue describe configuration template
	cueScript string
	cfgType   appsv1alpha1.CfgFileFormat

	// configmap key selector
	keySelector []ValidatorOptions
}

func (c *configCueValidator) Validate(cfg map[string]string) error {
	if c.cueScript == "" {
		return nil
	}
	for key, content := range cfg {
		if keyFilter(c.keySelector, key) {
			continue
		}
		if err := ValidateConfigurationWithCue(c.cueScript, c.cfgType, content); err != nil {
			return err
		}
	}
	return nil
}

type schemaValidator struct {
	typeName string
	schema   *apiext.JSONSchemaProps
	cfgType  appsv1alpha1.CfgFileFormat

	// configmap key selector
	keySelector []ValidatorOptions
}

func keyFilter(options []ValidatorOptions, key string) bool {
	if len(options) == 0 {
		return false
	}

	for _, option := range options {
		if !option(key) {
			return true
		}
	}
	return false
}

func (s schemaValidator) Validate(cfg map[string]string) error {
	openAPITypes := &kubeopenapispec.Schema{}
	validator := validate.NewSchemaValidator(openAPITypes, nil, "", strfmt.Default)
	for key, data := range cfg {
		if keyFilter(s.keySelector, key) {
			continue
		}
		cfg, err := loadConfiguration(s.cfgType, data)
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

type EmptyValidator struct {
}

func (e EmptyValidator) Validate(_ map[string]string) error {
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

func NewConfigValidator(configTemplate *appsv1alpha1.ConfigConstraintSpec, options ...ValidatorOptions) ConfigValidator {
	var (
		validator    ConfigValidator
		configSchema = configTemplate.ConfigurationSchema
	)

	switch {
	case configSchema == nil:
		validator = &EmptyValidator{}
	case len(configSchema.CUE) != 0:
		validator = &configCueValidator{
			cfgType:     configTemplate.FormatterConfig.Format,
			cueScript:   configSchema.CUE,
			keySelector: options,
		}
	case configSchema.Schema != nil:
		validator = &schemaValidator{
			typeName:    configTemplate.CfgSchemaTopLevelName,
			cfgType:     configTemplate.FormatterConfig.Format,
			schema:      configSchema.Schema,
			keySelector: options,
		}
	default:
		validator = &EmptyValidator{}
	}
	return validator
}
