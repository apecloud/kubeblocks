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

package parameters

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
)

func TestInitialImmutableParameters(t *testing.T) {
	base := map[string]string{"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n"}
	defs := paramsDefsWithImmutable([]string{"gtid_mode"})
	descs := iniConfigDescsForMyCnf()
	for _, tt := range []struct {
		name    string
		patch   map[string]*string
		initial map[string]*string
		want    string
		wantErr bool
	}{
		{name: "matching initial value", patch: map[string]*string{"gtid_mode": strPtr("ON"), "max_connections": strPtr("1500")}, initial: map[string]*string{"gtid_mode": strPtr("ON")}, want: "gtid_mode=ON"},
		{name: "different value", patch: map[string]*string{"gtid_mode": strPtr("OFF")}, initial: map[string]*string{"gtid_mode": strPtr("ON")}, wantErr: true},
		{name: "missing key", patch: map[string]*string{"gtid_mode": nil}, initial: map[string]*string{"max_connections": nil}, wantErr: true},
		{name: "missing initial", patch: map[string]*string{"gtid_mode": strPtr("ON")}, wantErr: true},
		{name: "removal does not permit a value", patch: map[string]*string{"gtid_mode": strPtr("ON")}, initial: map[string]*string{"gtid_mode": nil}, wantErr: true},
		{name: "value does not permit removal", patch: map[string]*string{"gtid_mode": nil}, initial: map[string]*string{"gtid_mode": strPtr("ON")}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			patch := map[string]parametersv1alpha1.ParametersInFile{"my.cnf": {Parameters: tt.patch}}
			initial := map[string]parametersv1alpha1.ParametersInFile{"my.cnf": {Parameters: tt.initial}}
			got, err := DoMergeWithInitialParameters(base, patch, initial, defs, descs)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "immutable parameter gtid_mode") {
					t.Fatalf("expected immutable rejection, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("applying initial immutable parameter: %v", err)
			}
			if !strings.Contains(got["my.cnf"], tt.want) {
				t.Fatalf("expected %q in merged config: %q", tt.want, got["my.cnf"])
			}
		})
	}
}

func TestInitialImmutableParameterRemoval(t *testing.T) {
	base := map[string]string{"config.yaml": "gtid_mode: OFF\nmax_connections: 1000\n"}
	defs := paramsDefsWithImmutable([]string{"gtid_mode"})
	defs[0].Spec.FileName = "config.yaml"
	descs := []parametersv1alpha1.ComponentConfigDescription{{Name: "config.yaml", FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.YAML}}}
	for _, removal := range []*string{nil, strPtr(parameterRemoveMarker)} {
		patch := map[string]parametersv1alpha1.ParametersInFile{"config.yaml": {Parameters: map[string]*string{"gtid_mode": removal}}}
		initial := map[string]parametersv1alpha1.ParametersInFile{"config.yaml": {Parameters: map[string]*string{"gtid_mode": nil}}}
		got, err := DoMergeWithInitialParameters(base, patch, initial, defs, descs)
		if err != nil {
			t.Fatalf("matching initial removal should merge: %v", err)
		}
		if strings.Contains(got["config.yaml"], "gtid_mode") || !strings.Contains(got["config.yaml"], "max_connections") {
			t.Fatalf("expected only initial parameter removal: %q", got["config.yaml"])
		}
	}
}

func TestInitialImmutableParametersKeepStrictEntryPoints(t *testing.T) {
	base := map[string]string{"my.cnf": "[mysqld]\ngtid_mode=OFF\n"}
	defs := paramsDefsWithImmutable([]string{"gtid_mode"})
	descs := iniConfigDescsForMyCnf()
	patch := map[string]parametersv1alpha1.ParametersInFile{"my.cnf": {Parameters: map[string]*string{"gtid_mode": strPtr("ON")}}}
	if _, err := DoMerge(base, patch, defs, descs); err == nil {
		t.Fatal("DoMerge must reject immutable parameters")
	}
	if _, err := MergeAndValidateConfigs(base, []core.ParamPairs{{Key: "my.cnf", UpdatedParams: map[string]any{"gtid_mode": "ON"}}}, defs, descs); err == nil {
		t.Fatal("MergeAndValidateConfigs must reject immutable parameters")
	}
}

func initialTypedParameterDefinitions() ([]*parametersv1alpha1.ParametersDefinition, []parametersv1alpha1.ComponentConfigDescription) {
	minimum := float64(1)
	defs := []*parametersv1alpha1.ParametersDefinition{{Spec: parametersv1alpha1.ParametersDefinitionSpec{
		FileName: "config.json", ImmutableParameters: []string{"replicas"},
		ParametersSchema: &parametersv1alpha1.ParametersSchema{SchemaInJSON: &apiextv1.JSONSchemaProps{
			Properties: map[string]apiextv1.JSONSchemaProps{openapi.DefaultSchemaName: {
				Type: "object", Properties: map[string]apiextv1.JSONSchemaProps{"replicas": {Type: "integer", Minimum: &minimum}},
			}},
		}},
	}}}
	descs := []parametersv1alpha1.ComponentConfigDescription{{Name: "config.json", FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.JSON}}}
	return defs, descs
}

func TestInitialImmutableParametersConvertEquivalentValues(t *testing.T) {
	defs, descs := initialTypedParameterDefinitions()
	base := map[string]string{"config.json": `{"replicas":1}`}
	initial := map[string]parametersv1alpha1.ParametersInFile{"config.json": {Parameters: map[string]*string{"replicas": strPtr("02")}}}
	patch := map[string]parametersv1alpha1.ParametersInFile{"config.json": {Parameters: map[string]*string{"replicas": strPtr("2")}}}
	got, err := DoMergeWithInitialParameters(base, patch, initial, defs, descs)
	if err != nil {
		t.Fatalf("equivalent converted initial values should merge: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got["config.json"]), &decoded); err != nil {
		t.Fatalf("decoding merged JSON: %v", err)
	}
	if decoded["replicas"] != float64(2) {
		t.Fatalf("expected typed initial value: %q", got["config.json"])
	}
}

func TestInitialImmutableParametersStillValidateSchema(t *testing.T) {
	defs, descs := initialTypedParameterDefinitions()
	defs[0].Spec.ParametersSchema.CUE = "replicas: int & >=1"
	patch := map[string]parametersv1alpha1.ParametersInFile{"config.json": {Parameters: map[string]*string{"replicas": strPtr("0")}}}
	_, err := DoMergeWithInitialParameters(map[string]string{"config.json": `{"replicas":1}`}, patch, patch, defs, descs)
	if err == nil || !strings.Contains(err.Error(), "validate") {
		t.Fatalf("initial permit must not bypass schema validation: %v", err)
	}
}

func TestInitialImmutableParametersDoNotPermitOtherUpdateForms(t *testing.T) {
	base := map[string]string{"my.cnf": "[mysqld]\ngtid_mode=OFF\n"}
	defs := paramsDefsWithImmutable([]string{"gtid_mode"})
	descs := iniConfigDescsForMyCnf()
	content := parametersv1alpha1.ParametersInFile{Content: strPtr("[mysqld]\ngtid_mode=ON\n")}
	unmanaged := parametersv1alpha1.ParametersInFile{UnmanagedUpdates: []parametersv1alpha1.UnmanagedParameterSectionUpdate{{
		Updates: []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateSet, Key: "gtid_mode", Value: strPtr("ON")}},
	}}}
	managed := parametersv1alpha1.ParametersInFile{Parameters: map[string]*string{"gtid_mode": strPtr("ON")}}
	for _, tt := range []struct {
		name    string
		patch   parametersv1alpha1.ParametersInFile
		initial parametersv1alpha1.ParametersInFile
	}{
		{name: "raw content with matching initial managed parameter", patch: content, initial: managed},
		{name: "unmanaged with matching initial managed parameter", patch: unmanaged, initial: managed},
		{name: "initial raw content grants no managed permit", patch: managed, initial: content},
		{name: "initial unmanaged update grants no managed permit", patch: managed, initial: unmanaged},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DoMergeWithInitialParameters(base, map[string]parametersv1alpha1.ParametersInFile{"my.cnf": tt.patch}, map[string]parametersv1alpha1.ParametersInFile{"my.cnf": tt.initial}, defs, descs)
			if err == nil || !strings.Contains(err.Error(), "immutable parameter gtid_mode") {
				t.Fatalf("expected immutable rejection, got %v", err)
			}
		})
	}
}

func TestApplyParametersInitialImmutableValue(t *testing.T) {
	defs := paramsDefsWithImmutable([]string{"gtid_mode"})
	initial := map[string]parametersv1alpha1.ParametersInFile{"my.cnf": {Parameters: map[string]*string{"gtid_mode": strPtr("ON")}}}
	item := parametersv1alpha1.ConfigTemplateItemDetail{ConfigFileParams: initial}
	base := &corev1.ConfigMap{Data: map[string]string{"my.cnf": "[mysqld]\ngtid_mode=OFF\n"}}
	got, err := ApplyParameters(item, base, iniConfigDescsForMyCnf(), defs, initial)
	if err != nil {
		t.Fatalf("applying initial immutable value: %v", err)
	}
	if !strings.Contains(got.Data["my.cnf"], "gtid_mode=ON") || strings.Contains(base.Data["my.cnf"], "gtid_mode=ON") {
		t.Fatalf("expected updated copy and unchanged base, got base=%v updated=%v", base.Data, got.Data)
	}
}
