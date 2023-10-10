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
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/mongodb"
	"github.com/apecloud/kubeblocks/lorry/util"
)

// MongoDBOperations is a binding implementation for MongoDB.
type MongoDBOperations struct {
	manager *mongodb.Manager
	BaseOperations
}

// type OpTime struct {
// 	TS primitive.Timestamp `bson:"ts"`
// 	T  int64               `bson:"t"`
// }
//
// type ReplSetMember struct {
// 	ID                   int64               `bson:"_id"`
// 	Name                 string              `bson:"name"`
// 	Health               int64               `bson:"health"`
// 	State                int64               `bson:"state"`
// 	StateStr             string              `bson:"stateStr"`
// 	Uptime               int64               `bson:"uptime"`
// 	Optime               *OpTime             `bson:"optime"`
// 	OptimeDate           time.Time           `bson:"optimeDate"`
// 	OptimeDurableDate    time.Time           `bson:"optimeDurableDate"`
// 	LastAppliedWallTime  time.Time           `bson:"lastAppliedWallTime"`
// 	LastDurableWallTime  time.Time           `bson:"lastDurableWallTime"`
// 	LastHeartbeatMessage string              `bson:"lastHeartbeatMessage"`
// 	SyncSourceHost       string              `bson:"syncSourceHost"`
// 	SyncSourceID         int64               `bson:"syncSourceId"`
// 	InfoMessage          string              `bson:"infoMessage"`
// 	ElectionTime         primitive.Timestamp `bson:"electionTime"`
// 	ElectionDate         time.Time           `bson:"electionDate"`
// 	ConfigVersion        int64               `bson:"configVersion"`
// 	ConfigTerm           int64               `bson:"configTerm"`
// 	Self                 bool                `bson:"self"`
// }
//
// type ReplSetGetStatus struct {
// 	Set                     string          `bson:"set"`
// 	Date                    time.Time       `bson:"date"`
// 	MyState                 int64           `bson:"myState"`
// 	Term                    int64           `bson:"term"`
// 	HeartbeatIntervalMillis int64           `bson:"heartbeatIntervalMillis"`
// 	Members                 []ReplSetMember `bson:"members"`
// 	Ok                      int64           `bson:"ok"`
// }

// NewMongoDB returns a new MongoDB Binding
func NewMongoDB() *MongoDBOperations {
	logger := ctrl.Log.WithName("Mongo")
	return &MongoDBOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the MongoDB Binding.
func (mongoOps *MongoDBOperations) Init(properties component.Properties) error {
	mongoOps.Logger.Info("Initializing MongoDB binding")
	mongoOps.BaseOperations.Init(properties)
	config, _ := mongodb.NewConfig(mongoOps.Metadata)
	manager, _ := mongodb.NewManager(mongoOps.Logger)

	mongoOps.DBType = "mongodb"
	mongoOps.manager = manager
	// mongoOps.InitIfNeed = mongoOps.initIfNeed
	mongoOps.DBPort = config.GetDBPort()
	mongoOps.BaseOperations.GetRole = mongoOps.GetRole
	mongoOps.RegisterOperationOnDBReady(util.GetRoleOperation, mongoOps.GetRoleOps, manager)
	mongoOps.RegisterOperationOnDBReady(util.CheckRoleOperation, mongoOps.CheckRoleOps, manager)

	return nil
}

// func (mongoOps *MongoDBOperations) initIfNeed() bool {
// 	if mongoOps.database == nil {
// 		go func() {
// 			err := mongoOps.InitDelay()
// 			mongoOps.Logger.Errorf("MongoDB connection init failed: %v", err)
// 		}()
// 		return true
// 	}
// 	return false
// }
//
// func (mongoOps *MongoDBOperations) InitDelay() error {
// 	mongoOps.mu.Lock()
// 	defer mongoOps.mu.Unlock()
// 	if mongoOps.database != nil {
// 		return nil
// 	}
// 	mongoOps.operationTimeout = mongoOps.mongoDBMetadata.operationTimeout
//
// 	client, err := getMongoDBClient(&mongoOps.mongoDBMetadata)
// 	if err != nil {
// 		mongoOps.Logger.Errorf("error in creating MongoDB client: %s", err)
// 		return err
// 	}
//
// 	if err = client.Ping(context.Background(), nil); err != nil {
// 		_ = client.Disconnect(context.Background())
// 		mongoOps.Logger.Errorf("error in connecting to MongoDB, host: %s error: %s", mongoOps.mongoDBMetadata.host, err)
// 		return err
// 	}
//
// 	db := client.Database(adminDatabase)
// 	_, err = getReplSetStatus(context.Background(), db)
// 	if err != nil {
// 		_ = client.Disconnect(context.Background())
// 		mongoOps.Logger.Errorf("error in getting repl status from mongodb, error: %s", err)
// 		return err
// 	}
//
// 	mongoOps.client = client
// 	mongoOps.database = db
//
// 	return nil
// }

func (mongoOps *MongoDBOperations) GetRole(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (string, error) {
	return mongoOps.manager.GetMemberState(ctx)
}

func (mongoOps *MongoDBOperations) LockInstance(ctx context.Context) error {
	// TODO: impl
	return fmt.Errorf("NotSupported")
}

func (mongoOps *MongoDBOperations) InternalQuery(ctx context.Context, sql string) ([]byte, error) {
	// TODO: impl
	return nil, nil
}

func (mongoOps *MongoDBOperations) InternalExec(ctx context.Context, sql string) (int64, error) {
	// TODO: impl
	return 0, nil
}

func (mongoOps *MongoDBOperations) GetLogger() logr.Logger {
	return mongoOps.Logger
}

func (mongoOps *MongoDBOperations) GetRunningPort() int {
	// TODO: impl
	return mongoOps.DBPort
}
