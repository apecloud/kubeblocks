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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

// MongoDBOperations is a binding implementation for MongoDB.
type MongoDBOperations struct {
	mongoDBMetadata
	mu               sync.Mutex
	client           *mongo.Client
	database         *mongo.Database
	operationTimeout time.Duration
	BaseOperations
}

type mongoDBMetadata struct {
	host             string
	username         string
	password         string
	server           string
	databaseName     string
	params           string
	operationTimeout time.Duration
}

type OpTime struct {
	TS primitive.Timestamp `bson:"ts"`
	T  int64               `bson:"t"`
}

type ReplSetMember struct {
	ID                   int64               `bson:"_id"`
	Name                 string              `bson:"name"`
	Health               int64               `bson:"health"`
	State                int64               `bson:"state"`
	StateStr             string              `bson:"stateStr"`
	Uptime               int64               `bson:"uptime"`
	Optime               *OpTime             `bson:"optime"`
	OptimeDate           time.Time           `bson:"optimeDate"`
	OptimeDurableDate    time.Time           `bson:"optimeDurableDate"`
	LastAppliedWallTime  time.Time           `bson:"lastAppliedWallTime"`
	LastDurableWallTime  time.Time           `bson:"lastDurableWallTime"`
	LastHeartbeatMessage string              `bson:"lastHeartbeatMessage"`
	SyncSourceHost       string              `bson:"syncSourceHost"`
	SyncSourceID         int64               `bson:"syncSourceId"`
	InfoMessage          string              `bson:"infoMessage"`
	ElectionTime         primitive.Timestamp `bson:"electionTime"`
	ElectionDate         time.Time           `bson:"electionDate"`
	ConfigVersion        int64               `bson:"configVersion"`
	ConfigTerm           int64               `bson:"configTerm"`
	Self                 bool                `bson:"self"`
}

type ReplSetGetStatus struct {
	Set                     string          `bson:"set"`
	Date                    time.Time       `bson:"date"`
	MyState                 int64           `bson:"myState"`
	Term                    int64           `bson:"term"`
	HeartbeatIntervalMillis int64           `bson:"heartbeatIntervalMillis"`
	Members                 []ReplSetMember `bson:"members"`
	Ok                      int64           `bson:"ok"`
}

const (
	host             = "host"
	username         = "username"
	password         = "password"
	server           = "server"
	databaseName     = "databaseName"
	operationTimeout = "operationTimeout"
	params           = "params"
	value            = "value"

	defaultTimeout = 5 * time.Second

	defaultDBPort = 27017

	// mongodb://<username>:<password@<host>/<database><params>
	connectionURIFormatWithAuthentication = "mongodb://%s:%s@%s/%s%s"

	// mongodb://<host>/<database><params>
	connectionURIFormat = "mongodb://%s/%s%s"

	// mongodb+srv://<server>/<params>
	connectionURIFormatWithSrv = "mongodb+srv://%s/%s"

	// mongodb+srv://<username>:<password>@<server>/<database><params>
	connectionURIFormatWithSrvAndCredentials = "mongodb+srv://%s:%s@%s/%s%s" //nolint:gosec

	adminDatabase = "admin"
)

// NewMongoDB returns a new MongoDB Binding
func NewMongoDB(logger logger.Logger) bindings.OutputBinding {
	return &MongoDBOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the MongoDB Binding.
func (mongoOps *MongoDBOperations) Init(metadata bindings.Metadata) error {
	mongoOps.Logger.Debug("Initializing MongoDB binding")
	mongoOps.BaseOperations.Init(metadata)
	meta, err := getMongoDBMetaData(metadata)
	if err != nil {
		return err
	}
	mongoOps.mongoDBMetadata = *meta

	mongoOps.DBType = "mongodb"
	mongoOps.InitIfNeed = mongoOps.initIfNeed
	mongoOps.DBPort = mongoOps.GetRunningPort()
	mongoOps.BaseOperations.GetRole = mongoOps.GetRole
	mongoOps.OperationMap[GetRoleOperation] = mongoOps.GetRoleOps
	return nil
}

func (mongoOps *MongoDBOperations) Ping() error {
	if err := mongoOps.client.Ping(context.Background(), nil); err != nil {
		return fmt.Errorf("mongoDB binding: error connecting to mongoDB at %s: %s", mongoOps.mongoDBMetadata.host, err)
	}
	return nil
}

func (mongoOps *MongoDBOperations) initIfNeed() bool {
	if mongoOps.database == nil {
		go func() {
			err := mongoOps.InitDelay()
			mongoOps.Logger.Errorf("MongoDB connection init failed: %v", err)
		}()
		return true
	}
	return false
}

func (mongoOps *MongoDBOperations) InitDelay() error {
	mongoOps.mu.Lock()
	defer mongoOps.mu.Unlock()
	if mongoOps.database != nil {
		return nil
	}
	mongoOps.operationTimeout = mongoOps.mongoDBMetadata.operationTimeout

	client, err := getMongoDBClient(&mongoOps.mongoDBMetadata)
	if err != nil {
		mongoOps.Logger.Errorf("error in creating mongodb client: %s", err)
		return err
	}

	if err = client.Ping(context.Background(), nil); err != nil {
		_ = client.Disconnect(context.Background())
		mongoOps.Logger.Errorf("error in connecting to mongodb, host: %s error: %s", mongoOps.mongoDBMetadata.host, err)
		return err
	}

	db := client.Database(adminDatabase)
	_, err = getReplSetStatus(context.Background(), db)
	if err != nil {
		_ = client.Disconnect(context.Background())
		mongoOps.Logger.Errorf("error in getting repl status from mongodb, error: %s", err)
		return err
	}

	mongoOps.client = client
	mongoOps.database = db

	return nil
}

func (mongoOps *MongoDBOperations) GetRunningPort() int {
	if viper.IsSet("KB_SERVICE_PORT") {
		return viper.GetInt("KB_SERVICE_PORT")
	}

	uri := getMongoURI(&mongoOps.mongoDBMetadata)
	index := strings.Index(uri, "://")
	if index < 0 {
		return defaultDBPort
	}
	uri = uri[index+len("://"):]
	index = strings.Index(uri, "/")
	if index < 0 {
		return defaultDBPort
	}
	uri = uri[:index]
	index = strings.Index(uri, "@")
	if index < 0 {
		return defaultDBPort
	}
	uri = uri[index:]
	index = strings.Index(uri, ":")
	if index < 0 {
		return defaultDBPort
	}
	portStr := uri[index+1:]
	if viper.IsSet("KB_SERVICE_PORT") {
		portStr = viper.GetString("KB_SERVICE_PORT")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultDBPort
	}

	return port
}

func (mongoOps *MongoDBOperations) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	status, err := getReplSetStatus(ctx, mongoOps.database)
	if err != nil {
		mongoOps.Logger.Errorf("rs.status() error: %", err)
		return "", err
	}
	for _, member := range status.Members {
		if member.State == status.MyState {
			return strings.ToLower(member.StateStr), nil
		}
	}
	return "", errors.New("role not found")
}

func (mongoOps *MongoDBOperations) GetRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	role, err := mongoOps.GetRole(ctx, req, resp)
	if err != nil {
		return nil, err
	}
	opsRes := OpsResult{}
	opsRes["role"] = role
	return opsRes, nil
}

func (mongoOps *MongoDBOperations) StatusCheck(ctx context.Context, cmd string, response *bindings.InvokeResponse) (OpsResult, error) {
	// TODO implement me when proposal is passed
	// proposal: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
	return nil, nil
}

func getMongoURI(metadata *mongoDBMetadata) string {
	if len(metadata.server) != 0 {
		if metadata.username != "" && metadata.password != "" {
			return fmt.Sprintf(connectionURIFormatWithSrvAndCredentials, metadata.username, metadata.password, metadata.server, metadata.databaseName, metadata.params)
		}

		return fmt.Sprintf(connectionURIFormatWithSrv, metadata.server, metadata.params)
	}

	if metadata.username != "" && metadata.password != "" {
		return fmt.Sprintf(connectionURIFormatWithAuthentication, metadata.username, metadata.password, metadata.host, metadata.databaseName, metadata.params)
	}

	return fmt.Sprintf(connectionURIFormat, metadata.host, metadata.databaseName, metadata.params)
}

func getMongoDBClient(metadata *mongoDBMetadata) (*mongo.Client, error) {
	uri := getMongoURI(metadata)

	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), metadata.operationTimeout)
	defer cancel()

	daprUserAgent := "dapr-" + logger.DaprVersion
	if clientOptions.AppName != nil {
		clientOptions.SetAppName(daprUserAgent + ":" + *clientOptions.AppName)
	} else {
		clientOptions.SetAppName(daprUserAgent)
	}

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func getMongoDBMetaData(metadata bindings.Metadata) (*mongoDBMetadata, error) {
	meta := mongoDBMetadata{
		operationTimeout: defaultTimeout,
	}

	if val, ok := metadata.Properties[host]; ok && val != "" {
		meta.host = val
	}

	if viper.IsSet("KB_SERVICE_PORT") {
		meta.host = "localhost:" + viper.GetString("KB_SERVICE_PORT")
	}

	if val, ok := metadata.Properties[server]; ok && val != "" {
		meta.server = val
	}

	if len(meta.host) == 0 && len(meta.server) == 0 {
		return nil, errors.New("must set 'host' or 'server' fields in metadata")
	}

	if len(meta.host) != 0 && len(meta.server) != 0 {
		return nil, errors.New("'host' or 'server' fields are mutually exclusive")
	}

	if val, ok := metadata.Properties[username]; ok && val != "" {
		meta.username = val
	}

	if val, ok := metadata.Properties[password]; ok && val != "" {
		meta.password = val
	}

	if viper.IsSet("KB_SERVICE_USER") {
		meta.username = viper.GetString("KB_SERVICE_USER")
	}

	if viper.IsSet("KB_SERVICE_PASSWORD") {
		meta.password = viper.GetString("KB_SERVICE_PASSWORD")
	}

	meta.databaseName = adminDatabase
	if val, ok := metadata.Properties[databaseName]; ok && val != "" {
		meta.databaseName = val
	}

	if val, ok := metadata.Properties[params]; ok && val != "" {
		meta.params = val
	}

	var err error
	if val, ok := metadata.Properties[operationTimeout]; ok && val != "" {
		meta.operationTimeout, err = time.ParseDuration(val)
		if err != nil {
			return nil, errors.New("incorrect operationTimeout field from metadata")
		}
	}

	return &meta, nil
}

func getCommand(ctx context.Context, db *mongo.Database, command bson.M) (bson.M, error) {
	var result bson.M
	err := db.RunCommand(ctx, command).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func getReplSetStatus(ctx context.Context, admin *mongo.Database) (*ReplSetGetStatus, error) {
	var result bson.M
	command := bson.M{"replSetGetStatus": 1}
	result, err := getCommand(ctx, admin, command)
	if err != nil {
		return nil, err
	}

	var r ReplSetGetStatus
	bsonBytes, _ := bson.Marshal(result)
	err = bson.Unmarshal(bsonBytes, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
