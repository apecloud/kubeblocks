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

package configmanager

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func NeedRestart(paramsDefs map[string]*parametersv1alpha1.ParametersDefinition, patch *core.ConfigPatchInfo) bool {
	if patch == nil {
		return false
	}
	for key := range patch.UpdateConfig {
		if paramsDef, ok := paramsDefs[key]; !ok || !IsSupportReload(paramsDef.Spec.ReloadAction) {
			return true
		}
	}
	return false
}

func IsSupportReload(reload *parametersv1alpha1.ReloadAction) bool {
	return reload != nil && isValidReloadPolicy(*reload)
}

func isValidReloadPolicy(reload parametersv1alpha1.ReloadAction) bool {
	return reload.AutoTrigger != nil || reload.ShellTrigger != nil
}

func IsAutoReload(reload *parametersv1alpha1.ReloadAction) bool {
	return reload != nil && reload.AutoTrigger != nil
}

func ValidateReloadOptions(reloadAction *parametersv1alpha1.ReloadAction, cli client.Client, ctx context.Context) error {
	switch {
	case reloadAction.ShellTrigger != nil:
		return checkShellTrigger(reloadAction.ShellTrigger)
	case reloadAction.AutoTrigger != nil:
		return nil
	}
	return core.MakeError("require special reload type!")
}

func checkShellTrigger(options *parametersv1alpha1.ShellTrigger) error {
	if len(options.Command) == 0 {
		return core.MakeError("required shell trigger")
	}
	return nil
}
