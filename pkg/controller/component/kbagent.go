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

package component

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	kbAgentContainerName = "kbagent"
	kbAgentCommand       = "/bin/kbagent"
	kbAgentPortName      = "http"
)

func buildKBAgentContainer(synthesizedComp *SynthesizedComponent) error {
	envVars, err := buildKBAgentStartupEnv(synthesizedComp)
	if err != nil {
		return err
	}

	port := 3501 // TODO: port

	container := builder.NewContainerBuilder(kbAgentContainerName).
		SetImage(viper.GetString(constant.KBToolsImage)).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands(kbAgentCommand, "--port", strconv.Itoa(port)).
		AddEnv(envVars...).
		AddPorts(corev1.ContainerPort{
			ContainerPort: int32(port),
			Name:          kbAgentPortName,
			Protocol:      "TCP",
		}).
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(port)},
			}}).
		GetObject()

	// set kb-agent container ports to host network
	if synthesizedComp.HostNetwork != nil {
		if synthesizedComp.HostNetwork.ContainerPorts == nil {
			synthesizedComp.HostNetwork.ContainerPorts = make([]appsv1alpha1.HostNetworkContainerPort, 0)
		}
		synthesizedComp.HostNetwork.ContainerPorts = append(
			synthesizedComp.HostNetwork.ContainerPorts,
			appsv1alpha1.HostNetworkContainerPort{
				Container: container.Name,
				Ports:     []string{kbAgentPortName},
			})
	}

	synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, *container)
	return nil
}

func buildKBAgentStartupEnv(synthesizedComp *SynthesizedComponent) ([]corev1.EnvVar, error) {
	var (
		actions []proto.Action
		probes  []proto.Probe
	)
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.PostProvision, "postProvision"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.PreTerminate, "preTerminate"); a != nil {
		actions = append(actions, *a)
	}
	if synthesizedComp.LifecycleActions.Switchover != nil {
		if a := buildAction4KBAgentLow(synthesizedComp.LifecycleActions.Switchover.WithoutCandidate, "switchover"); a != nil {
			actions = append(actions, *a)
		}
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.MemberJoin, "memberJoin"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.MemberLeave, "memberLeave"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.Readonly, "readonly"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.Readwrite, "readwrite"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.DataDump, "dataDump"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.DataLoad, "dataLoad"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.Reconfigure, "reconfigure"); a != nil {
		actions = append(actions, *a)
	}
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.AccountProvision, "accountProvision"); a != nil {
		actions = append(actions, *a)
	}
	if a, p := buildRoleProbe4KBAgent(synthesizedComp.LifecycleActions.RoleProbe); a != nil && p != nil {
		actions = append(actions, *a)
		probes = append(probes, *p)
	}
	return kbagent.BuildEnvVars(actions, probes)
}

func buildAction4KBAgent(handler *appsv1alpha1.LifecycleActionHandler, name string) *proto.Action {
	if handler == nil {
		return nil
	}
	return buildAction4KBAgentLow(handler.CustomHandler, name)
}

func buildAction4KBAgentLow(action *appsv1alpha1.Action, name string) *proto.Action {
	if action == nil || action.Exec == nil {
		return nil
	}
	return &proto.Action{
		Name: name,
		Exec: &proto.ExecAction{
			Commands: action.Exec.Command,
			Args:     action.Exec.Args,
			// Env:       action.Env,
			Container: action.Container,
		},
		TimeoutSeconds: action.TimeoutSeconds,
		RetryPolicy:    nil,
	}
}

func buildRoleProbe4KBAgent(roleProbe *appsv1alpha1.RoleProbe) (*proto.Action, *proto.Probe) {
	if roleProbe == nil || roleProbe.CustomHandler == nil || roleProbe.CustomHandler.Exec == nil {
		return nil, nil
	}
	a := &proto.Action{
		Name: "roleProbe",
		Exec: &proto.ExecAction{
			Commands: roleProbe.CustomHandler.Exec.Command,
			Args:     roleProbe.CustomHandler.Exec.Args,
			// Env:       roleProbe.CustomHandler.Env,
			Container: roleProbe.CustomHandler.Container,
		},
		TimeoutSeconds: roleProbe.CustomHandler.TimeoutSeconds,
		RetryPolicy:    nil,
	}
	p := &proto.Probe{
		Action:              "roleProbe",
		InitialDelaySeconds: roleProbe.InitialDelaySeconds,
		PeriodSeconds:       roleProbe.PeriodSeconds,
		SuccessThreshold:    1,
		FailureThreshold:    1,
		ReportPeriodSeconds: nil,
	}
	return a, p
}
