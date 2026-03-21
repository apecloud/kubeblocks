/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockParametersDefinitionFactory struct {
	testapps.BaseFactory[parametersv1alpha1.ParametersDefinition, *parametersv1alpha1.ParametersDefinition, MockParametersDefinitionFactory]
}

func NewParametersDefinitionFactory(name string) *MockParametersDefinitionFactory {
	f := &MockParametersDefinitionFactory{}
	f.Init("", name, &parametersv1alpha1.ParametersDefinition{
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			FileName: MysqlConfigFile,
			FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
				FormatterAction: parametersv1alpha1.FormatterAction{
					IniConfig: &parametersv1alpha1.IniConfig{
						SectionName: "mysqld",
					},
				},
			},
		},
	}, f)
	return f
}

func (f *MockParametersDefinitionFactory) SetComponentDefinition(name string) *MockParametersDefinitionFactory {
	f.Get().Spec.ComponentDef = name
	return f
}

func (f *MockParametersDefinitionFactory) SetServiceVersion(version string) *MockParametersDefinitionFactory {
	f.Get().Spec.ServiceVersion = version
	return f
}

func (f *MockParametersDefinitionFactory) SetTemplateName(name string) *MockParametersDefinitionFactory {
	f.Get().Spec.TemplateName = name
	return f
}

func (f *MockParametersDefinitionFactory) SetConfigFile(name string) *MockParametersDefinitionFactory {
	f.Get().Spec.FileName = name
	return f
}

func (f *MockParametersDefinitionFactory) SetFileFormatConfig(cfg parametersv1alpha1.FileFormatConfig) *MockParametersDefinitionFactory {
	f.Get().Spec.FileFormatConfig = &cfg
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
