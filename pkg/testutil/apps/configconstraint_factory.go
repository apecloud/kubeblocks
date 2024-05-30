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
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
)

type MockConfigConstraintFactory struct {
	BaseFactory[appsv1beta1.ConfigConstraint, *appsv1beta1.ConfigConstraint, MockConfigConstraintFactory]
}

func NewConfigConstraintFactory(name string) *MockConfigConstraintFactory {
	f := &MockConfigConstraintFactory{}
	f.Init("", name, &appsv1beta1.ConfigConstraint{
		Spec: appsv1beta1.ConfigConstraintSpec{
			ReloadAction: &appsv1beta1.ReloadAction{
				ShellTrigger: &appsv1beta1.ShellTrigger{
					Sync:    cfgutil.ToPointer(false),
					Command: []string{"/bin/true"},
				},
			},
			FileFormatConfig: &appsv1beta1.FileFormatConfig{
				Format: appsv1beta1.Ini,
			},
		},
	}, f)
	return f
}

func (f *MockConfigConstraintFactory) FileFormatConfig(config *appsv1beta1.FileFormatConfig) *MockConfigConstraintFactory {
	f.Get().Spec.FileFormatConfig = config
	return f
}

func (f *MockConfigConstraintFactory) ShellCommand(command []string) *MockConfigConstraintFactory {
	if f.Get().Spec.ReloadAction == nil {
		f.Get().Spec.ReloadAction = &appsv1beta1.ReloadAction{}
	}
	f.Get().Spec.ReloadAction.ShellTrigger = &appsv1beta1.ShellTrigger{
		Command: command,
	}
	return f
}

func (f *MockConfigConstraintFactory) Schema(schema string) *MockConfigConstraintFactory {
	if f.Get().Spec.ParametersSchema == nil {
		f.Get().Spec.ParametersSchema = &appsv1beta1.ParametersSchema{}
	}
	f.Get().Spec.ParametersSchema.CUE = schema
	return f
}
