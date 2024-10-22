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

package parameters

import (
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockParametersDefinitionFactory struct {
	testapps.BaseFactory[parametersv1alpha1.ParametersDefinition, *parametersv1alpha1.ParametersDefinition, MockParametersDefinitionFactory]
}

func NewParametersDefinitionFactory(name string) *MockParametersDefinitionFactory {
	f := &MockParametersDefinitionFactory{}
	f.Init("", name, &parametersv1alpha1.ParametersDefinition{}, f)
	return f
}

func (f *MockParametersDefinitionFactory) Schema(cue string) *MockParametersDefinitionFactory {
	openAPISchema, _ := openapi.GenerateOpenAPISchema(cue, "")
	f.Get().Spec.ParametersSchema = &parametersv1alpha1.ParametersSchema{
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

func (f *MockParametersDefinitionFactory) SetReloadAction(action *parametersv1alpha1.ReloadAction) *MockParametersDefinitionFactory {
	f.Get().Spec.ReloadAction = action
	return f
}

func WithNoneAction() *parametersv1alpha1.ReloadAction {
	return &parametersv1alpha1.ReloadAction{
		AutoTrigger: &parametersv1alpha1.AutoTrigger{
			ProcessName: "",
		},
	}
}
