/*
Copyright ApeCloud, Inc.

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

package mongodb

import (
	"context"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestGetMongoDBMetadata(t *testing.T) {
	t.Run("With defaults", func(t *testing.T) {
		properties := map[string]string{
			host: "127.0.0.1",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		metadata, err := getMongoDBMetaData(m)
		assert.Nil(t, err)
		assert.Equal(t, properties[host], metadata.host)
		assert.Equal(t, adminDatabase, metadata.databaseName)
	})

	t.Run("With custom values", func(t *testing.T) {
		properties := map[string]string{
			host:         "127.0.0.2",
			databaseName: "TestDB",
			username:     "username",
			password:     "password",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		metadata, err := getMongoDBMetaData(m)
		assert.Nil(t, err)
		assert.Equal(t, properties[host], metadata.host)
		assert.Equal(t, properties[databaseName], metadata.databaseName)
		assert.Equal(t, properties[username], metadata.username)
		assert.Equal(t, properties[password], metadata.password)
	})

	t.Run("Missing hosts", func(t *testing.T) {
		properties := map[string]string{
			username: "username",
			password: "password",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		_, err := getMongoDBMetaData(m)
		assert.NotNil(t, err)
	})

	t.Run("Valid connection string without params", func(t *testing.T) {
		properties := map[string]string{
			host:         "127.0.0.2",
			databaseName: "TestDB",
			username:     "username",
			password:     "password",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		metadata, err := getMongoDBMetaData(m)
		assert.Nil(t, err)

		uri := getMongoURI(metadata)
		expected := "mongodb://username:password@127.0.0.2/TestDB"

		assert.Equal(t, expected, uri)
	})

	t.Run("Valid connection string without username", func(t *testing.T) {
		properties := map[string]string{
			host:         "localhost:27017",
			databaseName: "TestDB",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		metadata, err := getMongoDBMetaData(m)
		assert.Nil(t, err)

		uri := getMongoURI(metadata)
		expected := "mongodb://localhost:27017/TestDB"

		assert.Equal(t, expected, uri)
	})

	t.Run("Valid connection string with params", func(t *testing.T) {
		properties := map[string]string{
			host:         "127.0.0.2",
			databaseName: "TestDB",
			username:     "username",
			password:     "password",
			params:       "?ssl=true",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		metadata, err := getMongoDBMetaData(m)
		assert.Nil(t, err)

		uri := getMongoURI(metadata)
		expected := "mongodb://username:password@127.0.0.2/TestDB?ssl=true"

		assert.Equal(t, expected, uri)
	})

	t.Run("Valid connection string with DNS SRV", func(t *testing.T) {
		properties := map[string]string{
			server:       "server.example.com",
			databaseName: "TestDB",
			params:       "?ssl=true",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		metadata, err := getMongoDBMetaData(m)
		assert.Nil(t, err)

		uri := getMongoURI(metadata)
		expected := "mongodb+srv://server.example.com/?ssl=true"

		assert.Equal(t, expected, uri)
	})

	t.Run("Invalid without host/server", func(t *testing.T) {
		properties := map[string]string{
			databaseName: "TestDB",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		_, err := getMongoDBMetaData(m)
		assert.NotNil(t, err)

		expected := "must set 'host' or 'server' fields in metadata"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("Invalid with both host/server", func(t *testing.T) {
		properties := map[string]string{
			server:       "server.example.com",
			host:         "127.0.0.2",
			databaseName: "TestDB",
		}
		m := bindings.Metadata{
			Base: metadata.Base{Properties: properties},
		}

		_, err := getMongoDBMetaData(m)
		assert.NotNil(t, err)

		expected := "'host' or 'server' fields are mutually exclusive"
		assert.Equal(t, expected, err.Error())
	})
}

func TestGetRole(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ShareClient(true).ClientType(mtest.Mock))
	defer mt.Close()

	mt.AddMockResponses(bson.D{
		primitive.E{Key: "ok", Value: 1},
		primitive.E{Key: "myState", Value: 1},
		primitive.E{Key: "members", Value: bson.A{
			bson.D{
				primitive.E{Key: "_id", Value: 0},
				primitive.E{Key: "state", Value: 1},
				primitive.E{Key: "stateStr", Value: "PRIMARY"},
			},
		}},
	})
	m := &MongoDB{
		database: mt.Client.Database(adminDatabase),
		logger:   logger.NewLogger("mongodb-test"),
	}
	role, err := m.GetRole(context.Background(), &bindings.InvokeRequest{}, &bindings.InvokeResponse{})
	if err != nil {
		t.Errorf("getRole error: %s", err)
	}
	if role != "primary" {
		t.Errorf("unexpected role: %s", role)
	}
}
