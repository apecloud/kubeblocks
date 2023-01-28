/*
Copyright ApeCloud Inc.

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
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/kube-openapi/pkg/validation/errors"
	kubeopenapispec "k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type ConfigValidator interface {
	Validate(cfg map[string]string) error
}

type configCueValidator struct {
	// cue describe configuration template
	cueScript string
	cfgType   dbaasv1alpha1.ConfigurationFormatter
}

func (c *configCueValidator) Validate(cfg map[string]string) error {
	if c.cueScript == "" {
		return nil
	}
	for _, content := range cfg {
		if err := ValidateConfigurationWithCue(c.cueScript, c.cfgType, content); err != nil {
			return err
		}
	}
	return nil
}

type schemaValidator struct {
	typeName string
	schema   *apiext.JSONSchemaProps
	cfgType  dbaasv1alpha1.ConfigurationFormatter
}

func (s schemaValidator) Validate(cfg map[string]string) error {
	openAPITypes := &kubeopenapispec.Schema{}
	validator := validate.NewSchemaValidator(openAPITypes, nil, "", strfmt.Default)
	for key, data := range cfg {
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

func NewConfigValidator(configTemplate *dbaasv1alpha1.ConfigConstraintSpec) ConfigValidator {
	var (
		validator    ConfigValidator
		configSchema = configTemplate.ConfigurationSchema
	)

	switch {
	case configSchema == nil:
		validator = &EmptyValidator{}
	case len(configSchema.CUE) != 0:
		validator = &configCueValidator{
			cfgType:   configTemplate.FormatterConfig.Formatter,
			cueScript: configSchema.CUE,
		}
	case configSchema.Schema != nil:
		validator = &schemaValidator{
			typeName: configTemplate.CfgSchemaTopLevelName,
			cfgType:  configTemplate.FormatterConfig.Formatter,
			schema:   configSchema.Schema,
		}
	default:
		validator = &EmptyValidator{}
	}
	return validator
}
