/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package plugin

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DBClient struct {
	dbPlugin DBPluginClient
}

func NewPluginClient(addr string) (*DBClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrap(err, "grpc: failed to dial")
	}

	client := NewDBPluginClient(conn)

	dbClient := &DBClient{
		dbPlugin: client,
	}
	return dbClient, nil
}

func (c *DBClient) IsDBStartupReady(ctx context.Context) bool {
	req := &IsDBReadyRequest{}
	resp, err := c.dbPlugin.IsDBReady(ctx, req)
	if err != nil {
		return false
	}
	return resp.Ready
}

func (c *DBClient) GetReplicaRole(ctx context.Context) (string, error) {
	getRoleRequest := &GetRoleRequest{
		DbInfo: GetDBInfo(),
	}

	resp, err := c.dbPlugin.GetRole(ctx, getRoleRequest)
	if err != nil {
		return "", err
	}

	return resp.Role, nil
}
