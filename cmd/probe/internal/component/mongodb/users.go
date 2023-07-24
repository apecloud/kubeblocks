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

func CreateUser(ctx context.Context, client *mongo.Client, user, pwd string, roles ...map[string]interface{}) error {
	resp := OKResponse{}

	res := client.Database("admin").RunCommand(ctx, bson.D{
		{Key: "createUser", Value: user},
		{Key: "pwd", Value: pwd},
		{Key: "roles", Value: roles},
	})
	if res.Err() != nil {
		return errors.Wrap(res.Err(), "failed to create user")
	}

	err := res.Decode(&resp)
	if err != nil {
		return errors.Wrap(err, "failed to decode response")
	}

	if resp.OK != 1 {
		return errors.Errorf("mongo says: %s", resp.Errmsg)
	}

	return nil
}

func GetUser(ctx context.Context, client *mongo.Client, userName string) (*User, error) {
	resp := UsersInfo{}
	res := client.Database("admin").RunCommand(ctx, bson.D{{Key: "usersInfo", Value: userName}})
	if res.Err() != nil {
		return nil, errors.Wrap(res.Err(), "run command")
	}

	err := res.Decode(&resp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}
	if resp.OK != 1 {
		return nil, errors.Errorf("mongo says: %s", resp.Errmsg)
	}
	if len(resp.Users) == 0 {
		return nil, nil
	}
	return &resp.Users[0], nil
}

func UpdateUserRoles(ctx context.Context, client *mongo.Client, userName string, roles []map[string]interface{}) error {
	return client.Database("admin").RunCommand(ctx, bson.D{{Key: "updateUser", Value: userName}, {Key: "roles", Value: roles}}).Err()
}

// UpdateUserPass updates user's password
func UpdateUserPass(ctx context.Context, client *mongo.Client, name, pass string) error {
	return client.Database("admin").RunCommand(ctx, bson.D{{Key: "updateUser", Value: name}, {Key: "pwd", Value: pass}}).Err()
}

// DropUser delete user
func DropUser(ctx context.Context, client *mongo.Client, userName string) error {
	user, err := GetUser(ctx, client, userName)
	if err != nil {
		return errors.Wrap(err, "get user")
	}

	if user == nil {
		return errors.New(userName + " user not exists")
	}

	err = client.Database("admin").RunCommand(ctx, bson.D{{Key: "dropUser", Value: userName}}).Err()
	return errors.Wrap(err, "drop user")
}
