/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package engine

import (
	"fmt"
	"sort"
	"strings"
)

const (
	stateMysql      = "mysql"
	statePostgreSQL = "postgresql"
	stateRedis      = "redis"
	stateMongoDB    = "mongodb"
)

// AuthInfo is the authentication information for the database
type AuthInfo struct {
	UserName   string
	UserPasswd string
}

type Interface interface {
	ConnectCommand(info *AuthInfo) []string
	Container() string
	ConnectExample(info *ConnectionInfo, client string) string
}

type EngineInfo struct {
	Client      string
	Container   string
	PasswordEnv string
	UserEnv     string
	Database    string
}

func New(typeName string) (Interface, error) {
	switch typeName {
	case stateMysql:
		return newMySQL(), nil
	case statePostgreSQL:
		return newPostgreSQL(), nil
	case stateRedis:
		return newRedis(), nil
	case stateMongoDB:
		return newMongoDB(), nil
	default:
		return nil, fmt.Errorf("unsupported engine type: %s", typeName)
	}
}

type ConnectionInfo struct {
	Host     string
	User     string
	Password string
	Database string
	Port     string
}

type buildConnectExample func(info *ConnectionInfo) string

func buildExample(info *ConnectionInfo, client string, examples map[ClientType]buildConnectExample) string {
	// if client is not specified, output all examples
	if len(client) == 0 {
		var keys = make([]string, len(examples))
		var i = 0
		for k := range examples {
			keys[i] = k.String()
			i++
		}
		sort.Strings(keys)

		var b strings.Builder
		for _, k := range keys {
			buildFn := examples[ClientType(k)]
			b.WriteString(fmt.Sprintf("========= %s connection example =========\n", k))
			b.WriteString(buildFn(info))
			b.WriteString("\n")
		}
		return b.String()
	}

	// return specified example
	if buildFn, ok := examples[ClientType(client)]; ok {
		return buildFn(info)
	}

	return ""
}
