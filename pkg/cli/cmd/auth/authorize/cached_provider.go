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

package authorize

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/auth/authorize/authenticator"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

const (
	authDir      = "auth"
	userInfoFile = "user_info.json"
	tokenFile    = "token.json"

	keyringKey     = "token"
	keyringService = "kubeblocks"
	keyringLabel   = "KUBEBLOCKS CLI"

	fileMode = 0o600
)

type KeyringCached struct {
	key     string
	valid   bool
	keyring keyring.Keyring
}

func (k *KeyringCached) isValid() bool {
	return k.valid
}

func (k *KeyringCached) get() ([]byte, error) {
	item, err := k.keyring.Get(k.key)
	if err != nil {
		return nil, err
	}
	return item.Data, nil
}

func (k *KeyringCached) set(data []byte) error {
	return k.keyring.Set(keyring.Item{
		Key:   k.key,
		Data:  data,
		Label: keyringLabel,
	})
}

func (k *KeyringCached) remove() error {
	return k.keyring.Remove(k.key)
}

type FileCached struct {
	tokenFilename    string
	userInfoFilename string
}

type KeyringCachedTokenProvider struct {
	keyringCached KeyringProvider
	fileCached    FileCached
}

func NewKeyringCachedTokenProvider(keyringCached *KeyringProvider) *KeyringCachedTokenProvider {
	fileCached := FileCached{
		tokenFilename:    tokenFile,
		userInfoFilename: userInfoFile,
	}

	if keyringCached == nil {
		defaultKeyring, isValid := getDefaultKeyring()
		return &KeyringCachedTokenProvider{
			keyringCached: &KeyringCached{
				key:     keyringKey,
				valid:   isValid,
				keyring: defaultKeyring,
			},
			fileCached: fileCached,
		}
	}

	return &KeyringCachedTokenProvider{
		keyringCached: *keyringCached,
		fileCached:    fileCached,
	}
}

func getDefaultKeyring() (keyring.Keyring, bool) {
	k, err := keyring.Open(keyring.Config{
		AllowedBackends: []keyring.BackendType{
			keyring.SecretServiceBackend,
			keyring.KWalletBackend,
			keyring.KeychainBackend,
			keyring.WinCredBackend,
		},
		ServiceName:              keyringService,
		KeychainTrustApplication: true,
		KeychainSynchronizable:   true,
	})

	if err != nil {
		return nil, false
	}
	return k, true
}

func (k *KeyringCachedTokenProvider) GetTokens() (*authenticator.TokenResponse, error) {
	if !k.keyringCached.isValid() {
		token, tokenErr := k.fileCached.readToken()
		if os.IsNotExist(tokenErr) {
			return nil, nil
		}
		return token, tokenErr
	}

	data, err := k.keyringCached.get()
	if err != nil {
		if err == keyring.ErrKeyNotFound {
			return nil, nil
		}
		return nil, errors.Wrap(err, "error getting token information from keyring")
	}

	var tokenResponse authenticator.TokenResponse
	err = json.Unmarshal(data, &tokenResponse)
	if err != nil {
		return nil, errors.Wrap(err, "could not unmarshal token data from keyring")
	}

	return &tokenResponse, nil
}

func (k *KeyringCachedTokenProvider) cacheTokens(tokenResponse *authenticator.TokenResponse) error {
	data, err := json.Marshal(tokenResponse)
	if err != nil {
		return errors.Wrap(err, "could not marshal token data for keyring")
	}

	if !k.keyringCached.isValid() {
		return k.fileCached.writeToken(data)
	}

	return k.keyringCached.set(data)
}

func (k *KeyringCachedTokenProvider) deleteTokens() error {
	if !k.keyringCached.isValid() {
		return k.fileCached.deleteToken()
	}

	return k.keyringCached.remove()
}

func (k *KeyringCachedTokenProvider) cacheUserInfo(userInfo *authenticator.UserInfoResponse) error {
	saveDir, err := k.fileCached.getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}
	savePath := filepath.Join(saveDir, k.fileCached.userInfoFilename)

	newData, err := json.Marshal(userInfo)
	if err != nil {
		return errors.Wrap(err, "failed to marshal user info")
	}

	if err := os.WriteFile(savePath, newData, fileMode); err != nil {
		return errors.Wrap(err, "failed to write user info file")
	}
	return nil
}

func (k *KeyringCachedTokenProvider) getUserInfo() (*authenticator.UserInfoResponse, error) {
	saveDir, err := k.fileCached.getConfigDir()
	if err != nil {
		return nil, err
	}
	savePath := filepath.Join(saveDir, k.fileCached.userInfoFilename)
	data, err := os.ReadFile(savePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read user info file")
	}

	var userInfo authenticator.UserInfoResponse
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal user info")
	}
	return &userInfo, nil
}

func (f *FileCached) getConfigDir() (string, error) {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cliHomeDir, authDir), nil
}

func (f *FileCached) getTokenPath() (string, error) {
	dir, err := f.getConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, f.tokenFilename), nil
}

func (f *FileCached) writeToken(data []byte) error {
	tokenPath, err := f.getTokenPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(tokenPath)

	_, err = os.Stat(configDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(configDir, os.ModePerm)
		if err != nil {
			return errors.New("error creating config directory")
		}
	} else if err != nil {
		return err
	}

	err = os.WriteFile(tokenPath, data, fileMode)
	if err != nil {
		return errors.Wrap(err, "error writing token")
	}

	return nil
}

func (f *FileCached) readToken() (*authenticator.TokenResponse, error) {
	var data []byte
	tokenPath, err := f.getTokenPath()
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		return nil, err
	} else {
		if stat.Mode()&^fileMode != 0 {
			err = os.Chmod(tokenPath, fileMode)
			if err != nil {
				log.Printf("Unable to change %v file mode to 0%o: %v", tokenPath, fileMode, err)
			}
		}
		data, err = os.ReadFile(tokenPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	var tokenResponse *authenticator.TokenResponse
	err = json.Unmarshal(data, tokenResponse)
	if err != nil {
		return nil, err
	}

	return tokenResponse, nil
}

func (f *FileCached) deleteToken() error {
	tokenPath, err := f.getTokenPath()
	if err != nil {
		return err
	}

	err = os.Remove(tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "error removing access token file")
		}
	}

	configFile, err := f.getConfigDir()
	if err != nil {
		return err
	}

	err = os.Remove(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "error removing default config file")
		}
	}
	return nil
}
