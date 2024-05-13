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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func (in *ConfigConstraintSpec) NeedDynamicReloadAction() bool {
	if in.MergeReloadAndRestart != nil {
		return !*in.MergeReloadAndRestart
	}
	return false
}

func (in *ConfigConstraintSpec) ReloadStaticParameters() bool {
	if in.ReloadStaticParamsBeforeRestart != nil {
		return *in.ReloadStaticParamsBeforeRestart
	}
	return false
}

func (in *ConfigConstraintSpec) GetToolsSetup() *ToolsSetup {
	if in.ReloadAction != nil && in.ReloadAction.ShellTrigger != nil {
		return in.ReloadAction.ShellTrigger.ToolsSetup
	}
	return nil
}

func (in *ConfigConstraintSpec) GetScriptConfigs() []ScriptConfig {
	scriptConfigs := make([]ScriptConfig, 0)
	for _, action := range in.DownwardAPIChangeTriggeredActions {
		if action.ScriptConfig != nil {
			scriptConfigs = append(scriptConfigs, *action.ScriptConfig)
		}
	}
	if in.ReloadAction == nil {
		return scriptConfigs
	}
	if in.ReloadAction.ShellTrigger != nil && in.ReloadAction.ShellTrigger.ScriptConfig != nil {
		scriptConfigs = append(scriptConfigs, *in.ReloadAction.ShellTrigger.ScriptConfig)
	}
	return scriptConfigs
}

func (in *ConfigConstraintSpec) ShellTrigger() bool {
	return in.ReloadAction != nil && in.ReloadAction.ShellTrigger != nil
}

func (in *ConfigConstraintSpec) BatchReload() bool {
	return in.ShellTrigger() &&
		in.ReloadAction.ShellTrigger.BatchReload != nil &&
		*in.ReloadAction.ShellTrigger.BatchReload
}

func (in *ConfigConstraintSpec) GetPodSelector() *metav1.LabelSelector {
	if in.ReloadAction != nil {
		return in.ReloadAction.TargetPodSelector
	}
	return nil
}

func (cs *ConfigConstraintStatus) ConfigConstraintTerminalPhases() bool {
	return cs.Phase == CCAvailablePhase
}

func (tc *ToolConfig) AsSidecarContainerImage() bool {
	return tc != nil &&
		tc.AsContainerImage != nil &&
		*tc.AsContainerImage
}
