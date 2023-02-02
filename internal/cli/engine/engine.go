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

import "fmt"

// ClusterDefinition Type Const Define
const (
	stateMysql  = "state.mysql"
	stateMysql8 = "state.mysql"
)

type Interface interface {
	ConnectCommand() []string
	EngineName() string
	EngineContainer() string
	ConnectExample(info *ConnectionInfo, client string) string
}

type ConnectionInfo struct {
	Host     string
	User     string
	Password string
	Database string
	Port     string
}

type buildConnectExample func(info *ConnectionInfo) string

func New(typeName string) (Interface, error) {
	switch typeName {
	case stateMysql:
		return &mysql{}, nil
	default:
		return nil, fmt.Errorf("unsupported engine type: %s", typeName)
	}
}
