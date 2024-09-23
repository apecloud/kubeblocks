/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	configv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
)

type MockParametersDefinitionFactory struct {
	BaseFactory[configv1alpha1.ParametersDefinition, *configv1alpha1.ParametersDefinition, MockParametersDefinitionFactory]
}

func NewParametersDefinitionFactory(name string) *MockParametersDefinitionFactory {
	f := &MockParametersDefinitionFactory{}
	f.Init("", name,
		&configv1alpha1.ParametersDefinition{
			Spec: configv1alpha1.ParametersDefinitionSpec{
				FileFormatConfig: &configv1alpha1.FileFormatConfig{
					Format: configv1alpha1.Ini,
					FormatterAction: configv1alpha1.FormatterAction{
						IniConfig: &configv1alpha1.IniConfig{
							SectionName: "mysql",
						},
					},
				},
			},
		}, f)
	return f
}

func (f *MockParametersDefinitionFactory) Schema(cue string) *MockParametersDefinitionFactory {
	openAPISchema, _ := openapi.GenerateOpenAPISchema(cue, "")
	f.Get().Spec.ParametersSchema = &configv1alpha1.ParametersSchema{
		CUE:          cue,
		SchemaInJSON: openAPISchema,
	}
	return f
}

func (f *MockParametersDefinitionFactory) StaticParameters(params []string) *MockParametersDefinitionFactory {
	f.Get().Spec.StaticParameters = params
	return f
}

func (f *MockParametersDefinitionFactory) DynamicParameters(params []string) *MockParametersDefinitionFactory {
	f.Get().Spec.DynamicParameters = params
	return f
}

func (f *MockParametersDefinitionFactory) ImmutableParameters(params []string) *MockParametersDefinitionFactory {
	f.Get().Spec.ImmutableParameters = params
	return f
}

func (f *MockParametersDefinitionFactory) SetReloadAction(action *configv1alpha1.ReloadAction) *MockParametersDefinitionFactory {
	f.Get().Spec.ReloadAction = action
	return f
}

func WithNoneAction() *configv1alpha1.ReloadAction {
	return &configv1alpha1.ReloadAction{
		AutoTrigger: &configv1alpha1.AutoTrigger{
			ProcessName: "",
		},
	}
}
