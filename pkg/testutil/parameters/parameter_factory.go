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
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockParameterFactory struct {
	testapps.BaseFactory[parametersv1alpha1.Parameter, *parametersv1alpha1.Parameter, MockParameterFactory]
}

func NewParameterFactory(name, ns, clusterName, compName string) *MockParameterFactory {
	f := &MockParameterFactory{}
	f.Init(ns, name, &parametersv1alpha1.Parameter{
		Spec: parametersv1alpha1.ParameterSpec{
			ClusterName: clusterName,
			ComponentParameters: []parametersv1alpha1.ComponentParametersSpec{
				{
					ComponentName: compName,
				},
			},
		},
	}, f)
	return f
}

func (f *MockParameterFactory) AddParameters(paramName, paramValue string) *MockParameterFactory {
	param := &f.Get().Spec.ComponentParameters[0]
	if param.Parameters == nil {
		param.Parameters = make(map[string]*string)
	}
	param.Parameters[paramName] = pointer.String(paramValue)
	return f
}

func (f *MockParameterFactory) AddCustomTemplate(tpl string, templateName, ns string) *MockParameterFactory {
	param := &f.Get().Spec.ComponentParameters[0]
	if param.CustomTemplates == nil {
		param.CustomTemplates = make(map[string]appsv1.ConfigTemplateExtension)
	}
	param.CustomTemplates[tpl] = appsv1.ConfigTemplateExtension{
		TemplateRef: templateName,
		Namespace:   ns,
		Policy:      appsv1.PatchPolicy,
	}
	return f
}
