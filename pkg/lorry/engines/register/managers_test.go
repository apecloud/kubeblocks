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

package register

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	fakeCharacterType = "fake-db"
	fakeWrongContent  = "wrong"
	fakeConfigContent = `
name: fake-db
spec:
  version: v1
  metadata:
    - name: url # Required
      value: "user=test password=test host=localhost"`
	fakeConfigFile = "/fake-config-file"
	fakeConfigDir  = "fake-dir"
)

func TestReadConfig(t *testing.T) {
	fs = afero.NewMemMapFs()
	viper.SetFs(fs)
	defer func() {
		fs = afero.NewOsFs()
		viper.Reset()
	}()

	t.Run("viper read in config failed", func(t *testing.T) {
		name, property, err := readConfig(fakeConfigFile)
		assert.NotNil(t, err)
		assert.Nil(t, property)
		assert.Equal(t, "", name)
	})

	file, err := fs.Create(fakeConfigFile)
	assert.Nil(t, err)
	_, err = file.WriteString(fakeConfigContent)
	assert.Nil(t, err)
	_ = file.Close()

	t.Run("read config successfully", func(t *testing.T) {
		name, property, err := readConfig(fakeConfigFile)
		assert.Nil(t, err)
		assert.Equal(t, fakeCharacterType, name)
		assert.Equal(t, "user=test password=test host=localhost", property["url"])
	})
}

func TestGetAllComponent(t *testing.T) {
	fs = afero.NewMemMapFs()
	viper.SetFs(fs)
	defer func() {
		fs = afero.NewOsFs()
		viper.Reset()
	}()

	t.Run("read dir failed", func(t *testing.T) {
		err := GetAllComponent(fakeConfigDir)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "file does not exist")
	})

	err := fs.Mkdir(fakeConfigDir, os.ModeDir)
	assert.Nil(t, err)
	file, err := fs.Create(fakeConfigDir + fakeConfigFile)
	assert.Nil(t, err)
	_, err = file.WriteString(fakeWrongContent)
	assert.Nil(t, err)
	_ = file.Close()

	t.Run("read config failed", func(t *testing.T) {
		err = GetAllComponent(fakeConfigDir)
		assert.NotNil(t, err)
	})

	err = fs.Remove(fakeConfigDir + fakeConfigFile)
	assert.Nil(t, err)
	file, err = fs.Create(fakeConfigDir + fakeConfigFile)
	assert.Nil(t, err)
	_, err = file.WriteString(fakeConfigContent)
	_ = file.Close()

	t.Run("get all component successfully", func(t *testing.T) {
		err = GetAllComponent(fakeConfigDir)
		assert.Nil(t, err)

		property := GetProperties(fakeCharacterType)
		assert.Equal(t, "user=test password=test host=localhost", property["url"])
	})
}

func TestInitDBManager(t *testing.T) {
	fs = afero.NewMemMapFs()
	viper.SetFs(fs)
	realDBManager := dbManager
	defer func() {
		fs = afero.NewOsFs()
		viper.Reset()
		dbManager = realDBManager
	}()
	configDir := fakeConfigDir

	t.Run("characterType not set", func(t *testing.T) {
		err := InitDBManager(configDir)

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "KB_SERVICE_CHARACTER_TYPE not set")
		_, err = GetDBManager()
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no db manager")
	})

	viper.Set(constant.KBEnvBuiltinHandler, fakeCharacterType)
	t.Run("get all component failed", func(t *testing.T) {
		err := InitDBManager(configDir)

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "fatal error config file")
		_, err = GetDBManager()
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no db manager")
	})

	err := fs.Mkdir(fakeConfigDir, os.ModeDir)
	assert.Nil(t, err)
	file, err := fs.Create(fakeConfigDir + fakeConfigFile)
	assert.Nil(t, err)
	_, err = file.WriteString(fakeConfigContent)
	assert.Nil(t, err)
	_ = file.Close()

	t.Run("new func nil", func(t *testing.T) {
		err = InitDBManager(configDir)

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no db manager for characterType fake-db and workloadType ")
		_, err = GetDBManager()
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no db manager")
	})

	fakeNewFunc := func(engines.Properties) (engines.DBManager, error) {
		return nil, fmt.Errorf("some error")
	}
	RegisterEngine(fakeCharacterType, "", fakeNewFunc, nil)
	t.Run("new func failed", func(t *testing.T) {
		err = InitDBManager(configDir)

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
		_, err = GetDBManager()
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no db manager")
	})

	fakeNewFunc = func(engines.Properties) (engines.DBManager, error) {
		return &engines.MockManager{}, nil
	}
	RegisterEngine(fakeCharacterType, "", fakeNewFunc, func() engines.ClusterCommands {
		return nil
	})
	t.Run("new func successfully", func(t *testing.T) {
		err = InitDBManager(configDir)

		assert.Nil(t, err)
		_, err = GetDBManager()
		assert.Nil(t, err)
	})

	SetDBManager(&engines.MockManager{})
	t.Run("db manager exists", func(t *testing.T) {
		err = InitDBManager(configDir)
		assert.Nil(t, err)
		_, err = GetDBManager()
		assert.Nil(t, err)
	})

	t.Run("new cluster command", func(t *testing.T) {
		_, err = NewClusterCommands("")
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "unsupported engine type: ")
		_, err = NewClusterCommands(fakeCharacterType)
		assert.Nil(t, err)
	})
}
