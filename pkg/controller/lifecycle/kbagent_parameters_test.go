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

package lifecycle

import "testing"

func TestAddShellSafeParameterAliasesPreservesRawParameter(t *testing.T) {
	parameters := map[string]string{
		"maxmemory-policy": "allkeys-lru",
	}

	addShellSafeParameterAliases(parameters)

	if got := parameters["maxmemory-policy"]; got != "allkeys-lru" {
		t.Fatalf("raw parameter changed, got %q", got)
	}
	if got := parameters["MAXMEMORY_POLICY"]; got != "allkeys-lru" {
		t.Fatalf("shell-safe alias mismatch, got %q", got)
	}
}

func TestAddShellSafeParameterAliasesFillsEmptyAlias(t *testing.T) {
	parameters := map[string]string{
		"maxmemory-policy": "allkeys-lru",
		"MAXMEMORY_POLICY": "",
	}

	addShellSafeParameterAliases(parameters)

	if got := parameters["MAXMEMORY_POLICY"]; got != "allkeys-lru" {
		t.Fatalf("empty shell-safe alias should be filled from raw parameter, got %q", got)
	}
}

func TestAddShellSafeParameterAliasesKeepsExplicitAlias(t *testing.T) {
	parameters := map[string]string{
		"maxmemory-policy": "allkeys-lru",
		"MAXMEMORY_POLICY": "volatile-lru",
	}

	addShellSafeParameterAliases(parameters)

	if got := parameters["MAXMEMORY_POLICY"]; got != "volatile-lru" {
		t.Fatalf("non-empty explicit alias should be preserved, got %q", got)
	}
}

func TestShellSafeParameterAlias(t *testing.T) {
	tests := map[string]string{
		"maxmemory-policy": "MAXMEMORY_POLICY",
		"foo.bar-baz":      "FOO_BAR_BAZ",
		"1st-param":        "_1ST_PARAM",
		"ALREADY_SAFE":     "ALREADY_SAFE",
	}
	for input, expected := range tests {
		if got := shellSafeParameterAlias(input); got != expected {
			t.Fatalf("shellSafeParameterAlias(%q)=%q, want %q", input, got, expected)
		}
	}
}
