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
