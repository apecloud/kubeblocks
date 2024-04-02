/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
