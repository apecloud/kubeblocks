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

package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestGetPostgresqlMetadata(t *testing.T) {
	t.Run("With defaults", func(t *testing.T) {
		properties := map[string]string{
			ConnectionURLKey: "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10",
		}

		metadata, err := NewConfig(properties)
		assert.Nil(t, err)
		assert.Equal(t, "postgres", metadata.Username)
		assert.Equal(t, "docker", metadata.Password)
		assert.Equal(t, "localhost", metadata.Host)
		assert.Equal(t, 5432, metadata.Port)
		assert.Equal(t, "postgres", metadata.Database)
		assert.Equal(t, int32(1), metadata.MinConnections)
		assert.Equal(t, int32(10), metadata.MaxConnections)
	})

	t.Run("url not set", func(t *testing.T) {
		properties := map[string]string{}

		_, err := NewConfig(properties)
		assert.NotNil(t, err)
	})

	t.Run("pool max connection too small", func(t *testing.T) {
		properties := map[string]string{
			ConnectionURLKey: "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=0",
		}

		_, err := NewConfig(properties)
		assert.NotNil(t, err)
	})

	t.Run("set env", func(t *testing.T) {
		viper.Set("KB_SERVICE_USER", "test")
		viper.Set("KB_SERVICE_PASSWORD", "test_pwd")
		properties := map[string]string{
			ConnectionURLKey: "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10",
		}
		metadata, err := NewConfig(properties)
		assert.Nil(t, err)

		assert.Equal(t, metadata.Username, "test")
		assert.Equal(t, metadata.Password, "test_pwd")
	})
}

func TestConfigFunc(t *testing.T) {
	properties := map[string]string{
		ConnectionURLKey: "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10",
	}
	metadata, err := NewConfig(properties)
	assert.NotNil(t, metadata)
	assert.Nil(t, err)

	t.Run("get db port", func(t *testing.T) {
		port := metadata.GetDBPort()
		assert.Equal(t, port, 5432)

		metadata.Port = 0
		port = metadata.GetDBPort()
		assert.Equal(t, port, 5432)
	})

	t.Run("get consensus IP port", func(t *testing.T) {
		cluster := &dcs.Cluster{
			ClusterCompName: "test",
			Namespace:       "default",
		}

		consensusIPPort := metadata.GetConsensusIPPort(cluster, "test")
		assert.Equal(t, consensusIPPort, "test.test-headless.default.svc:15432")
	})
}
