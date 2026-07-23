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

package v1

import "testing"

func TestSystemAccountLegacyGoAPICompatibility(t *testing.T) {
	legacyConfig := PasswordConfig{NumDigits: 4}
	legacyAccount := SystemAccount{PasswordGenerationPolicy: legacyConfig}

	var digits int32 = 2
	legacyConfig.NumDigits = digits
	legacyAccount.PasswordGenerationPolicy = legacyConfig

	if legacyAccount.PasswordGenerationPolicy.NumDigits != digits {
		t.Fatalf("expected legacy assignment to preserve numDigits=%d", digits)
	}
}
