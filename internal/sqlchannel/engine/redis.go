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

import "strings"

type redis struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newRedis() *redis {
	return &redis{
		info: EngineInfo{
			Client:    "redis-cli",
			Container: "redis",
		},
		examples: map[ClientType]buildConnectExample{},
	}
}

func (r redis) ConnectCommand(connectInfo *AuthInfo) []string {
	redisCmd := []string{
		"redis-cli",
	}

	if connectInfo != nil {
		redisCmd = append(redisCmd, "--user", connectInfo.UserName)
		redisCmd = append(redisCmd, "--pass", connectInfo.UserPasswd)
	}
	return []string{"sh", "-c", strings.Join(redisCmd, " ")}
}

func (r redis) Container() string {
	return r.info.Container
}

func (r redis) ConnectExample(info *ConnectionInfo, client string) string {
	// TODO implement me
	panic("implement me")
}

var _ Interface = &redis{}
