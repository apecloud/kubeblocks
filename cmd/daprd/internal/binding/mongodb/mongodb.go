/*
Copyright ApeCloud Inc.

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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

	// mongodb://<username>:<password@<host>/<database><params>
	connectionURIFormatWithAuthentication = "mongodb://%s:%s@%s/%s%s"

	// mongodb://<host>/<database><params>
	connectionURIFormat = "mongodb://%s/%s%s"

	// mongodb+srv://<server>/<database><params>
	connectionURIFormatWithSrv = "mongodb+srv://%s/%s%s"

	// mongodb+srv://<username>:<password>@<server>/<database><params>
	connectionURIFormatWithSrvAndCredentials = "mongodb+srv://%s:%s@%s/%s%s" //nolint:gosec

	adminDatabase = "admin"

	queryOperation bindings.OperationKind = "query"
)

var oriRole = ""

// MongoDB is a binding implementation for MongoDB.
type MongoDB struct {
	client           *mongo.Client
	database         *mongo.Database
	operationTimeout time.Duration
	metadata         mongoDBMetadata

	logger logger.Logger
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

// NewMongoDB returns a new MongoDB Binding
func NewMongoDB(logger logger.Logger) bindings.OutputBinding {
	s := &MongoDB{logger: logger}

	return s
}

// Init initializes the MySQL Binding.
func (m *MongoDB) Init(metadata bindings.Metadata) error {
	m.logger.Debug("Initializing MongoDB binding")
	meta, err := getMongoDBMetaData(metadata)
	if err != nil {
		return err
	}
	m.metadata = *meta

	return nil
}

// InitIfNeed do the real init
func (m *MongoDB) InitIfNeed() error {
	if m.database != nil {
		return nil
	}

	m.operationTimeout = m.metadata.operationTimeout

	client, err := getMongoDBClient(&m.metadata)
	if err != nil {
		return fmt.Errorf("error in creating mongodb client: %s", err)
	}

	if err = client.Ping(context.Background(), nil); err != nil {
		return fmt.Errorf("error in connecting to mongodb, host: %s error: %s", m.metadata.host, err)
	}

	db := client.Database(adminDatabase)
	_, err = getReplSetStatus(context.Background(), db)
	if err != nil {
		return fmt.Errorf("error in getting repl status from mongodb, error: %s", err)
	}

	m.client = client
	m.database = db

	return nil
}

func (m *MongoDB) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{queryOperation}
}

func (m *MongoDB) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	resp := &bindings.InvokeResponse{}

	if req == nil {
		return nil, errors.New("invoke request required")
	}

	if err := m.InitIfNeed(); err != nil {
		resp.Data = []byte("db not ready")
		return resp, nil
	}

	data, err := m.roleCheck(ctx)
	if err != nil {
		return nil, err
	}
	resp.Data = data

	return resp, nil

}

func (m *MongoDB) Close() (err error) {
	return m.client.Disconnect(context.Background())
}

func (m *MongoDB) Ping() error {
	if err := m.client.Ping(context.Background(), nil); err != nil {
		return fmt.Errorf("mongoDB store: error connecting to mongoDB at %s: %s", m.metadata.host, err)
	}

	return nil
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

func (m *MongoDB) roleCheck(ctx context.Context) ([]byte, error) {
	status, err := getReplSetStatus(ctx, m.database)
	if err != nil {
		m.logger.Errorf("rs.status() error: %", err)
	}
	role := ""
	for _, member := range status.Members {
		if member.State == status.MyState {
			role = strings.ToLower(member.StateStr)
		}
	}
	if oriRole != role {
		result := map[string]string{}
		result["event"] = "roleChanged"
		result["originalRole"] = oriRole
		result["role"] = role
		msg, _ := json.Marshal(result)
		m.logger.Infof(string(msg))
		oriRole = role
		return nil, errors.New(string(msg))
	}

	return []byte(oriRole), nil
}

type OpTime struct {
	TS primitive.Timestamp `bson:"ts"`
	T  int64               `bson:"t"`
}

type ReplSetMember struct {
	ID                   int64  `bson:"_id"`
	Name                 string `bson:"name"`
	Health               int64  `bson:"health"`
	State                int64  `bson:"state"`
	StateStr             string `bson:"stateStr"`
	Uptime               int64  `bson:"uptime"`
	Optime               *OpTime
	OptimeDate           time.Time           `bson:"optimeDate"`
	OptimeDurableDate    time.Time           `bson:"optimeDurableDate"`
	LastAppliedWallTime  time.Time           `bson:"lastAppliedWallTime"`
	LastDurableWallTime  time.Time           `bson:"lastDurableWallTime"`
	LastHeartbeatMessage string              `bson:"lastHeartbeatMessage"`
	SyncSourceHost       string              `bson:"syncSourceHost"`
	SyncSourceId         int64               `bson:"syncSourceId"`
	InfoMessage          string              `bson:"infoMessage"`
	ElectionTime         primitive.Timestamp `bson:"electionTime"`
	ElectionDate         time.Time           `bson:"electionDate"`
	ConfigVersion        int64               `bson:"configVersion"`
	ConfigTerm           int64               `bson:"configTerm"`
	Self                 bool                `bson:"self"`
}

type ReplSetGetStatus struct {
	Set                     string    `bson:"set"`
	Date                    time.Time `bson:"date"`
	MyState                 int64     `bson:"myState"`
	Term                    int64     `bson:"term"`
	HeartbeatIntervalMillis int64     `bson:"heartbeatIntervalMillis"`

	Members []ReplSetMember `bson:"members"`
	Ok      int64           `bson:"ok"`
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
