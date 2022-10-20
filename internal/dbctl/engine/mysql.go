/*
Copyright 2022 The KubeBlocks Authors

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

type mysql struct{}

var (
	// key is the dbctl command name, value is the command to execute
	commands = map[string]*ExecInfo{
		"connect": {
			Command: []string{"mysql"},
			// use the default container
			ContainerName: "",
		},
	}
)

const (
	mysqlEngineName = "mysql"
)

var _ Interface = &mysql{}

func (m *mysql) GetExecCommand(name string) *ExecInfo {
	if cmd, ok := commands[name]; ok {
		return cmd
	} else {
		return nil
	}
}

func (m *mysql) GetEngineName() string {
	return mysqlEngineName
}
