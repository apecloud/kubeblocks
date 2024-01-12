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

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func NewMongodbClient(ctx context.Context, config *Config) (*mongo.Client, error) {
	if len(config.Hosts) == 0 {
		return nil, errors.New("Get replset client without hosts")
	}

	opts := options.Client().
		SetHosts(config.Hosts).
		SetReplicaSet(config.ReplSetName).
		SetAuth(options.Credential{
			Password: config.Password,
			Username: config.Username,
		}).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority(), writeconcern.J(true))).
		SetReadPreference(readpref.Primary()).
		SetDirect(config.Direct)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "connect to mongodb")
	}
	return client, nil
}

func NewReplSetClient(ctx context.Context, hosts []string) (*mongo.Client, error) {
	config := GetConfig().DeepCopy()
	config.Hosts = hosts
	config.Direct = false
	return NewMongodbClient(ctx, config)

}

func NewMongosClient(ctx context.Context, hosts []string) (*mongo.Client, error) {
	config := GetConfig().DeepCopy()
	config.Hosts = hosts
	config.Direct = false
	config.ReplSetName = ""

	return NewMongodbClient(ctx, config)
}

func NewStandaloneClient(ctx context.Context, host string) (*mongo.Client, error) {
	config := GetConfig().DeepCopy()
	config.Hosts = []string{host}
	config.Direct = true
	config.ReplSetName = ""

	return NewMongodbClient(ctx, config)
}

func NewLocalUnauthClient(ctx context.Context) (*mongo.Client, error) {
	config := GetConfig().DeepCopy()
	config.Direct = true
	config.ReplSetName = ""

	opts := options.Client().
		SetHosts(config.Hosts).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority(), writeconcern.J(true))).
		SetReadPreference(readpref.Primary()).
		SetDirect(config.Direct)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "connect to mongodb")
	}

	return client, nil
}
