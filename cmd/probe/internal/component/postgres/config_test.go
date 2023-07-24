package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPostgresqlMetadata(t *testing.T) {
	t.Run("With defaults", func(t *testing.T) {
		properties := map[string]string{
			connectionURLKey: "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10",
		}

		metadata, err := NewConfig(properties)
		assert.Nil(t, err)
		assert.Equal(t, "postgres", metadata.username)
		assert.Equal(t, "docker", metadata.password)
		assert.Equal(t, "localhost", metadata.host)
		assert.Equal(t, 5432, metadata.port)
		assert.Equal(t, "postgres", metadata.database)
		assert.Equal(t, int32(1), metadata.minConns)
		assert.Equal(t, int32(10), metadata.maxConns)
	})

	t.Run("url not set", func(t *testing.T) {
		properties := map[string]string{}

		_, err := NewConfig(properties)
		assert.NotNil(t, err)
	})

	t.Run("pool max connection too small", func(t *testing.T) {
		properties := map[string]string{
			connectionURLKey: "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=0",
		}

		_, err := NewConfig(properties)
		assert.NotNil(t, err)
	})
}
