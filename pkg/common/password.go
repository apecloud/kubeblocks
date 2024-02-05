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

	"github.com/sethvargo/go-password/password"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	// Letters is the list of all letters.
	Letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Symbols is the list of symbols.
	Symbols = "!@#&*"
)

// GeneratePassword generates a password with the given requirements and seed.
func GeneratePassword(length, numDigits, numSymbols int, noUpper bool, seed string) (string, error) {
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

	chars := length - numDigits - numSymbols
	if chars < 0 {
		return "", errors.New("number of digits and symbols must be less than total length")
	}

	passwd := make([]byte, length)
	// Characters
	for i := 0; i < chars; i++ {
		if noUpper {
			passwd[i] = password.LowerLetters[utilrand.Intn(len(password.LowerLetters))]
		} else {
			passwd[i] = Letters[utilrand.Intn(len(Letters))]
		}
	}

	// Digits
	for i := 0; i < numDigits; i++ {
		passwd[chars+i] = password.Digits[utilrand.Intn(len(password.Digits))]
	}

	// Symbols
	for i := 0; i < numSymbols; i++ {
		passwd[chars+numDigits+i] = Symbols[utilrand.Intn(len(Symbols))]
	}

	// Shuffle the password characters
	if len(seed) > 0 {
		for i := 0; i < length; i++ {
			j := utilrand.Intn(length)
			passwd[i], passwd[j] = passwd[j], passwd[i]
		}
	}

	return string(passwd), nil
}
