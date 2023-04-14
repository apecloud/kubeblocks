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

type mongodb struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newMongoDB() *mongodb {
	return &mongodb{
		info: EngineInfo{
			Client:      "mongosh",
			Container:   "mongodb",
			UserEnv:     "$MONGODB_ROOT_USER",
			PasswordEnv: "$MONGODB_ROOT_PASSWORD",
		},
		examples: map[ClientType]buildConnectExample{},
	}
}

func (r mongodb) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := r.info.UserEnv
	userPass := r.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	mongodbCmd := []string{fmt.Sprintf("%s -u %s -p %s", r.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(mongodbCmd, " ")}
}

func (r mongodb) Container() string {
	return r.info.Container
}

func (r mongodb) ConnectExample(info *ConnectionInfo, client string) string {
	// TODO implement me
	panic("implement me")
}

var _ Interface = &mongodb{}
