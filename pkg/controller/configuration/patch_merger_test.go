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

package configuration

import (
	"strings"
	"testing"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

// The following tests cover the content-path immutable guard added in
// intctrlutil.ValidateImmutableContentChanges (release-1.0 counterpart of
// the main-branch PR; see pkg/parameters/patch_merger_test.go on main for
// the same coverage matrix).
//
// Coverage matrix:
//   - content modifies an immutable parameter   -> reject
//   - content adds an immutable parameter       -> reject
//   - content removes an immutable parameter    -> reject
//   - content only reorders / re-formats        -> accept
//   - content has mutable change plus format
//     noise but immutable unchanged             -> accept
//   - file has no immutable candidate, missing
//     FileFormatConfig                          -> accept
//   - file has immutable candidate but missing
//     FileFormatConfig                          -> reject (fail-safe)

func strPtr(v string) *string {
	return &v
}

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
	//   1) format noise — comment rewrites, extra blank lines, `key=value`
	//      vs `key = value` spacing — must NOT trip the immutable check;
	//   2) a mutable parameter (max_connections) changes value — that is
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
