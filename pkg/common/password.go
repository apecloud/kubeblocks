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
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	// LowerLetters is the list of lowercase letters.
	LowerLetters = "abcdefghijklmnopqrstuvwxyz"

	// UpperLetters is the list of uppercase letters.
	UpperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Letters is the list of all letters.
	Letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Digits is the list of permitted digits.
	Digits = "0123456789"

	// Symbols is the list of symbols.
	Symbols = "!@#&*"
)

var (
	// ErrExceedsTotalLength is the error returned with the number of digits and
	// symbols is greater than the total length.
	ErrExceedsTotalLength = errors.New("number of digits and symbols must be less than total length")
)

// GeneratePasswordWithSeed generates a password with the given requirements and seed.
func GeneratePasswordWithSeed(length, numDigits, numSymbols int, noUpper bool, seed string) (string, error) {
	if len(seed) == 0 {
		utilrand.Seed(time.Now().UnixNano())
	} else {
		h := sha256.New()
		_, err := h.Write([]byte(seed))
		if err != nil {
			return "", err
		}
		uSeed := binary.BigEndian.Uint64(h.Sum(nil))
		utilrand.Seed(int64(uSeed))
	}

	letters := LowerLetters
	if !noUpper {
		letters += UpperLetters
	}

	chars := length - numDigits - numSymbols
	if chars < 0 {
		return "", ErrExceedsTotalLength
	}

	password := make([]byte, length)
	// Characters
	for i := 0; i < chars; i++ {
		if noUpper {
			password[i] = LowerLetters[utilrand.Intn(len(LowerLetters))]
		} else {
			password[i] = Letters[utilrand.Intn(len(Letters))]
		}
	}

	// Digits
	for i := 0; i < numDigits; i++ {
		password[chars+i] = Digits[utilrand.Intn(len(Digits))]
	}

	// Symbols
	for i := 0; i < numSymbols; i++ {
		password[chars+numDigits+i] = Symbols[utilrand.Intn(len(Symbols))]
	}

	return string(password), nil
}
