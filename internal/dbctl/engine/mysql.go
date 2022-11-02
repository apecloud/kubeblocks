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

package engine

import "fmt"

// ClusterDefinition Type Const Define
const (
	stateMysql         = "state.mysql"
	stateMysql8        = "state.mysql-8"
	connectModule      = "connect"
	mysqlEngineName    = "mysql"
	mysqlClient        = "mysql"
	mysqlContainerName = "mysql"
	logsModule         = "logs"
)

type Interface interface {
	ConnectCommand(database string) []string
	EngineName() string
	EngineContainer() string
}

// Interface implementation of mysql connect
type mysql struct{}

func (m *mysql) ConnectCommand(database string) []string {
	if len(database) > 0 {
		return []string{mysqlClient, "-D", database}
	}
	return []string{mysqlClient}
}

func (m *mysql) EngineName() string {
	return mysqlEngineName
}

func (m *mysql) EngineContainer() string {
	return mysqlContainerName
}

func New(typeName string) (Interface, error) {
	if v, err := GetContext(typeName, connectModule); err == nil {
		if iv, ok := v.(Interface); ok {
			return iv, nil
		}
	}
	return nil, fmt.Errorf("unsupported engine type: %s", typeName)
}

func init() {
	// todo a more high level abstraction will continue and automatically registered by yaml-conf may be more better in the future.
	// for well-known database systems, how to connect or what logs they is a common sense, which maybe not require ISV configuration.

	// register connect context for mysql and mysql8 engine
	var m = &mysql{}
	Registry(stateMysql, connectModule, m)
	Registry(stateMysql8, connectModule, m)
	// register log context for mysql
	registryMySQLLogsContext()
}

func registryMySQLLogsContext() {
	mysqlLogsContext := map[string]LogVariables{
		"stdout": {
			DefaultFilePath: "stdout/stderr",
			Variables:       nil,
			PathSQL:         "",
			Describe:        "the stdout or stderr of container.",
		},
		"error": {
			DefaultFilePath: "",
			Variables:       []string{"log_error"},
			PathSQL:         "mysql -N -e \"select VARIABLE_VALUE from performance_schema.global_variables where VARIABLE_NAME = 'log_error'\"",
			Describe:        "the error log of mysql, and variable log_error indicate the log file path.",
		},
		"slow": {
			DefaultFilePath: "",
			Variables:       []string{"slow_query_log_file", "slow_query_log", "long_query_time", "log_output"},
			PathSQL:         "mysql -N -e \"select VARIABLE_VALUE from performance_schema.global_variables where VARIABLE_NAME = 'slow_query_log_file'\"",
			Describe:        "the slow log of mysql, and variable slow_query_log_file indicate the log file path.",
		},
	}
	Registry(stateMysql, logsModule, mysqlLogsContext)
	Registry(stateMysql8, logsModule, mysqlLogsContext)
}

type LogVariables struct {
	// PathSQL indicate the SQL of log file path
	PathSQL string
	// Variables engine variables of this specify log type
	Variables []string
	// DefaultFilePath indicate the default path for extensible file, such as dmesg, and have a more premium priority than PathVar.
	DefaultFilePath string
	// Describe info
	Describe string
}

func LogsContext(engine string) (map[string]LogVariables, error) {
	if v, err := GetContext(engine, logsModule); err == nil {
		if iv, ok := v.(map[string]LogVariables); ok {
			return iv, nil
		}
	}
	return nil, fmt.Errorf("no log context for engine %s", engine)
}
