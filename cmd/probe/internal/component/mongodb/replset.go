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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetReplSetStatus(ctx context.Context, client *mongo.Client) (*ReplSetStatus, error) {
	status := &ReplSetStatus{}

	resp := client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}})
	if resp.Err() != nil {
		err := errors.Wrap(resp.Err(), "replSetGetStatus")
		return nil, err
	}

	if err := resp.Decode(status); err != nil {
		err := errors.Wrap(err, "failed to decode rs status")
		return nil, err
	}

	if status.OK != 1 {
		err := errors.Errorf("mongo says: %s", status.Errmsg)
		return nil, err
	}

	return status, nil
}

func SetReplSetConfig(ctx context.Context, rsClient *mongo.Client, cfg *RSConfig) error {
	resp := OKResponse{}

	res := rsClient.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetReconfig", Value: cfg}})
	if res.Err() != nil {
		err := errors.Wrap(res.Err(), "replSetReconfig")
		return err
	}

	if err := res.Decode(&resp); err != nil {
		err = errors.Wrap(err, "failed to decode to replSetReconfigResponse")
		return err
	}

	if resp.OK != 1 {
		err := errors.Errorf("mongo says: %s", resp.Errmsg)
		return err
	}

	return nil
}

func GetReplSetConfig(ctx context.Context, client *mongo.Client) (*RSConfig, error) {
	resp := ReplSetGetConfig{}
	res := client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetConfig", Value: 1}})
	if res.Err() != nil {
		err := errors.Wrap(res.Err(), "replSetGetConfig")
		return nil, err
	}
	if err := res.Decode(&resp); err != nil {
		err := errors.Wrap(err, "failed to decode to replSetGetConfig")
		return nil, err
	}

	if resp.Config == nil {
		err := errors.Errorf("mongo says: %s", resp.Errmsg)
		return nil, err
	}

	return resp.Config, nil
}
