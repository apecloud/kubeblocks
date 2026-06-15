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

package common

import (
	"math"
	"testing"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestToCamelCase(t *testing.T) {
	cases := map[string]string{
		"make-food":     "MakeFood",
		"make.food":     "MakeFood",
		"alreadyCamel":  "Alreadycamel",
		"multi-part.id": "MultiPartId",
	}
	for input, want := range cases {
		if got := ToCamelCase(input); got != want {
			t.Fatalf("ToCamelCase(%q) expected %q, got %q", input, want, got)
		}
	}
}

func TestIsCompactMode(t *testing.T) {
	if IsCompactMode(nil) {
		t.Fatalf("nil annotations should not be compact mode")
	}
	if IsCompactMode(map[string]string{"other": "true"}) {
		t.Fatalf("unrelated annotations should not be compact mode")
	}
	if !IsCompactMode(map[string]string{constant.FeatureReconciliationInCompactModeAnnotationKey: "true"}) {
		t.Fatalf("compact mode annotation should be detected")
	}
}

func TestSafeAddInt(t *testing.T) {
	if got := SafeAddInt(1, 2); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	if got := SafeAddInt(-3, 2); got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}

	assertPanic(t, func() { SafeAddInt(math.MaxInt, 1) })
	assertPanic(t, func() { SafeAddInt(math.MinInt, -1) })
}

func TestCutString(t *testing.T) {
	if got := CutString("abcdef", 3); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
	if got := CutString("abc", 10); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
}

func assertPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	f()
}
