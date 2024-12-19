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
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	desc := intctrlutil.GetComponentConfigDescription(&f.Get().Spec, key)
	if desc != nil {
		return desc
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, parametersv1alpha1.ComponentConfigDescription{
		Name: key,
	})
	return intctrlutil.GetComponentConfigDescription(&f.Get().Spec, key)
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
