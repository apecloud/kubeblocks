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
