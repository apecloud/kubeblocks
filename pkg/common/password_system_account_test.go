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

package common

import (
	"strings"
	"testing"
	"unicode"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

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
			name: "legacy non-zero configuration remains supported",
			account: appsv1.SystemAccount{
				PasswordGenerationPolicy: appsv1.PasswordConfig{
					Length:     12,
					NumDigits:  2,
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
					NumDigits:  0,
					LetterCase: appsv1.LowerCases,
				},
				PasswordGenerationPolicy: appsv1.PasswordConfig{
					Length:     8,
					NumDigits:  8,
					LetterCase: appsv1.UpperCases,
				},
			},
			wantLength: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := GenerateSystemAccountPassword(tt.account)
			if err != nil {
				t.Fatalf("generate password: %v", err)
			}
			if len(password) != tt.wantLength {
				t.Fatalf("expected password length %d, got %d", tt.wantLength, len(password))
			}
		})
	}
}

func TestGenerateSystemAccountPasswordNewConfigurationPreservesExplicitValues(t *testing.T) {
	password, err := GenerateSystemAccountPassword(appsv1.SystemAccount{
		PasswordConfig: &appsv1.PasswordConfig{
			Length:     20,
			NumDigits:  0,
			LetterCase: appsv1.LowerCases,
		},
		PasswordGenerationPolicy: appsv1.PasswordConfig{
			Length:     8,
			NumDigits:  8,
			LetterCase: appsv1.UpperCases,
		},
	})
	if err != nil {
		t.Fatalf("generate password: %v", err)
	}
	if strings.ContainsFunc(password, unicode.IsUpper) {
		t.Fatalf("expected new lower-case configuration to take precedence, got %q", password)
	}
	if strings.ContainsFunc(password, unicode.IsDigit) {
		t.Fatalf("expected explicit numDigits=0 to be preserved, got %q", password)
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
