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
	"crypto/sha256"
	"encoding/binary"
	"errors"
	mathrand "math/rand"
	"time"
	"unicode"

	"github.com/sethvargo/go-password/password"
)

const (
	// Symbols is the list of symbols.
	Symbols = "!@#&*"
)

type PasswordReader struct {
	rand *mathrand.Rand
}

func (r *PasswordReader) Read(data []byte) (int, error) {
	return r.rand.Read(data)
}

func (r *PasswordReader) Seed(seed int64) {
	r.rand.Seed(seed)
}

// GeneratePassword generates a password with the given requirements and seed.
func GeneratePassword(length, numDigits, numSymbols int, noUpper bool, seed string) (string, error) {
	rand, err := newRngFromSeed(seed)
	if err != nil {
		return "", err
	}
	passwordReader := &PasswordReader{rand: rand}
	gen, err := password.NewGenerator(&password.GeneratorInput{
		LowerLetters: password.LowerLetters,
		UpperLetters: password.UpperLetters,
		Symbols:      Symbols,
		Digits:       password.Digits,
		Reader:       passwordReader,
	})
	if err != nil {
		return "", err
	}
	pwd, err := gen.Generate(length, numDigits, numSymbols, noUpper, true)
	if err != nil {
		return "", err
	}
	if noUpper {
		return pwd, nil
	} else {
		return EnsureMixedCase(pwd, seed)
	}
}

// EnsureMixedCase randomizes the letter casing in the given string, ensuring
// that the result contains at least one uppercase and one lowercase letter
// if there are at least two letters in total. Non-letter characters (digits,
// symbols) remain unchanged. If seed is non-empty, it provides deterministic
// randomness; otherwise, the current time is used for a non-deterministic seed.
func EnsureMixedCase(in, seed string) (string, error) {
	// Convert string to a mutable slice of runes.
	runes := []rune(in)

	// Gather indices of all letters in the string.
	letterIndices := make([]int, 0, len(runes))
	for i, r := range runes {
		if unicode.IsLetter(r) {
			letterIndices = append(letterIndices, i)
		}
	}
	L := len(letterIndices)

	// If fewer than two letters,cannot generate both uppercase and lowercase.
	if L < 2 {
		return in, errors.New("not enough letters in the password to generate mixed cases")
	}

	// We want an integer in [1, 2^L - 2], effectively discarding the all-0 and all-1 patterns.
	rng, err := newRngFromSeed(seed)
	if err != nil {
		return in, err
	}
	rangeSize := int64((1 << L) - 2)
	if rangeSize <= 0 {
		return in, nil
	}

	// Get a random number x in [0, rangeSize), then shift to [1, 2^L - 2].
	x := uint64(rng.Int63n(rangeSize)) + 1

	// For each letter, pick the bit at position i to decide uppercase (1) or lowercase (0).
	for i := 0; i < L; i++ {
		bit := (x >> i) & 1
		idx := letterIndices[i]
		if bit == 0 {
			runes[idx] = unicode.ToLower(runes[idx])
		} else {
			runes[idx] = unicode.ToUpper(runes[idx])
		}
	}

	return string(runes), nil
}

// newRngFromSeed initializes a *mathrand.Rand from the given seed. If seed is empty,
// it uses the current time, making the output non-deterministic.
func newRngFromSeed(seed string) (*mathrand.Rand, error) {
	if seed == "" {
		return mathrand.New(mathrand.NewSource(time.Now().UnixNano())), nil
	}
	// Convert seed string to a 64-bit integer via SHA-256
	h := sha256.New()
	if _, err := h.Write([]byte(seed)); err != nil {
		return nil, err
	}
	sum := h.Sum(nil)
	uSeed := binary.BigEndian.Uint64(sum)
	return mathrand.New(mathrand.NewSource(int64(uSeed))), nil
}
