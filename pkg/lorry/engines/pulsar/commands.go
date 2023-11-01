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

package pulsar

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

type Commands struct {
	info     engines.EngineInfo
	examples map[models.ClientType]engines.BuildConnectExample
}

var _ engines.ClusterCommands = &Commands{}

func NewBrokerCommands() engines.ClusterCommands {
	return NewCommands("broker")
}

func NewProxyCommands() engines.ClusterCommands {
	return NewCommands("proxy")
}

func NewCommands(containName string) engines.ClusterCommands {
	return &Commands{
		info: engines.EngineInfo{
			Client:    "pulsar-shell",
			Container: containName,
		},
		examples: map[models.ClientType]engines.BuildConnectExample{
			models.CLI: func(info *engines.ConnectionInfo) string {
				return "# pulsar client connection example\n bin/pulsar-shell"
			},
		},
	}
}

func (r *Commands) ConnectCommand(connectInfo *engines.AuthInfo) []string {
	return []string{"sh", "-c", "bin/pulsar-shell"}
}

func (r *Commands) Container() string {
	return r.info.Container
}

func (r *Commands) ConnectExample(info *engines.ConnectionInfo, client string) string {
	return engines.BuildExample(info, client, r.examples)
}

func (r *Commands) ExecuteCommand([]string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("%s not implemented", r.info.Client)
}
