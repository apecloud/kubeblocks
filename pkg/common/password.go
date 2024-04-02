/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	mathrand "math/rand"
	"time"

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
	var rand *mathrand.Rand
	if len(seed) == 0 {
		rand = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	} else {
		h := sha256.New()
		_, err := h.Write([]byte(seed))
		if err != nil {
			return "", err
		}
		uSeed := binary.BigEndian.Uint64(h.Sum(nil))
		rand = mathrand.New(mathrand.NewSource(int64(uSeed)))
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
	return gen.Generate(length, numDigits, numSymbols, noUpper, true)
}
