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

package common

import (
	"testing"

	"github.com/sethvargo/go-password/password"
)

const (
	N = 10000
)

func testGeneratorGeneratePasswordWithSeed(t *testing.T) {
	seed := "mock-seed-for-generate-the same-password"
	resultSeedFirstTime := ""
	resultSeedEachTime := ""
	for i := 0; i < N; i++ {
		res, err := GeneratePassword(10, 5, 0, false, seed)
		if err != nil {
			t.Error(err)
		}
		resultSeedEachTime = res
		if len(resultSeedFirstTime) == 0 {
			resultSeedFirstTime = res
		}
		if resultSeedFirstTime != resultSeedEachTime {
			t.Errorf("%q should be equal to %q", resultSeedFirstTime, resultSeedEachTime)
		}
	}
}

func testGeneratorGeneratePassword(t *testing.T) {
	t.Run("exceeds_length", func(t *testing.T) {
		t.Parallel()

		if _, err := GeneratePassword(0, 1, 0, false, ""); err != password.ErrExceedsTotalLength {
			t.Errorf("expected %q to be %q", err, password.ErrExceedsTotalLength)
		}

		if _, err := GeneratePassword(0, 0, 1, false, ""); err != password.ErrExceedsTotalLength {
			t.Errorf("expected %q to be %q", err, password.ErrExceedsTotalLength)
		}
	})

	t.Run("should be different when seed is empty", func(t *testing.T) {
		t.Parallel()
		resultSeedFirstTime := ""
		resultSeedEachTime := ""
		hasDiffPassword := false
		for i := 0; i < N; i++ {
			res, err := GeneratePassword(i%len(password.LowerLetters), 0, 0, true, "")
			if err != nil {
				t.Error(err)
			}
			resultSeedEachTime = res
			if len(resultSeedFirstTime) == 0 {
				resultSeedFirstTime = res
			}
			if resultSeedFirstTime != resultSeedEachTime {
				hasDiffPassword = true
				break
			}
		}
		if !hasDiffPassword {
			t.Errorf("%q should be different to %q", resultSeedFirstTime, resultSeedEachTime)
		}
	})
}

func TestGeneratorGeneratePassword(t *testing.T) {
	testGeneratorGeneratePassword(t)
}

func TestGeneratorGeneratePasswordWithSeed(t *testing.T) {
	testGeneratorGeneratePasswordWithSeed(t)
}

// containsUppercase checks if s has at least one uppercase letter (A-Z).
func containsUppercase(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

// containsLowercase checks if s has at least one lowercase letter (a-z).
func containsLowercase(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

// TestGeneratorEnsureMixedCase verifies two requirements:
// 1) When noUpper = false, the generated password contains uppercase and lowercase letters.
// 2) Passwords generated with the same seed are identical.
func TestGeneratorEnsureMixedCase(t *testing.T) {
	t.Run("should_contain_mixed_case_when_noUpper_false", func(t *testing.T) {
		length := 12
		numDigits := 3
		numSymbols := 2
		seed := ""

		// Generate multiple passwords and check they have both upper and lower letters.
		for i := 0; i < 100; i++ {
			pwd, err := GeneratePassword(length, numDigits, numSymbols, false, seed)
			if err != nil {
				t.Fatalf("unexpected error generating password: %v", err)
			}
			if !containsUppercase(pwd) || !containsLowercase(pwd) {
				t.Errorf("password %q does not contain both uppercase and lowercase letters", pwd)
			}
		}
	})

	t.Run("should_produce_same_result_with_same_seed", func(t *testing.T) {
		length := 10
		numDigits := 2
		numSymbols := 1
		seed := "fixed-seed-123"

		var firstPwd string
		for i := 0; i < 50; i++ {
			pwd, err := GeneratePassword(length, numDigits, numSymbols, false, seed)
			if err != nil {
				t.Fatalf("unexpected error generating password with seed: %v", err)
			}
			if i == 0 {
				firstPwd = pwd
			} else {
				if pwd != firstPwd {
					t.Errorf("expected the same password for the same seed, but got %q vs %q", firstPwd, pwd)
				}
			}
		}
	})
}
