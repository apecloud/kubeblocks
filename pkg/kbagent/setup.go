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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	containerName = "kbagent"
	commandName   = "/bin/kbagent"
	actionEnvName = "KB_AGENT_ACTION"
)

func BuildSidecarContainer(actions *appsv1alpha1.ComponentLifecycleActions, port int32) (*corev1.Container, error) {
	actionEnv, err := buildActionEnv(actions)
	if err != nil {
		return nil, err
	}
	return builder.NewContainerBuilder(containerName).
		SetImage("apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/pause:3.6").
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands(commandName).
		AddEnv(*actionEnv).
		AddPorts(corev1.ContainerPort{
			Name:          "http",
			ContainerPort: port,
			Protocol:      "TCP",
		}).
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(int(port))},
			}}).
		GetObject(), nil
}

func Initialize(envVars map[string]string) (any, error) {
	data := getActionEnvValue(envVars)
	if len(data) == 0 {
		return nil, nil
	}

	actions, err := deserializeAction(data)
	if err != nil {
		return nil, err
	}

	return service.NewService(actions), nil
}

func buildActionEnv(actions *appsv1alpha1.ComponentLifecycleActions) (*corev1.EnvVar, error) {
	value, err := serializeAction(actions)
	if err != nil {
		return nil, err
	}
	return &corev1.EnvVar{
		Name:  actionEnvName,
		Value: value,
	}, nil
}

func getActionEnvValue(envVars map[string]string) string {
	value, ok := envVars[actionEnvName]
	if !ok {
		return ""
	}
	return value
}

func serializeAction(actions *appsv1alpha1.ComponentLifecycleActions) (string, error) {
	data, err := json.Marshal(actions)
	if err != nil {
		return "", nil
	}
	return string(data), nil
}

func deserializeAction(value string) (*appsv1alpha1.ComponentLifecycleActions, error) {
	actions := &appsv1alpha1.ComponentLifecycleActions{}
	if err := json.Unmarshal([]byte(value), actions); err != nil {
		return nil, err
	}
	return actions, nil
}
