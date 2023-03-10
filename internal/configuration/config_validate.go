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

type ValidatorOptions = func(key string) bool

type ConfigValidator interface {
	Validate(cfg map[string]string) error
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

func (c *configCueValidator) Validate(cfg map[string]string) error {
	if c.cueScript == "" {
		return nil
	}
	for key, content := range cfg {
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

func (s *schemaValidator) Validate(cfg map[string]string) error {
	openAPITypes := &kubeopenapispec.Schema{}
	validator := validate.NewSchemaValidator(openAPITypes, nil, "", strfmt.Default)
	for key, data := range cfg {
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

func NewConfigValidator(configTemplate *appsv1alpha1.ConfigConstraintSpec, options ...ValidatorOptions) ConfigValidator {
	var (
		validator    ConfigValidator
		configSchema = configTemplate.ConfigurationSchema
	)

	switch {
	case configSchema == nil:
		validator = &emptyValidator{}
	case len(configSchema.CUE) != 0:
		validator = &configCueValidator{
			cmKeySelector: cmKeySelector{
				keySelector: options,
			},
			cfgType:   configTemplate.FormatterConfig.Format,
			cueScript: configSchema.CUE,
		}
	case configSchema.Schema != nil:
		validator = &schemaValidator{
			cmKeySelector: cmKeySelector{
				keySelector: options,
			},
			typeName: configTemplate.CfgSchemaTopLevelName,
			cfgType:  configTemplate.FormatterConfig.Format,
			schema:   configSchema.Schema,
		}
	default:
		validator = &emptyValidator{}
	}
	return validator
}
