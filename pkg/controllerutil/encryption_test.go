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

package controllerutil

import (
	"testing"
)

func TestEncryptor(t *testing.T) {
	secretKey := "dp-aes-test"
	e := NewEncryptor(secretKey)
	plaintexts := []string{"password1", "test-password", "passwr0d", "$2@dsR^T"}
	for _, v := range plaintexts {
		ciphertext, err := e.Encrypt([]byte(v))
		if err != nil {
			t.Error(err.Error())
		}
		plaintext, err := e.Decrypt([]byte(ciphertext))
		if err != nil {
			t.Error(err.Error())
		}
		if plaintext != v {
			t.Errorf("encrypt/decrypt value is incorrect for %s", v)
		}
	}
}
