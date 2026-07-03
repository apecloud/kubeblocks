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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestComponentDefinitionConvertFromMapsNativeV1Templates(t *testing.T) {
	// a ComponentDefinition created natively as v1 carries no
	// increment-converter annotation, so the renamed template fields must be
	// mapped from the v1 source directly
	src := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "postgresql",
		},
		Spec: appsv1.ComponentDefinitionSpec{
			Configs: []appsv1.ComponentFileTemplate{{
				Name:     "postgresql-configuration",
				Template: "postgresql-configuration-tpl",
			}},
			Scripts: []appsv1.ComponentFileTemplate{{
				Name:     "postgresql-scripts",
				Template: "postgresql-scripts-tpl",
			}},
		},
	}

	dst := &ComponentDefinition{}
	if err := dst.ConvertFrom(src); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if len(dst.Spec.Configs) != 1 || dst.Spec.Configs[0].TemplateRef != "postgresql-configuration-tpl" {
		t.Fatalf("config templateRef not mapped from v1 template: %+v", dst.Spec.Configs)
	}
	if len(dst.Spec.Scripts) != 1 || dst.Spec.Scripts[0].TemplateRef != "postgresql-scripts-tpl" {
		t.Fatalf("script templateRef not mapped from v1 template: %+v", dst.Spec.Scripts)
	}
}

func TestComponentDefinitionConvertRoundTripKeepsTemplates(t *testing.T) {
	// an object created as v1alpha1 must survive the v1 round trip through the
	// increment-converter annotation with its templateRef intact
	orig := &ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "postgresql",
		},
		Spec: ComponentDefinitionSpec{
			Configs: []ComponentConfigSpec{{
				ComponentTemplateSpec: ComponentTemplateSpec{
					Name:        "postgresql-configuration",
					TemplateRef: "postgresql-configuration-tpl",
				},
			}},
			Scripts: []ComponentTemplateSpec{{
				Name:        "postgresql-scripts",
				TemplateRef: "postgresql-scripts-tpl",
			}},
		},
	}

	hub := &appsv1.ComponentDefinition{}
	if err := orig.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if hub.Spec.Configs[0].Template != "postgresql-configuration-tpl" {
		t.Fatalf("v1 config template not set: %+v", hub.Spec.Configs)
	}

	back := &ComponentDefinition{}
	if err := back.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if back.Spec.Configs[0].TemplateRef != "postgresql-configuration-tpl" {
		t.Fatalf("round-trip config templateRef lost: %+v", back.Spec.Configs)
	}
	if back.Spec.Scripts[0].TemplateRef != "postgresql-scripts-tpl" {
		t.Fatalf("round-trip script templateRef lost: %+v", back.Spec.Scripts)
	}
}
