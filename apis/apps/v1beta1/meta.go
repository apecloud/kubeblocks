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

package v1beta1

func (in *ConfigConstraintSpec) NeedDynamicReloadAction() bool {
	if in.DynamicActionCanBeMerged != nil {
		return !*in.DynamicActionCanBeMerged
	}
	return false
}

func (in *ConfigConstraintSpec) DynamicParametersPolicy() DynamicParameterSelectedPolicy {
	if in.DynamicParameterSelectedPolicy != nil {
		return *in.DynamicParameterSelectedPolicy
	}
	return SelectedDynamicParameters
}

func (in *ConfigConstraintSpec) ShellTrigger() bool {
	return in.DynamicReloadAction != nil && in.DynamicReloadAction.ShellTrigger != nil
}

func (in *ConfigConstraintSpec) BatchReload() bool {
	return in.ShellTrigger() &&
		in.DynamicReloadAction.ShellTrigger.BatchReload != nil &&
		*in.DynamicReloadAction.ShellTrigger.BatchReload
}

func (cs *ConfigConstraintStatus) ConfigConstraintTerminalPhases() bool {
	return cs.Phase == CCAvailablePhase
}

func (tc *ToolConfig) AsSidecarContainerImage() bool {
	return tc != nil &&
		tc.AsContainerImage != nil &&
		*tc.AsContainerImage
}
