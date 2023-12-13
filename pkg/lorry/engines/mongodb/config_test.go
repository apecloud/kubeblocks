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

package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMongoDBMetadata(t *testing.T) {
	t.Run("With defaults", func(t *testing.T) {
		properties := map[string]string{
			host: "127.0.0.1",
		}

		metadata, err := NewConfig(properties)
		assert.Nil(t, err)
		assert.Equal(t, properties[host], metadata.Hosts[0])
		assert.Equal(t, adminDatabase, metadata.DatabaseName)
	})

	t.Run("With custom values", func(t *testing.T) {
		properties := map[string]string{
			host:         "127.0.0.2",
			databaseName: "TestDB",
			username:     "username",
			password:     "password",
		}

		metadata, err := NewConfig(properties)
		assert.Nil(t, err)
		assert.Equal(t, properties[host], metadata.Hosts[0])
		assert.Equal(t, properties[databaseName], metadata.DatabaseName)
		assert.Equal(t, properties[username], metadata.Username)
		assert.Equal(t, properties[password], metadata.Password)
	})

	t.Run("Missing hosts", func(t *testing.T) {
		properties := map[string]string{
			username: "username",
			password: "password",
		}

		_, err := NewConfig(properties)
		assert.NotNil(t, err)
	})

	t.Run("Invalid without host/server", func(t *testing.T) {
		properties := map[string]string{
			databaseName: "TestDB",
		}

		_, err := NewConfig(properties)
		assert.NotNil(t, err)

		expected := "must set 'host' in metadata or KB_SERVICE_PORT environment variable"
		assert.Equal(t, expected, err.Error())
	})
}
