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

type MockParametersDrivenConfigFactory struct {
	testapps.BaseFactory[parametersv1alpha1.ParameterDrivenConfigRender, *parametersv1alpha1.ParameterDrivenConfigRender, MockParametersDrivenConfigFactory]
}

func NewParametersDrivenConfigFactory(name string) *MockParametersDrivenConfigFactory {
	f := &MockParametersDrivenConfigFactory{}
	f.Init("", name, &parametersv1alpha1.ParameterDrivenConfigRender{
		Spec: parametersv1alpha1.ParameterDrivenConfigRenderSpec{
			Configs: []parametersv1alpha1.ComponentConfigDescription{
				{
					Name: MysqlConfigFile,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.Ini,
					},
				},
			},
		},
	}, f)
	return f
}

func (f *MockParametersDrivenConfigFactory) SetParametersDefs(paramsDefs ...string) *MockParametersDrivenConfigFactory {
	f.Get().Spec.ParametersDefs = paramsDefs
	return f
}

func (f *MockParametersDrivenConfigFactory) SetComponentDefinition(cmpd string) *MockParametersDrivenConfigFactory {
	f.Get().Spec.ComponentDef = cmpd
	return f
}

func (f *MockParametersDrivenConfigFactory) safeGetConfigDescription(key string) *parametersv1alpha1.ComponentConfigDescription {
	desc := intctrlutil.GetComponentConfigDescription(&f.Get().Spec, key)
	if desc != nil {
		return desc
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, parametersv1alpha1.ComponentConfigDescription{
		Name: key,
	})
	return intctrlutil.GetComponentConfigDescription(&f.Get().Spec, key)
}

func (f *MockParametersDrivenConfigFactory) SetConfigDescription(key, tpl string, formatter parametersv1alpha1.FileFormatConfig) *MockParametersDrivenConfigFactory {
	desc := f.safeGetConfigDescription(key)
	desc.TemplateName = tpl
	desc.FileFormatConfig = formatter.DeepCopy()
	return f
}

func (f *MockParametersDrivenConfigFactory) SetTemplateName(tpl string) *MockParametersDrivenConfigFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.TemplateName = tpl
	return f
}

func (f *MockParametersDrivenConfigFactory) HScaleEnabled() *MockParametersDrivenConfigFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.ReRenderResourceTypes = append(desc.ReRenderResourceTypes, parametersv1alpha1.ComponentHScaleType)
	return f
}

func (f *MockParametersDrivenConfigFactory) TLSEnabled() *MockParametersDrivenConfigFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.ReRenderResourceTypes = append(desc.ReRenderResourceTypes, parametersv1alpha1.ComponentTLSType)
	return f
}

func (f *MockParametersDrivenConfigFactory) VScaleEnabled() *MockParametersDrivenConfigFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.ReRenderResourceTypes = append(desc.ReRenderResourceTypes, parametersv1alpha1.ComponentVScaleType)
	return f
}

func (f *MockParametersDrivenConfigFactory) SetInjectEnv(containers ...string) *MockParametersDrivenConfigFactory {
	desc := f.safeGetConfigDescription(MysqlConfigFile)
	desc.InjectEnvTo = containers
	return f
}
