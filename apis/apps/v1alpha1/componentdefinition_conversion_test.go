/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestComponentDefinitionConvertFromPreservesTemplates(t *testing.T) {
	src := &appsv1.ComponentDefinition{
		Spec: appsv1.ComponentDefinitionSpec{
			Configs: []appsv1.ComponentFileTemplate{
				{
					Name:     "postgresql-configuration",
					Template: "postgresql12-configuration-1.2.0-alpha.0",
				},
			},
			Scripts: []appsv1.ComponentFileTemplate{
				{
					Name:     "postgresql-scripts",
					Template: "postgresql-scripts-1.2.0-alpha.0",
				},
			},
		},
	}

	var dst ComponentDefinition
	if err := dst.ConvertFrom(src); err != nil {
		t.Fatalf("ConvertFrom() error = %v", err)
	}

	if got, want := dst.Spec.Configs[0].TemplateRef, src.Spec.Configs[0].Template; got != want {
		t.Fatalf("configs[0].TemplateRef = %q, want %q", got, want)
	}
	if got, want := dst.Spec.Scripts[0].TemplateRef, src.Spec.Scripts[0].Template; got != want {
		t.Fatalf("scripts[0].TemplateRef = %q, want %q", got, want)
	}
}
