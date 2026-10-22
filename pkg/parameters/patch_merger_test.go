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

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
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

	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	got, err := DoMerge(base, patch, paramsDefs, configDescs)
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

func TestDoMergeRejectsUnmanagedUpdateSettingImmutableParameter(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			UnmanagedUpdates: []parametersv1alpha1.UnmanagedParameterSectionUpdate{{
				Section: strPtr("mysqld"),
				Updates: []parametersv1alpha1.ParameterUpdate{
					{Type: parametersv1alpha1.ParameterUpdateSet, Key: "gtid_mode", Value: strPtr("ON")},
				},
			}},
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err == nil {
		t.Fatalf("expected immutable parameter modification via unmanaged update to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "gtid_mode") {
		t.Fatalf("expected error to mention immutable parameter gtid_mode, got %q", err.Error())
	}
}

func TestDoMergeRejectsUnmanagedUpdateRemovingImmutableParameter(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			UnmanagedUpdates: []parametersv1alpha1.UnmanagedParameterSectionUpdate{{
				Section: strPtr("mysqld"),
				Updates: []parametersv1alpha1.ParameterUpdate{
					{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "gtid_mode"},
				},
			}},
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err == nil {
		t.Fatalf("expected immutable parameter removal via unmanaged update to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "gtid_mode") {
		t.Fatalf("expected error to mention immutable parameter gtid_mode, got %q", err.Error())
	}
}

func TestDoMergeAcceptsUnmanagedUpdateWithImmutableKeyInDifferentSection(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n\n[client]\ngtid_mode=OFF\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			UnmanagedUpdates: []parametersv1alpha1.UnmanagedParameterSectionUpdate{{
				Section: strPtr("client"),
				Updates: []parametersv1alpha1.ParameterUpdate{
					{Type: parametersv1alpha1.ParameterUpdateSet, Key: "gtid_mode", Value: strPtr("ON")},
				},
			}},
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	got, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err != nil {
		t.Fatalf("expected unmanaged update in a different INI section to be accepted, got error %v", err)
	}
	content := got["my.cnf"]
	if !strings.Contains(content, "[mysqld]\ngtid_mode=OFF") {
		t.Fatalf("expected managed mysqld gtid_mode to stay unchanged, got %q", content)
	}
	if !strings.Contains(content, "[client]\ngtid_mode=ON") {
		t.Fatalf("expected client section gtid_mode to be updated, got %q", content)
	}
}

func strPtr(v string) *string {
	return &v
}

// The following tests cover the content-path immutable guard added in
// validateImmutableContentChanges. Each case constructs a base ConfigMap +
// ParametersInFile content patch and asserts whether DoMerge accepts or
// rejects the result.
//
// Coverage matrix:
//   - content modifies an immutable parameter   -> reject
//   - content adds an immutable parameter       -> reject
//   - content removes an immutable parameter    -> reject
//   - content only reorders / re-formats        -> accept (no false positive)
//   - file has no immutable candidate, missing
//     FileFormatConfig                          -> accept (no backward-compat regression)
//   - file has immutable candidate but missing
//     FileFormatConfig                          -> reject (fail-safe)

func iniConfigDescsForMyCnf() []parametersv1alpha1.ComponentConfigDescription {
	return []parametersv1alpha1.ComponentConfigDescription{{
		Name:         "my.cnf",
		TemplateName: "mysql-config",
		FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
			},
		},
	}}
}

func paramsDefsWithImmutable(immutable []string) []*parametersv1alpha1.ParametersDefinition {
	return []*parametersv1alpha1.ParametersDefinition{{
		Spec: parametersv1alpha1.ParametersDefinitionSpec{
			FileName:            "my.cnf",
			ImmutableParameters: immutable,
		},
	}}
}

func TestDoMergeRejectsContentModifyingImmutableParameter(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[mysqld]\ngtid_mode=ON\nmax_connections=1000\n"),
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err == nil {
		t.Fatalf("expected immutable parameter modification via content to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "gtid_mode") {
		t.Fatalf("expected error to mention immutable parameter gtid_mode, got %q", err.Error())
	}
}

func TestDoMergeRejectsContentAddingImmutableParameter(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[mysqld]\nmax_connections=1000\ngtid_mode=ON\n"),
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err == nil {
		t.Fatalf("expected immutable parameter addition via content to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "gtid_mode") {
		t.Fatalf("expected error to mention immutable parameter gtid_mode, got %q", err.Error())
	}
}

func TestDoMergeRejectsContentRemovingImmutableParameter(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[mysqld]\nmax_connections=1000\n"),
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err == nil {
		t.Fatalf("expected immutable parameter removal via content to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "gtid_mode") {
		t.Fatalf("expected error to mention immutable parameter gtid_mode, got %q", err.Error())
	}
}

func TestDoMergeAcceptsContentWithOnlyFormatChange(t *testing.T) {
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	// Same semantic content, just reordered and with an added blank line.
	// Must not be flagged as an immutable change because the parsed parameter
	// map for gtid_mode is unchanged.
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[mysqld]\n\nmax_connections=1000\ngtid_mode=OFF\n"),
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	if _, err := DoMerge(base, patch, paramsDefs, configDescs); err != nil {
		t.Fatalf("expected format-only content change to be accepted, got error %v", err)
	}
}

func TestDoMergeAcceptsContentWithMutableChangeAndFormatNoiseWhileImmutableUnchanged(t *testing.T) {
	// Realistic MySQL-style fixture covering the joint case the guard must
	// accept:
	//   1) format noise -- comment rewrites, extra blank lines, `key=value`
	//      vs `key = value` spacing -- must NOT trip the immutable check;
	//   2) a mutable parameter (max_connections) changes value -- that is
	//      explicitly allowed because it is not in ImmutableParameters;
	//   3) the immutable parameter (gtid_mode) keeps the same parsed value
	//      across base and patch, so the guard must let the merge through.
	//
	// Together these confirm the guard only triggers on immutable-parameter
	// semantic delta and ignores everything else.
	base := map[string]string{
		"my.cnf": "" +
			"# header comment\n" +
			"[mysqld]\n" +
			"gtid_mode=OFF\n" +
			"max_connections=1000\n" +
			"# inline comment\n" +
			"slow_query_log=ON\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("" +
				"# rewritten header\n" +
				"\n" +
				"[mysqld]\n" +
				"\n" +
				"# reordered block\n" +
				"slow_query_log = ON\n" +
				"max_connections = 2000\n" +
				"gtid_mode = OFF\n"),
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	if _, err := DoMerge(base, patch, paramsDefs, configDescs); err != nil {
		t.Fatalf("expected realistic format/whitespace/comment noise without immutable-parameter change to be accepted, got error %v", err)
	}
}

func TestDoMergeAcceptsContentOnFileWithoutImmutableCandidate(t *testing.T) {
	// File has no ParametersDefinition entry, and the content payload uses an
	// arbitrary format with no FileFormatConfig registered. The guard must
	// stay out of the way: there is no immutable contract to protect on this
	// file, so a missing parser must not block a benign content update.
	base := map[string]string{
		"custom.conf": "key1=value1\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"custom.conf": {
			Content: strPtr("key1=value1\nkey2=value2\n"),
		},
	}

	if _, err := DoMerge(base, patch, nil, nil); err != nil {
		t.Fatalf("expected content update on file without immutable candidate to be accepted even without parser, got error %v", err)
	}
}

func TestDoMergeRejectsContentRemovingEntireSectionContainingImmutableParameter(t *testing.T) {
	// Regression for the nil-deref panic introduced by core.FromConfigObject
	// returning nil when an INI sub-section is absent. The content payload
	// drops the entire [mysqld] section that contains the immutable
	// parameter gtid_mode; the guard must reject the change (immutable
	// removed) instead of panicking on GetAllParameters of a nil
	// ConfigObject.
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[other]\nfoo=bar\n"),
		},
	}
	configDescs := iniConfigDescsForMyCnf()
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, configDescs)
	if err == nil {
		t.Fatalf("expected immutable parameter removal via whole-section deletion to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "gtid_mode") {
		t.Fatalf("expected error to mention immutable parameter gtid_mode, got %q", err.Error())
	}
}

func TestDoMergeRejectsContentWhenImmutableDeclaredButFormatMissing(t *testing.T) {
	// File has an immutable parameter declared but no FileFormatConfig is
	// available. Fail-safe: reject rather than silently allow because the
	// guard cannot verify whether the content change touched gtid_mode.
	base := map[string]string{
		"my.cnf": "[mysqld]\ngtid_mode=OFF\n",
	}
	patch := map[string]parametersv1alpha1.ParametersInFile{
		"my.cnf": {
			Content: strPtr("[mysqld]\ngtid_mode=OFF\nmax_connections=1000\n"),
		},
	}
	paramsDefs := paramsDefsWithImmutable([]string{"gtid_mode"})

	_, err := DoMerge(base, patch, paramsDefs, nil)
	if err == nil {
		t.Fatalf("expected fail-safe rejection when immutable parameter declared but file format config is missing, got nil error")
	}
	if !strings.Contains(err.Error(), "missing file format config") {
		t.Fatalf("expected error to mention missing file format config, got %q", err.Error())
	}
}

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
