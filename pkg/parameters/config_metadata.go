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
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func GetComponentConfigDescription(configs []parametersv1alpha1.ComponentConfigDescription, name string) *parametersv1alpha1.ComponentConfigDescription {
	match := func(desc parametersv1alpha1.ComponentConfigDescription) bool {
		return desc.Name == name
	}

	if index := generics.FindFirstFunc(configs, match); index >= 0 {
		return &configs[index]
	}
	return nil
}

func GetComponentConfigDescriptions(configs []parametersv1alpha1.ComponentConfigDescription, tpl string) []parametersv1alpha1.ComponentConfigDescription {
	match := func(desc parametersv1alpha1.ComponentConfigDescription) bool {
		return desc.TemplateName == tpl
	}
	return generics.FindFunc(configs, match)
}

func HasValidParameterTemplate(configs []parametersv1alpha1.ComponentConfigDescription) bool {
	return len(configs) != 0
}
