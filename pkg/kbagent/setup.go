/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package util

import (
	"encoding/json"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	ContainerName     = "kbagent"
	InitContainerName = "init-kbagent"
	DefaultPortName   = "http"

	actionEnvName = "KB_AGENT_ACTION"
	probeEnvName  = "KB_AGENT_PROBE"
)

func BuildStartupEnv(actions []proto.Action, probes []proto.Probe) ([]corev1.EnvVar, error) {
	da, dp, err := serializeActionNProbe(actions, probes)
	if err != nil {
		return nil, err
	}
	return append(util.DefaultEnvVars(), []corev1.EnvVar{
		{
			Name:  actionEnvName,
			Value: da,
		},
		{
			Name:  probeEnvName,
			Value: dp,
		},
	}...), nil
}

func Initialize(logger logr.Logger, envs []string) ([]service.Service, error) {
	da, dp := getActionNProbeEnvValue(envs)
	if len(da) == 0 {
		return nil, nil
	}

	actions, probes, err := deserializeActionNProbe(da, dp)
	if err != nil {
		return nil, err
	}

	return service.New(logger, actions, probes)
}

func getActionNProbeEnvValue(envs []string) (string, string) {
	envVars := util.EnvL2M(envs)
	da, ok := envVars[actionEnvName]
	if !ok {
		return "", ""
	}
	dp, ok := envVars[probeEnvName]
	if !ok {
		return da, ""
	}
	return da, dp
}

func serializeActionNProbe(actions []proto.Action, probes []proto.Probe) (string, string, error) {
	da, err := json.Marshal(actions)
	if err != nil {
		return "", "", nil
	}
	dp, err := json.Marshal(probes)
	if err != nil {
		return "", "", nil
	}
	return string(da), string(dp), nil
}

func deserializeActionNProbe(da, dp string) ([]proto.Action, []proto.Probe, error) {
	actions := make([]proto.Action, 0)
	if err := json.Unmarshal([]byte(da), &actions); err != nil {
		return nil, nil, err
	}
	probes := make([]proto.Probe, 0)
	if err := json.Unmarshal([]byte(dp), &probes); err != nil {
		return nil, nil, err
	}
	return actions, probes, nil
}
