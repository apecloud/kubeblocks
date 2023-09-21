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

package engine

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type pulsar struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

var _ ClusterCommands = &pulsar{}

func newPulsar(containName string) *pulsar {
	return &pulsar{
		info: EngineInfo{
			Client:    "pulsar-shell",
			Container: containName,
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return "# pulsar client connection example\n bin/pulsar-shell"
			},
		},
	}
}

func (r *pulsar) ConnectCommand(connectInfo *AuthInfo) []string {
	return []string{"sh", "-c", "bin/pulsar-shell"}
}

func (r *pulsar) Container() string {
	return r.info.Container
}

func (r *pulsar) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, r.examples)
}

func (r *pulsar) ExecuteCommand([]string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("%s not implemented", r.info.Client)
}
