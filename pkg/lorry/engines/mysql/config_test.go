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

package mysql

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	fakeUser     = "fake-user"
	fakePassword = "fake-password"
	fakePemPath  = "fake-pem-path"
	fakeAddr     = "fake-addr"
)

var (
	fakeProperties = engines.Properties{
		connectionURLKey:   "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true",
		maxOpenConnsKey:    "5",
		maxIdleConnsKey:    "4",
		connMaxLifetimeKey: "10m",
		connMaxIdleTimeKey: "500s",
	}

	fakePropertiesWithPem = engines.Properties{
		pemPathKey: fakePemPath,
	}

	fakePropertiesWithWrongURL = engines.Properties{
		connectionURLKey: "fake-url",
	}
)

func TestNewConfig(t *testing.T) {
	fs = afero.NewMemMapFs()
	defer func() {
		fs = afero.NewOsFs()
		viper.Reset()
	}()

	t.Run("with empty properties", func(t *testing.T) {
		fakeConfig, err := NewConfig(map[string]string{})
		assert.Nil(t, err)
		assert.NotNil(t, fakeConfig)
		assert.Equal(t, "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true", fakeConfig.url)
	})

	t.Run("with default properties", func(t *testing.T) {
		viper.Set(constant.KBEnvServiceUser, fakeUser)
		viper.Set(constant.KBEnvServicePassword, fakePassword)

		fakeConfig, err := NewConfig(fakeProperties)
		assert.Nil(t, err)
		assert.NotNil(t, fakeConfig)
		assert.Equal(t, "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true", fakeConfig.url)
		assert.Equal(t, 5, fakeConfig.maxOpenConns)
		assert.Equal(t, 4, fakeConfig.maxIdleConns)
		assert.Equal(t, time.Minute*10, fakeConfig.connMaxLifetime)
		assert.Equal(t, time.Second*500, fakeConfig.connMaxIdletime)
		assert.Equal(t, fakeUser, fakeConfig.username)
		assert.Equal(t, fakePassword, fakeConfig.password)
	})

	t.Run("can't open pem file", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakePropertiesWithPem)
		assert.Nil(t, fakeConfig)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "Error reading PEM file from fake-pem-path")
	})

	f, err := fs.Create(fakePemPath)
	assert.Nil(t, err)
	_ = f.Close()
	t.Run("", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakePropertiesWithPem)
		assert.Nil(t, fakeConfig)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "failed to append PEM")
	})
}

func TestConfig_GetLocalDBConn(t *testing.T) {
	t.Run("parse dsn failed", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakePropertiesWithWrongURL)
		assert.Nil(t, err)

		db, err := fakeConfig.GetLocalDBConn()
		assert.Nil(t, db)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "illegal Data Source Name (DNS) specified by url")
	})

	t.Run("get DB connection with addr successfully", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakeProperties)
		assert.Nil(t, err)

		db, err := fakeConfig.GetLocalDBConn()
		assert.Nil(t, err)
		assert.NotNil(t, db)
	})
}

func TestConfig_GetDBConnWithAddr(t *testing.T) {
	t.Run("parse dsn failed", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakePropertiesWithWrongURL)
		assert.Nil(t, err)

		db, err := fakeConfig.GetDBConnWithAddr(fakeAddr)
		assert.Nil(t, db)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "illegal Data Source Name (DNS) specified by url")
	})

	t.Run("get local DB connection successfully", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakeProperties)
		assert.Nil(t, err)

		db, err := fakeConfig.GetDBConnWithAddr(fakeAddr)
		assert.Nil(t, err)
		assert.NotNil(t, db)
	})
}

func TestConfig_GetDBPort(t *testing.T) {
	t.Run("parse dsn failed", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakePropertiesWithWrongURL)
		assert.Nil(t, err)

		port := fakeConfig.GetDBPort()
		assert.Equal(t, 3306, port)
	})

	t.Run("get db port successfully", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakeProperties)
		assert.Nil(t, err)

		port := fakeConfig.GetDBPort()
		assert.Equal(t, 3306, port)
	})
}
