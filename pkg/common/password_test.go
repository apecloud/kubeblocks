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
	"strings"
	"testing"
	"unicode"

	"github.com/sethvargo/go-password/password"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	N = 10000
)

func testGeneratorGeneratePasswordWithSeed(t *testing.T) {
	seed := "mock-seed-for-generate-the same-password"
	resultSeedFirstTime := ""
	resultSeedEachTime := ""
	for i := 0; i < N; i++ {
		res, err := generatePassword(10, 5, 0, seed, "")
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

		if _, err := generatePassword(0, 1, 0, "", ""); err != password.ErrExceedsTotalLength {
			t.Errorf("expected %q to be %q", err, password.ErrExceedsTotalLength)
		}

		if _, err := generatePassword(0, 0, 1, "", ""); err != password.ErrExceedsTotalLength {
			t.Errorf("expected %q to be %q", err, password.ErrExceedsTotalLength)
		}
	})

	t.Run("should respect allowed symbols", func(t *testing.T) {
		t.Parallel()

		symbols := "!$_#"
		for i := 0; i < N; i++ {
			res, err := generatePassword(10, 0, 5, "", symbols)
			if err != nil {
				t.Error(err)
			}
			for _, r := range res {
				if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
					continue
				}
				if !strings.ContainsRune(symbols, r) {
					t.Errorf("unexpected symbol %q in password %q", r, res)
				}
			}
		}

	})

	t.Run("should be different when seed is empty", func(t *testing.T) {
		t.Parallel()
		resultSeedFirstTime := ""
		resultSeedEachTime := ""
		hasDiffPassword := false
		for i := 0; i < N; i++ {
			res, err := generatePassword(i%(len(password.LowerLetters)+len(password.UpperLetters)), 0, 0, "", "")
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
			pwd, err := generatePassword(length, numDigits, numSymbols, seed, "")
			if err != nil {
				t.Fatalf("unexpected error generating password: %v", err)
			}
			pwd, err = ensureMixedCase(pwd, seed)
			if err != nil {
				t.Fatalf("unexpected error Ensuring mixed-case password: %v", err)
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
			pwd, err := generatePassword(length, numDigits, numSymbols, seed, "")
			if err != nil {
				t.Fatalf("unexpected error generating password with seed: %v", err)
			}
			pwd, err = ensureMixedCase(pwd, seed)
			if err != nil {
				t.Fatalf("unexpected error Ensuring mixed-case password: %v", err)
			}
			if i == 0 {
				firstPwd = pwd
			} else if pwd != firstPwd {
				t.Errorf("expected the same password for the same seed, but got %q vs %q", firstPwd, pwd)
			}
		}
	})
}

func TestGenerateSystemAccountPassword(t *testing.T) {
	tests := []struct {
		name       string
		account    appsv1.SystemAccount
		wantLength int
	}{
		{
			name:       "no configuration means passwordless",
			account:    appsv1.SystemAccount{},
			wantLength: 0,
		},
		{
			name: "legacy configuration remains supported",
			account: appsv1.SystemAccount{
				PasswordGenerationPolicy: &appsv1.PasswordConfig{
					Length:     12,
					NumDigits:  ptr.To(int32(2)),
					LetterCase: appsv1.MixedCases,
				},
			},
			wantLength: 12,
		},
		{
			name: "new configuration takes precedence over legacy",
			account: appsv1.SystemAccount{
				PasswordConfig: &appsv1.PasswordConfig{
					Length:     20,
					NumDigits:  ptr.To(int32(0)),
					LetterCase: appsv1.LowerCases,
				},
				PasswordGenerationPolicy: &appsv1.PasswordConfig{
					Length:     8,
					NumDigits:  ptr.To(int32(8)),
					LetterCase: appsv1.UpperCases,
				},
			},
			wantLength: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generated, err := GenerateSystemAccountPassword(tt.account)
			if err != nil {
				t.Fatalf("generate password: %v", err)
			}
			if len(generated) != tt.wantLength {
				t.Fatalf("expected password length %d, got %d", tt.wantLength, len(generated))
			}
		})
	}
}

func TestGenerateSystemAccountPasswordNewConfigurationPreservesExplicitValues(t *testing.T) {
	generated, err := GenerateSystemAccountPassword(appsv1.SystemAccount{
		PasswordConfig: &appsv1.PasswordConfig{
			Length:     20,
			NumDigits:  ptr.To(int32(0)),
			LetterCase: appsv1.LowerCases,
		},
		PasswordGenerationPolicy: &appsv1.PasswordConfig{
			Length:     8,
			NumDigits:  ptr.To(int32(8)),
			LetterCase: appsv1.UpperCases,
		},
	})
	if err != nil {
		t.Fatalf("generate password: %v", err)
	}
	if strings.ContainsFunc(generated, unicode.IsUpper) {
		t.Fatalf("expected new lower-case configuration to take precedence, got %q", generated)
	}
	if strings.ContainsFunc(generated, unicode.IsDigit) {
		t.Fatalf("expected explicit numDigits=0 to be preserved, got %q", generated)
	}
}

func TestValidateSystemAccountPassword(t *testing.T) {
	tests := []struct {
		name     string
		password []byte
		wantErr  string
	}{
		{name: "empty password is valid", password: nil},
		{name: "maximum length is valid", password: []byte(strings.Repeat("a", 64))},
		{name: "over maximum length is invalid", password: []byte(strings.Repeat("a", 65)), wantErr: "password length exceeds 64 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSystemAccountPassword(tt.password)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate password: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}
