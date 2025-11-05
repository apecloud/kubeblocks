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
	"github.com/apecloud/kubeblocks/pkg/parameters"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockParamConfigRendererFactory struct {
	testapps.BaseFactory[parametersv1alpha1.ParamConfigRenderer, *parametersv1alpha1.ParamConfigRenderer, MockParamConfigRendererFactory]
}

func NewParamConfigRendererFactory(name string) *MockParamConfigRendererFactory {
	f := &MockParamConfigRendererFactory{}
	f.Init("", name, &parametersv1alpha1.ParamConfigRenderer{
		Spec: parametersv1alpha1.ParamConfigRendererSpec{
			Configs: []parametersv1alpha1.ComponentConfigDescription{
				{
					Name: MysqlConfigFile,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.Ini,
						FormatterAction: parametersv1alpha1.FormatterAction{
							IniConfig: &parametersv1alpha1.IniConfig{
								SectionName: "mysqld",
							},
						},
					},
				},
			},
		},
	}, f)
	return f
}

func (f *MockParamConfigRendererFactory) SetParametersDefs(paramsDefs ...string) *MockParamConfigRendererFactory {
	f.Get().Spec.ParametersDefs = paramsDefs
	return f
}

func (f *MockParamConfigRendererFactory) SetComponentDefinition(cmpd string) *MockParamConfigRendererFactory {
	f.Get().Spec.ComponentDef = cmpd
	return f
}

func (f *MockParamConfigRendererFactory) safeGetConfigDescription(key string) *parametersv1alpha1.ComponentConfigDescription {
	desc := parameters.GetComponentConfigDescription(&f.Get().Spec, key)
	if desc != nil {
		return desc
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, parametersv1alpha1.ComponentConfigDescription{
		Name: key,
	})
	return parameters.GetComponentConfigDescription(&f.Get().Spec, key)
}

func (f *MockParamConfigRendererFactory) SetConfigDescription(key, tpl string, formatter parametersv1alpha1.FileFormatConfig) *MockParamConfigRendererFactory {
	desc := f.safeGetConfigDescription(key)
	desc.TemplateName = tpl
	desc.FileFormatConfig = formatter.DeepCopy()
	return f
}

func (f *MockParamConfigRendererFactory) SetTemplateName(tpl string) *MockParamConfigRendererFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.TemplateName = tpl
	return f
}

func (f *MockParamConfigRendererFactory) HScaleEnabled() *MockParamConfigRendererFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.ReRenderResourceTypes = append(desc.ReRenderResourceTypes, parametersv1alpha1.ComponentHScaleType)
	return f
}

func (f *MockParamConfigRendererFactory) TLSEnabled() *MockParamConfigRendererFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.ReRenderResourceTypes = append(desc.ReRenderResourceTypes, parametersv1alpha1.ComponentTLSType)
	return f
}

func (f *MockParamConfigRendererFactory) VScaleEnabled() *MockParamConfigRendererFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.ReRenderResourceTypes = append(desc.ReRenderResourceTypes, parametersv1alpha1.ComponentVScaleType)
	return f
}
