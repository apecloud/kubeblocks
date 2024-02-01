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

// fork from github.com/sethvargo/go-password/password, and add GenerateWithSeed function to generate password with seed

package common

import (
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

type (
	MockReader struct {
		Counter int64
	}
)

const (
	N = 10000
)

func (mr *MockReader) Read(data []byte) (int, error) {
	for i := 0; i < len(data); i++ {
		data[i] = byte(atomic.AddInt64(&mr.Counter, 1))
	}
	return len(data), nil
}

func testHasDuplicates(tb testing.TB, s string) bool {
	found := make(map[rune]struct{}, len(s))
	for _, ch := range s {
		if _, ok := found[ch]; ok {
			return true
		}
		found[ch] = struct{}{}
	}
	return false
}

func testGeneratorGenerate(t *testing.T, reader io.Reader) {
	t.Parallel()

	gen, err := NewGenerator(nil)
	if reader != nil {
		gen.reader = reader
	}
	if err != nil {
		t.Fatal(err)
	}

	t.Run("exceeds_length", func(t *testing.T) {
		t.Parallel()

		if _, err := gen.Generate(0, 1, 0, false, false); err != ErrExceedsTotalLength {
			t.Errorf("expected %q to be %q", err, ErrExceedsTotalLength)
		}

		if _, err := gen.Generate(0, 0, 1, false, false); err != ErrExceedsTotalLength {
			t.Errorf("expected %q to be %q", err, ErrExceedsTotalLength)
		}
	})

	t.Run("exceeds_letters_available", func(t *testing.T) {
		t.Parallel()

		if _, err := gen.Generate(1000, 0, 0, false, false); err != ErrLettersExceedsAvailable {
			t.Errorf("expected %q to be %q", err, ErrLettersExceedsAvailable)
		}
	})

	t.Run("exceeds_digits_available", func(t *testing.T) {
		t.Parallel()

		if _, err := gen.Generate(52, 11, 0, false, false); err != ErrDigitsExceedsAvailable {
			t.Errorf("expected %q to be %q", err, ErrDigitsExceedsAvailable)
		}
	})

	t.Run("exceeds_symbols_available", func(t *testing.T) {
		t.Parallel()

		if _, err := gen.Generate(52, 0, 31, false, false); err != ErrSymbolsExceedsAvailable {
			t.Errorf("expected %q to be %q", err, ErrSymbolsExceedsAvailable)
		}
	})

	t.Run("gen_lowercase", func(t *testing.T) {
		t.Parallel()

		for i := 0; i < N; i++ {
			res, err := gen.Generate(i%len(LowerLetters), 0, 0, true, true)
			if err != nil {
				t.Error(err)
			}

			if res != strings.ToLower(res) {
				t.Errorf("%q is not lowercase", res)
			}
		}
	})

	t.Run("gen_uppercase", func(t *testing.T) {
		t.Parallel()

		res, err := gen.Generate(1000, 0, 0, false, true)
		if err != nil {
			t.Error(err)
		}

		if res == strings.ToLower(res) {
			t.Errorf("%q does not include uppercase", res)
		}
	})

	t.Run("gen_no_repeats", func(t *testing.T) {
		t.Parallel()

		for i := 0; i < N; i++ {
			res, err := gen.Generate(52, 10, 3, false, false)
			if err != nil {
				t.Error(err)
			}

			if testHasDuplicates(t, res) {
				t.Errorf("%q should not have duplicates", res)
			}
		}
	})
}

func TestGeneratorGenerate(t *testing.T) {
	testGeneratorGenerate(t, nil)
}

func TestGenerator_Reader_Generate(t *testing.T) {
	testGeneratorGenerate(t, &MockReader{})
}

func testGeneratorGenerateCustom(t *testing.T, reader io.Reader) {
	t.Parallel()

	gen, err := NewGenerator(&GeneratorInput{
		LowerLetters: "abcde",
		UpperLetters: "ABCDE",
		Symbols:      "!@#$%",
		Digits:       "01234",
		Reader:       reader,
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < N; i++ {
		res, err := gen.Generate(52, 10, 10, false, true)
		if err != nil {
			t.Error(err)
		}

		if strings.Contains(res, "f") {
			t.Errorf("%q should only contain lower letters abcde", res)
		}

		if strings.Contains(res, "F") {
			t.Errorf("%q should only contain upper letters ABCDE", res)
		}

		if strings.Contains(res, "&") {
			t.Errorf("%q should only include symbols !@#$%%", res)
		}

		if strings.Contains(res, "5") {
			t.Errorf("%q should only contain digits 01234", res)
		}
	}
}

func testGeneratorGenerateWithSeed(t *testing.T, reader io.Reader) {
	gen, err := NewGenerator(&GeneratorInput{
		LowerLetters: "abcde",
		UpperLetters: "ABCDE",
		Symbols:      "!@#$%",
		Digits:       "01234",
		Reader:       reader,
	})
	if err != nil {
		t.Fatal(err)
	}

	seed := "mock-seed-for-generate-the same-password"
	resultSeedFirstTime := ""
	resultSeedEachTime := ""
	for i := 0; i < N; i++ {
		res, err := gen.GenerateWithSeed(10, 5, 0, false, seed)
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

func TestGeneratorGenerateCustom(t *testing.T) {
	testGeneratorGenerateCustom(t, nil)
}

func TestGenerator_Reader_Generate_Custom(t *testing.T) {
	testGeneratorGenerateCustom(t, &MockReader{})
}

func TestGeneratorGenerateWithSeed(t *testing.T) {
	testGeneratorGenerateWithSeed(t, nil)
}

func TestGeneratorGenerateWithSeedReader(t *testing.T) {
	testGeneratorGenerateWithSeed(t, &MockReader{})
}
