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
	"errors"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	kbAgentCommand              = "/bin/kbagent"
	kbAgentSharedMountPath      = "/kubeblocks"
	kbAgentCommandOnSharedMount = "/kubeblocks/kbagent"

	minAvailablePort   = 1025
	maxAvailablePort   = 65535
	kbAgentDefaultPort = 3501
)

var (
	sharedVolumeMount = corev1.VolumeMount{Name: "kubeblocks", MountPath: kbAgentSharedMountPath}
)

func IsKBAgentContainer(c *corev1.Container) bool {
	return c.Name == kbagent.ContainerName || c.Name == kbagent.InitContainerName
}

func UpdateKBAgentContainer4HostNetwork(synthesizedComp *SynthesizedComponent) {
	idx, c := intctrlutil.GetContainerByName(synthesizedComp.PodSpec.Containers, kbagent.ContainerName)
	if c == nil {
		return
	}

	httpPort := 0
	for _, port := range c.Ports {
		if port.Name == kbagent.DefaultPortName {
			httpPort = int(port.ContainerPort)
			break
		}
	}
	if httpPort == 0 {
		return
	}

	// update port in args
	for i, arg := range c.Args {
		if arg == "--port" {
			c.Args[i+1] = strconv.Itoa(httpPort)
			break
		}
	}

	// update startup probe
	if c.StartupProbe != nil && c.StartupProbe.TCPSocket != nil {
		c.StartupProbe.TCPSocket.Port = intstr.FromInt(httpPort)
	}

	synthesizedComp.PodSpec.Containers[idx] = *c
}

func buildKBAgentContainer(synthesizedComp *SynthesizedComponent) error {
	if synthesizedComp.LifecycleActions == nil {
		return nil
	}

	envVars, err := buildKBAgentStartupEnvs(synthesizedComp)
	if err != nil {
		return err
	}

	ports, err := getAvailablePorts(synthesizedComp.PodSpec.Containers, []int32{int32(kbAgentDefaultPort)})
	if err != nil {
		return err
	}

	port := int(ports[0])
	container := builder.NewContainerBuilder(kbagent.ContainerName).
		SetImage(viper.GetString(constant.KBToolsImage)).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands(kbAgentCommand).
		AddArgs("--port", strconv.Itoa(port)).
		AddEnv(mergedActionEnv4KBAgent(synthesizedComp)...).
		AddEnv(envVars...).
		AddPorts(corev1.ContainerPort{
			ContainerPort: int32(port),
			Name:          kbagent.DefaultPortName,
			Protocol:      "TCP",
		}).
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(port)},
			}}).
		GetObject()

	if err = adaptKBAgentIfCustomImageNContainerDefined(synthesizedComp, container); err != nil {
		return err
	}

	// set kb-agent container ports to host network
	if synthesizedComp.HostNetwork != nil {
		if synthesizedComp.HostNetwork.ContainerPorts == nil {
			synthesizedComp.HostNetwork.ContainerPorts = make([]appsv1.HostNetworkContainerPort, 0)
		}
		synthesizedComp.HostNetwork.ContainerPorts = append(
			synthesizedComp.HostNetwork.ContainerPorts,
			appsv1.HostNetworkContainerPort{
				Container: container.Name,
				Ports:     []string{kbagent.DefaultPortName},
			})
	}

	synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, *container)
	return nil
}

func mergedActionEnv4KBAgent(synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)
	envSet := sets.New[string]()

	checkedAppend := func(action *appsv1.Action) {
		if action != nil && action.Exec != nil {
			for _, e := range action.Exec.Env {
				if !envSet.Has(e.Name) {
					env = append(env, e)
					envSet.Insert(e.Name)
				}
			}
		}
	}

	for _, action := range []*appsv1.Action{
		synthesizedComp.LifecycleActions.PostProvision,
		synthesizedComp.LifecycleActions.PreTerminate,
		synthesizedComp.LifecycleActions.Switchover,
		synthesizedComp.LifecycleActions.MemberJoin,
		synthesizedComp.LifecycleActions.MemberLeave,
		synthesizedComp.LifecycleActions.Readonly,
		synthesizedComp.LifecycleActions.Readwrite,
		synthesizedComp.LifecycleActions.DataDump,
		synthesizedComp.LifecycleActions.DataLoad,
		synthesizedComp.LifecycleActions.Reconfigure,
		synthesizedComp.LifecycleActions.AccountProvision,
	} {
		checkedAppend(action)
	}
	if synthesizedComp.LifecycleActions.RoleProbe != nil {
		checkedAppend(&synthesizedComp.LifecycleActions.RoleProbe.Action)
	}

	return env
}

func buildKBAgentStartupEnvs(synthesizedComp *SynthesizedComponent) ([]corev1.EnvVar, error) {
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
	if a := buildAction4KBAgent(synthesizedComp.LifecycleActions.Switchover, "switchover"); a != nil {
		actions = append(actions, *a)
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

	if a, p := buildProbe4KBAgent(synthesizedComp.LifecycleActions.RoleProbe, "roleProbe", synthesizedComp.FullCompName); a != nil && p != nil {
		actions = append(actions, *a)
		probes = append(probes, *p)
	}
	if a, p := buildProbe4KBAgent(synthesizedComp.LifecycleActions.AvailableProbe, availableProbe, synthesizedComp.FullCompName); a != nil && p != nil {
		actions = append(actions, *a)
		probes = append(probes, *p)
	}

	return kbagent.BuildStartupEnv(actions, probes)
}

func buildAction4KBAgent(action *appsv1.Action, name string) *proto.Action {
	if action == nil || action.Exec == nil {
		return nil
	}
	a := &proto.Action{
		Name: name,
		Exec: &proto.ExecAction{
			Commands: action.Exec.Command,
			Args:     action.Exec.Args,
		},
		TimeoutSeconds: action.TimeoutSeconds,
	}
	if action.RetryPolicy != nil {
		a.RetryPolicy = &proto.RetryPolicy{
			MaxRetries:    action.RetryPolicy.MaxRetries,
			RetryInterval: action.RetryPolicy.RetryInterval,
		}
	}
	return a
}

func buildProbe4KBAgent(probe *appsv1.Probe, name, instance string) (*proto.Action, *proto.Probe) {
	if probe == nil || probe.Exec == nil {
		return nil, nil
	}
	a := buildAction4KBAgent(&probe.Action, name)
	p := &proto.Probe{
		Action:              name,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
		ReportPeriodSeconds: nil, // TODO: impl
		Instance:            instance,
	}
	return a, p
}

func adaptKBAgentIfCustomImageNContainerDefined(synthesizedComp *SynthesizedComponent, container *corev1.Container) error {
	image, c, err := customExecActionImageNContainer(synthesizedComp)
	if err != nil {
		return err
	}

	if len(image) > 0 {
		// init-container to copy binaries to the shared mount point /kubeblocks
		initContainer := buildKBAgentInitContainer()
		synthesizedComp.PodSpec.InitContainers = append(synthesizedComp.PodSpec.InitContainers, *initContainer)

		container.Image = image
		container.Command[0] = kbAgentCommandOnSharedMount
		container.VolumeMounts = append(container.VolumeMounts, sharedVolumeMount)
	}

	// TODO: share more container resources
	if c != nil {
		container.VolumeMounts = append(container.VolumeMounts, c.VolumeMounts...)
	}

	return nil
}

func customExecActionImageNContainer(synthesizedComp *SynthesizedComponent) (string, *corev1.Container, error) {
	if synthesizedComp.LifecycleActions == nil {
		return "", nil, nil
	}

	actions := []*appsv1.Action{
		synthesizedComp.LifecycleActions.PostProvision,
		synthesizedComp.LifecycleActions.PreTerminate,
		synthesizedComp.LifecycleActions.Switchover,
		synthesizedComp.LifecycleActions.MemberJoin,
		synthesizedComp.LifecycleActions.MemberLeave,
		synthesizedComp.LifecycleActions.Readonly,
		synthesizedComp.LifecycleActions.Readwrite,
		synthesizedComp.LifecycleActions.DataDump,
		synthesizedComp.LifecycleActions.DataLoad,
		synthesizedComp.LifecycleActions.Reconfigure,
		synthesizedComp.LifecycleActions.AccountProvision,
	}
	if synthesizedComp.LifecycleActions.RoleProbe != nil && synthesizedComp.LifecycleActions.RoleProbe.Exec != nil {
		actions = append(actions, &synthesizedComp.LifecycleActions.RoleProbe.Action)
	}

	var image, container string
	for _, action := range actions {
		if action == nil || action.Exec == nil {
			continue
		}
		if action.Exec.Image != "" {
			if len(image) > 0 && image != action.Exec.Image {
				return "", nil, fmt.Errorf("only one exec image is allowed in lifecycle actions")
			}
			image = action.Exec.Image
		}
		if action.Exec.Container != "" {
			if len(container) > 0 && container != action.Exec.Container {
				return "", nil, fmt.Errorf("only one exec container is allowed in lifecycle actions")
			}
			container = action.Exec.Container
		}
	}

	var c *corev1.Container
	if len(container) > 0 {
		for i, cc := range synthesizedComp.PodSpec.Containers {
			if cc.Name == container {
				c = &synthesizedComp.PodSpec.Containers[i]
				break
			}
		}
		if c == nil {
			return "", nil, fmt.Errorf("exec container %s not found", container)
		}
	}
	return image, c, nil
}

func buildKBAgentInitContainer() *corev1.Container {
	return builder.NewContainerBuilder(kbagent.InitContainerName).
		SetImage(viper.GetString(constant.KBToolsImage)).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands([]string{"cp", "-r", kbAgentCommand, "/bin/curl", kbAgentSharedMountPath + "/"}...).
		AddVolumeMounts(sharedVolumeMount).
		GetObject()
}

func getAvailablePorts(containers []corev1.Container, containerPorts []int32) ([]int32, error) {
	inUse, err := getInUsePorts(containers)
	if err != nil {
		return nil, err
	}
	availablePort := make([]int32, len(containerPorts))
	for i, p := range containerPorts {
		if availablePort[i], err = iterAvailablePort(p, inUse); err != nil {
			return nil, err
		}
	}
	return availablePort, nil
}

func getInUsePorts(containers []corev1.Container) (map[int32]bool, error) {
	inUse := map[int32]bool{}
	for _, container := range containers {
		for _, v := range container.Ports {
			_, ok := inUse[v.ContainerPort]
			if ok {
				return nil, fmt.Errorf("containerPorts conflict: [%+v]", v.ContainerPort)
			}
			inUse[v.ContainerPort] = true
		}
	}
	return inUse, nil
}

func iterAvailablePort(port int32, set map[int32]bool) (int32, error) {
	if port < minAvailablePort || port > maxAvailablePort {
		port = minAvailablePort
	}
	sentinel := port
	for {
		if _, ok := set[port]; !ok {
			set[port] = true
			return port, nil
		}
		port++
		if port == sentinel {
			return -1, errors.New("no available port for container")
		}
		if port > maxAvailablePort {
			port = minAvailablePort
		}
	}
}
