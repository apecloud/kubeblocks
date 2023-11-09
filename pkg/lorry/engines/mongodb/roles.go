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

func CreateRole(ctx context.Context, client *mongo.Client, role string, privileges []RolePrivilege, roles []interface{}) error {
	resp := OKResponse{}

	privilegesArr := bson.A{}
	for _, p := range privileges {
		privilegesArr = append(privilegesArr, p)
	}

	rolesArr := bson.A{}
	for _, r := range roles {
		rolesArr = append(rolesArr, r)
	}

	m := bson.D{
		{Key: "createRole", Value: role},
		{Key: "privileges", Value: privilegesArr},
		{Key: "roles", Value: rolesArr},
	}

	res := client.Database("admin").RunCommand(ctx, m)
	if res.Err() != nil {
		return errors.Wrap(res.Err(), "failed to create role")
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

func UpdateRole(ctx context.Context, client *mongo.Client, role string, privileges []RolePrivilege, roles []interface{}) error {
	resp := OKResponse{}

	privilegesArr := bson.A{}
	for _, p := range privileges {
		privilegesArr = append(privilegesArr, p)
	}

	rolesArr := bson.A{}
	for _, r := range roles {
		rolesArr = append(rolesArr, r)
	}

	m := bson.D{
		{Key: "updateRole", Value: role},
		{Key: "privileges", Value: privilegesArr},
		{Key: "roles", Value: rolesArr},
	}

	res := client.Database("admin").RunCommand(ctx, m)
	if res.Err() != nil {
		return errors.Wrap(res.Err(), "failed to create role")
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

func GetRole(ctx context.Context, client *mongo.Client, role string) (*Role, error) {
	resp := RoleInfo{}

	res := client.Database("admin").RunCommand(ctx, bson.D{
		{Key: "rolesInfo", Value: role},
		{Key: "showPrivileges", Value: true},
	})
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
	if len(resp.Roles) == 0 {
		return nil, nil
	}
	return &resp.Roles[0], nil
}
