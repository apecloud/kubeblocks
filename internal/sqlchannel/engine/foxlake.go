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
	"strings"
)

type foxlake struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newFoxLake() *foxlake {
	return &foxlake{
		info: EngineInfo{
			Client:      "usql",
			Container:   "foxlake",
			UserEnv:     "$FOXLAKE_ROOT_USER",
			PasswordEnv: "$FOXLAKE_ROOT_PASSWORD",
		},
		examples: map[ClientType]buildConnectExample{},
	}
}

func (r *foxlake) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := r.info.UserEnv
	userPass := r.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	foxlakeCmd := []string{fmt.Sprintf("%s mysql://%s:%s@:${serverPort}", r.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(foxlakeCmd, " ")}
}

func (r *foxlake) Container() string {
	return r.info.Container
}

func (r *foxlake) ConnectExample(info *ConnectionInfo, client string) string {
	// TODO implement me
	panic("implement me")
}
