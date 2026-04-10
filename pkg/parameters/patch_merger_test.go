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
	"strings"
	"testing"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

func TestDoMergeAppliesUnmanagedUpdatesAfterContentAndParameters(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[mysqld]\ngtid_mode=OFF\nmax_connections=1500\n"),
			Parameters: map[string]*string{
				"max_connections": strPtr("2000"),
			},
			UnmanagedUpdates: []parametersv1alpha1.UnmanagedParameterSectionUpdate{{
				Section: strPtr("mysqld"),
				Updates: []parametersv1alpha1.ParameterUpdate{
					{Type: parametersv1alpha1.ParameterUpdateSet, Key: "custom_local", Value: strPtr("on")},
					{Type: parametersv1alpha1.ParameterUpdateSet, Key: "max_connections", Value: strPtr("2500")},
				},
			}},
		},
	}
	configDescs := []parametersv1alpha1.ComponentConfigDescription{{
		Name:         "my.cnf",
		TemplateName: "mysql-config",
		FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
			},
		},
	}}

	got, err := DoMerge(base, patch, nil, configDescs)
	if err != nil {
		t.Fatalf("DoMerge() error = %v", err)
	}
	content := got["my.cnf"]
	if !strings.Contains(content, "max_connections=2500") {
		t.Fatalf("expected unmanaged updates to be applied after managed parameters, got %q", content)
	}
	if !strings.Contains(content, "custom_local=on") {
		t.Fatalf("expected unmanaged update to append custom_local, got %q", content)
	}
	if !strings.Contains(content, "gtid_mode=OFF") {
		t.Fatalf("expected unrelated config to remain after merge, got %q", content)
	}
}

func strPtr(v string) *string {
	return &v
}
