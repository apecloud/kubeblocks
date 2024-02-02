/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
)

const (
	N = 10000
)

func testGeneratorGeneratePasswordWithSeed(t *testing.T) {
	seed := "mock-seed-for-generate-the same-password"
	resultSeedFirstTime := ""
	resultSeedEachTime := ""
	for i := 0; i < N; i++ {
		res, err := GeneratePasswordWithSeed(10, 5, 0, false, seed)
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

func TestGeneratorGeneratePasswordWithSeed(t *testing.T) {
	testGeneratorGeneratePasswordWithSeed(t)
}
