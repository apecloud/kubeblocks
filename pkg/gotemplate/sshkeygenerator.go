/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gotemplate

import (
	"fmt"

	"github.com/charmbracelet/keygen"
)

const memoString = "kubeblocks@localhost"

type SSHKeyPair struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

func sshKeyGenerate(passphrase ...string) (*SSHKeyPair, error) {
	options := []keygen.Option{
		keygen.WithKeyType(keygen.RSA),
	}

	if len(passphrase) > 0 {
		options = append(options, keygen.WithPassphrase(passphrase[0]))
	}
	generator, err := keygen.New("", options...)
	if err != nil {
		return nil, err
	}

	rawPrimaryKey := generator.RawProtectedPrivateKey()
	if rawPrimaryKey == nil {
		return nil, keygen.ErrMissingSSHKeys
	}
	ak := generator.AuthorizedKey()
	return &SSHKeyPair{
		PrivateKey: string(rawPrimaryKey),
		// memo is optional
		PublicKey: fmt.Sprintf("%s %s", ak, memoString),
	}, nil
}
