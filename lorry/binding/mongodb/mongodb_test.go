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

	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"

	. "github.com/apecloud/kubeblocks/lorry/binding"
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
				primitive.E{Key: "self", Value: true},
			},
		}},
	})
	properties := map[string]string{
		"host":         "127.0.0.2",
		"databaseName": "TestDB",
		"username":     "username",
		"password":     "password",
	}
	development, _ := zap.NewDevelopment()
	m := &MongoDBOperations{
		BaseOperations: BaseOperations{Logger: zapr.NewLogger(development)},
	}
	err := m.Init(properties)
	assert.Nil(t, err)
	m.manager.Client = mt.Client
	role, err := m.GetRole(context.Background(), &ProbeRequest{}, &ProbeResponse{})
	if err != nil {
		t.Errorf("getRole error: %s", err)
	}
	if role != "primary" {
		t.Errorf("unexpected role: %s", role)
	}
}
