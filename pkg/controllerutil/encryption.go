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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const purposeEncryptionKey = "encryption"

type encryptor struct {
	encryptionKey     []byte
	encryptionHashKey []byte
	gcm               cipher.AEAD
}

func NewEncryptor(encryptionKey string) *encryptor {
	return &encryptor{
		encryptionKey: []byte(encryptionKey),
	}
}

func (e *encryptor) deriveKey() {
	key := make([]byte, 32)
	k := hkdf.New(sha256.New, e.encryptionKey, []byte(purposeEncryptionKey), nil)
	_, _ = io.ReadFull(k, key)
	e.encryptionHashKey = key
}

func (e *encryptor) init() error {
	e.deriveKey()
	block, err := aes.NewCipher(e.encryptionHashKey)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	e.gcm = gcm
	return nil
}

// Encrypt encrypts the plaintext by secretKey by nonce string.
func (e *encryptor) Encrypt(plaintext []byte) (string, error) {
	if err := e.init(); err != nil {
		return "", err
	}
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := e.gcm.Seal(nil, nonce, plaintext, e.encryptionHashKey)
	// append nonce to ciphertext for decrypt
	ciphertext = append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts the ciphertext by secretKey.
func (e *encryptor) Decrypt(ciphertext []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return "", err
	}
	if err = e.init(); err != nil {
		return "", err
	}
	// fetch nonce from secret key
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext is too short")
	}
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, e.encryptionHashKey)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
