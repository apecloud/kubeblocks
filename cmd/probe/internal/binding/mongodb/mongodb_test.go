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
	"context"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
)

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
	properties := map[string]string{
		"host":         "127.0.0.2",
		"databaseName": "TestDB",
		"username":     "username",
		"password":     "password",
	}
	m := bindings.Metadata{
		Base: metadata.Base{Properties: properties},
	}
	m := &MongoDBOperations{
		BaseOperations: BaseOperations{Logger: logger.NewLogger("mongodb-test")},
	}
	m.Init(m)
	m.manager.DB = mt.Client.Database(adminDatabase)
	role, err := m.GetRole(context.Background(), &bindings.InvokeRequest{}, &bindings.InvokeResponse{})
	if err != nil {
		t.Errorf("getRole error: %s", err)
	}
	if role != "primary" {
		t.Errorf("unexpected role: %s", role)
	}
}
