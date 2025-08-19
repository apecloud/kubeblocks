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
	"math/rand"
	"strings"
	"time"
	"unicode"

	"github.com/sethvargo/go-password/password"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	// defaultSymbols is the list of default symbols to generate password.
	defaultSymbols = "!@#&*"
)

type passwordReader struct {
	rand *rand.Rand
}

func (r *passwordReader) Read(data []byte) (int, error) {
	return r.rand.Read(data)
}

func (r *passwordReader) Seed(seed int64) {
	r.rand.Seed(seed)
}

func GeneratePasswordByConfig(config appsv1.PasswordConfig) (string, error) {
	passwd, err := generatePassword((int)(config.Length), (int)(config.NumDigits), (int)(config.NumSymbols), config.Seed, config.SymbolCharacters)
	if err != nil {
		return "", err
	}
	switch config.LetterCase {
	case appsv1.UpperCases:
		passwd = strings.ToUpper(passwd)
	case appsv1.LowerCases:
		passwd = strings.ToLower(passwd)
	case appsv1.MixedCases:
		passwd, err = ensureMixedCase(passwd, config.Seed)
	}
	return passwd, err
}

// generatePassword generates a password with the given requirements and seed in lowercase.
func generatePassword(length, numDigits, numSymbols int, seed string, symbols string) (string, error) {
	// #nosec G404
	rand, err := newRandFromSeed(seed)
	if err != nil {
		return "", err
	}
	if symbols == "" {
		symbols = defaultSymbols
	}
	gen, err := password.NewGenerator(&password.GeneratorInput{
		LowerLetters: password.LowerLetters,
		UpperLetters: password.UpperLetters,
		Symbols:      symbols,
		Digits:       password.Digits,
		Reader:       &passwordReader{rand: rand},
	})
	if err != nil {
		return "", err
	}
	return gen.Generate(length, numDigits, numSymbols, true, true)
}

// ensureMixedCase randomizes the letter casing in the given string, ensuring
// that the result contains at least one uppercase and one lowercase letter.
// If the give string only has one letter, it is returned unmodified.
func ensureMixedCase(in, seed string) (string, error) {
	runes := []rune(in)
	letterIndices := make([]int, 0, len(runes))
	for i, r := range runes {
		if unicode.IsLetter(r) {
			letterIndices = append(letterIndices, i)
		}
	}
	L := len(letterIndices)

	if L < 2 {
		return in, nil
	}

	// Get a random number x in [1, 2^L - 2], whose binary list will be used to determine the letter casing.
	// avoid the all-0 and all-1 patterns, which cause all-lowercase and all-uppercase password.
	rand, err := newRandFromSeed(seed)
	if err != nil {
		return in, err
	}
	x := uint64(rand.Int63n(int64((1<<L)-2))) + 1

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

func newRandFromSeed(seed string) (*rand.Rand, error) {
	if seed == "" {
		return rand.New(rand.NewSource(time.Now().UnixNano())), nil
	}
	// Convert seed string to a 64-bit integer via SHA-256
	h := sha256.New()
	if _, err := h.Write([]byte(seed)); err != nil {
		return nil, err
	}
	sum := h.Sum(nil)
	uSeed := binary.BigEndian.Uint64(sum)
	return rand.New(rand.NewSource(int64(uSeed))), nil
}
