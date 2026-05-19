/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package parameters

import (
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

func TestResolveParameterTemplateMatchesConfigTemplateReference(t *testing.T) {
	templates := ResolveParameterTemplate(appsv1.ComponentDefinitionSpec{
		Configs: []appsv1.ComponentFileTemplate{
			componentFileTemplate("mssql-config", "mssql2022-config-1.2.0-alpha.0", true),
		},
	}, []configv1alpha1.ComponentConfigDescription{
		{TemplateName: "mssql2022-config-1.2.0-alpha.0"},
	})

	if len(templates) != 1 {
		t.Fatalf("expected one matched template, got %d", len(templates))
	}
	if templates[0].Name != "mssql-config" {
		t.Fatalf("expected template name mssql-config, got %q", templates[0].Name)
	}
	if templates[0].Template != "mssql2022-config-1.2.0-alpha.0" {
		t.Fatalf("expected template reference mssql2022-config-1.2.0-alpha.0, got %q", templates[0].Template)
	}
}

func TestResolveParameterTemplateKeepsLegacyNameMatch(t *testing.T) {
	templates := ResolveParameterTemplate(appsv1.ComponentDefinitionSpec{
		Configs: []appsv1.ComponentFileTemplate{
			componentFileTemplate("legacy-config", "legacy-template", true),
		},
	}, []configv1alpha1.ComponentConfigDescription{
		{TemplateName: "legacy-config"},
	})

	if len(templates) != 1 {
		t.Fatalf("expected legacy name match to keep working, got %d", len(templates))
	}
	if templates[0].Name != "legacy-config" {
		t.Fatalf("expected template name legacy-config, got %q", templates[0].Name)
	}
}

func TestResolveParameterTemplateReturnsAllConfigsMatchedBySameTemplateName(t *testing.T) {
	templates := ResolveParameterTemplate(appsv1.ComponentDefinitionSpec{
		Configs: []appsv1.ComponentFileTemplate{
			componentFileTemplate("matched-by-template", "shared-template", true),
			componentFileTemplate("shared-template", "legacy-template", true),
		},
	}, []configv1alpha1.ComponentConfigDescription{
		{TemplateName: "shared-template"},
	})

	if len(templates) != 2 {
		t.Fatalf("expected both configs matched by the same templateName, got %d", len(templates))
	}
	if templates[0].Name != "matched-by-template" {
		t.Fatalf("expected first config to match by template, got %q", templates[0].Name)
	}
	if templates[1].Name != "shared-template" {
		t.Fatalf("expected second config to match by legacy name fallback, got %q", templates[1].Name)
	}
}

func TestResolveParameterTemplateDoesNotDoubleAppendConfigMatchedByTemplateAndName(t *testing.T) {
	templates := ResolveParameterTemplate(appsv1.ComponentDefinitionSpec{
		Configs: []appsv1.ComponentFileTemplate{
			componentFileTemplate("same-template", "same-template", true),
		},
	}, []configv1alpha1.ComponentConfigDescription{
		{TemplateName: "same-template"},
	})

	if len(templates) != 1 {
		t.Fatalf("expected config matched by both template and name to be appended once, got %d", len(templates))
	}
	if templates[0].Name != "same-template" {
		t.Fatalf("expected matched config same-template, got %q", templates[0].Name)
	}
}

func TestResolveParameterTemplateIgnoresNonExternalManagedConfig(t *testing.T) {
	templates := ResolveParameterTemplate(appsv1.ComponentDefinitionSpec{
		Configs: []appsv1.ComponentFileTemplate{
			componentFileTemplate("mssql-config", "mssql2022-config-1.2.0-alpha.0", false),
		},
	}, []configv1alpha1.ComponentConfigDescription{
		{TemplateName: "mssql2022-config-1.2.0-alpha.0"},
	})

	if len(templates) != 0 {
		t.Fatalf("expected non-external-managed config to be ignored, got %d", len(templates))
	}
}

func TestResolveParameterTemplateIgnoresEmptySentinelMatches(t *testing.T) {
	templates := ResolveParameterTemplate(appsv1.ComponentDefinitionSpec{
		Configs: []appsv1.ComponentFileTemplate{
			componentFileTemplate("", "", true),
		},
	}, []configv1alpha1.ComponentConfigDescription{
		{TemplateName: ""},
	})

	if len(templates) != 0 {
		t.Fatalf("expected empty template/name sentinel to be ignored, got %d", len(templates))
	}
}

func componentFileTemplate(name, template string, externalManaged bool) appsv1.ComponentFileTemplate {
	return appsv1.ComponentFileTemplate{
		Name:            name,
		Template:        template,
		ExternalManaged: &externalManaged,
	}
}
