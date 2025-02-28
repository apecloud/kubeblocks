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
	"k8s.io/utils/pointer"

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
		param.CustomTemplates = make(map[string]parametersv1alpha1.ConfigTemplateExtension)
	}
	param.CustomTemplates[tpl] = parametersv1alpha1.ConfigTemplateExtension{
		TemplateRef: templateName,
		Namespace:   ns,
		Policy:      parametersv1alpha1.PatchPolicy,
	}
	return f
}
