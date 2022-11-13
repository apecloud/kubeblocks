/*
Copyright 2022.

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
}

func (s schemaValidator) Validate(cfg map[string]string) error {
	// TODO implement me
	return MakeError("not support schema validate.")
}

type EmptyValidator struct {
}

func (e EmptyValidator) Validate(cfg map[string]string) error {
	return nil
}

func NewConfigValidator(configTemplate *dbaasv1alpha1.ConfigurationTemplateSpec) ConfigValidator {
	var (
		meta      = configTemplate.ConfigurationSchema
		validator ConfigValidator
	)

	switch {
	case meta != nil:
		validator = &EmptyValidator{}
	case meta.Cue != nil:
		validator = &configCueValidator{
			cfgType:   configTemplate.Formatter,
			cueScript: *meta.Cue,
		}
	case meta.Schema != nil:
		validator = &schemaValidator{
			typeName: configTemplate.CfgSchemaTopLevelName,
			schema:   meta.Schema,
		}
	default:
		validator = &EmptyValidator{}
	}
	return validator
}
