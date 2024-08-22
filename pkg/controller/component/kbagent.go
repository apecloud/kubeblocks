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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	kbAgentContainerName     = "kba"
	kbAgentInitContainerName = "init-kbagent"
	kbAgentCommand           = "/bin/kbagent"
	kbAgentPortName          = "http"
	kbAgentPortArg           = "--port"
	kbAgentTCPProtocol       = "TCP"

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
	// TODO: Because the implementation of multiple images is required, an update is needed here. About kbAgentContainerName
	return c.Name == kbAgentContainerName || c.Name == kbAgentInitContainerName || strings.Contains(kbAgentContainerName, c.Name)
}

func UpdateKBAgentContainer4HostNetwork(synthesizedComp *SynthesizedComponent) {
	// TODO: Because the implementation of multiple images is required, an update is needed here. About KbAgentContainerName
	idx, c := intctrlutil.GetContainerByName(synthesizedComp.PodSpec.Containers, kbAgentContainerName)
	if c == nil {
		return
	}

	httpPort := 0
	for _, port := range c.Ports {
		if port.Name == kbAgentPortName {
			httpPort = int(port.ContainerPort)
			break
		}
	}
	if httpPort == 0 {
		return
	}

	// update port in args
	for i, arg := range c.Args {
		if arg == kbAgentPortArg {
			c.Args[i+1] = strconv.Itoa(httpPort)
			break
		}
	}

	// update startup probe
	if c.StartupProbe != nil && c.StartupProbe.TCPSocket != nil {
		c.StartupProbe.TCPSocket.Port = intstr.FromInt32(int32(httpPort))
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

	container := builder.NewContainerBuilder(kbAgentContainerName).
		SetImage(viper.GetString(constant.KBToolsImage)).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands(kbAgentCommand).
		AddEnv(envVars...).
		GetObject()

	containers, err := adaptKBAgentIfCustomImageNContainerDefined(synthesizedComp, *container)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		containers = append(containers, container)
	}

	allPorts := make([]int32, 0)
	for i := range containers {
		allPorts = append(allPorts, int32(kbAgentDefaultPort+i))
	}
	ports, err := getAvailablePorts(synthesizedComp.PodSpec.Containers, allPorts)
	if err != nil {
		return err
	}

	for i, container := range containers {
		container.Ports = []corev1.ContainerPort{
			{
				ContainerPort: ports[i],
				Name:          fmt.Sprintf("%s-%s", container.Name, kbAgentPortName),
				Protocol:      kbAgentTCPProtocol,
			},
		}
		container.Args = append(container.Args, kbAgentPortArg, strconv.Itoa(int(ports[i])))
		container.StartupProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(ports[i])},
			},
		}
	}

	if synthesizedComp.HostNetwork != nil {
		if synthesizedComp.HostNetwork.ContainerPorts == nil {
			synthesizedComp.HostNetwork.ContainerPorts = make([]appsv1alpha1.HostNetworkContainerPort, 0)
		}
		for _, container := range containers {
			synthesizedComp.HostNetwork.ContainerPorts = append(
				synthesizedComp.HostNetwork.ContainerPorts,
				appsv1alpha1.HostNetworkContainerPort{
					Container: container.Name,
					// TODO The port name is limited to 15 characters and needs to adjust the method of fetching ports in pod name and kb-agent client.
					Ports: []string{fmt.Sprintf("%s-%s", container.Name, kbAgentPortName)},
				})
		}
	}

	for _, container := range containers {
		synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, *container)
	}
	return nil
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

	if a, p := buildProbe4KBAgent(synthesizedComp.LifecycleActions.RoleProbe, "roleProbe"); a != nil && p != nil {
		actions = append(actions, *a)
		probes = append(probes, *p)
	}

	return kbagent.BuildStartupEnvs(actions, probes)
}

func buildAction4KBAgent(action *appsv1alpha1.Action, name string) *proto.Action {
	if action == nil || action.Exec == nil {
		return nil
	}
	a := &proto.Action{
		Name: name,
		Exec: &proto.ExecAction{
			Commands: action.Exec.Command,
			Args:     action.Exec.Args,
			// Env:      action.Exec.Env,
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

func buildProbe4KBAgent(probe *appsv1alpha1.Probe, name string) (*proto.Action, *proto.Probe) {
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
	}
	return a, p
}

func adaptKBAgentIfCustomImageNContainerDefined(synthesizedComp *SynthesizedComponent, container corev1.Container) ([]*corev1.Container, error) {
	actionNames, images, containers, err := customExecActionImageNContainer(synthesizedComp)
	if err != nil {
		return nil, err
	}
	if len(actionNames) == 0 {
		return nil, nil
	}
	// init-container to copy binaries to the shared mount point /kubeblocks
	initContainer := buildKBAgentInitContainer()
	synthesizedComp.PodSpec.InitContainers = append(synthesizedComp.PodSpec.InitContainers, *initContainer)

	cc := make([]*corev1.Container, len(containers))
	for i, c := range containers {
		wrapContainer := container.DeepCopy()
		if c == nil {
			wrapContainer.Image = images[i]
		} else {
			wrapContainer.Image = c.Image
		}
		wrapContainer.Name = fmt.Sprintf("%s-%s", actionNames[i], kbAgentContainerName)
		wrapContainer.Command[0] = kbAgentCommandOnSharedMount
		wrapContainer.VolumeMounts = uniqueVolumeMounts(wrapContainer.VolumeMounts, []corev1.VolumeMount{sharedVolumeMount})
		// TODO: share more container resources
		if c != nil {
			wrapContainer.VolumeMounts = uniqueVolumeMounts(wrapContainer.VolumeMounts, c.VolumeMounts)
		}
		cc[i] = wrapContainer
	}
	return cc, nil
}

func customExecActionImageNContainer(synthesizedComp *SynthesizedComponent) ([]string, []string, []*corev1.Container, error) {
	if synthesizedComp.LifecycleActions == nil {
		return nil, nil, nil, nil
	}

	actions := []*appsv1alpha1.Action{
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

	// TODO: Discussion is needed for the definition of action name
	actionNameMap := map[*appsv1alpha1.Action]string{
		synthesizedComp.LifecycleActions.PostProvision:    "postpr",
		synthesizedComp.LifecycleActions.PreTerminate:     "preter",
		synthesizedComp.LifecycleActions.Switchover:       "switch",
		synthesizedComp.LifecycleActions.MemberJoin:       "mbrin",
		synthesizedComp.LifecycleActions.MemberLeave:      "mbrlv",
		synthesizedComp.LifecycleActions.Readonly:         "readol",
		synthesizedComp.LifecycleActions.Readwrite:        "readwr",
		synthesizedComp.LifecycleActions.DataDump:         "datadp",
		synthesizedComp.LifecycleActions.DataLoad:         "datald",
		synthesizedComp.LifecycleActions.Reconfigure:      "reconf",
		synthesizedComp.LifecycleActions.AccountProvision: "accpr",
	}
	if synthesizedComp.LifecycleActions.RoleProbe != nil && synthesizedComp.LifecycleActions.RoleProbe.Exec != nil {
		actionNameMap[&synthesizedComp.LifecycleActions.RoleProbe.Action] = "rolepb"
	}

	var images []string
	var containerNames []string
	var actionNames []string
	for _, action := range actions {
		if action == nil || action.Exec == nil {
			continue
		}
		image := action.Exec.Image
		containerName := action.Exec.Container
		if len(image) == 0 && len(containerName) == 0 {
			continue
		}
		images = append(images, image)
		containerNames = append(containerNames, containerName)
		actionNames = append(actionNames, actionNameMap[action])
	}

	containers := make([]*corev1.Container, len(images))
	for i := 0; i < len(images); i++ {
		var c *corev1.Container
		if len(containerNames[i]) > 0 {
			for j, cc := range synthesizedComp.PodSpec.Containers {
				if cc.Name == containerNames[i] {
					c = &synthesizedComp.PodSpec.Containers[j]
					break
				}
			}
			if c == nil {
				return nil, nil, nil, fmt.Errorf("exec container %s not found", containerNames[i])
			}
			if len(images[i]) > 0 && c.Image != images[i] {
				return nil, nil, nil, fmt.Errorf("exec image and container must be the same")
			}
			containers[i] = c
		} else {
			containers[i] = nil
		}
	}
	return actionNames, images, containers, nil
}

func buildKBAgentInitContainer() *corev1.Container {
	return builder.NewContainerBuilder(kbAgentInitContainerName).
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

func uniqueVolumeMounts(existingMounts []corev1.VolumeMount, newMounts []corev1.VolumeMount) []corev1.VolumeMount {
	for _, vm := range newMounts {
		if !mountPathExists(existingMounts, vm.MountPath) {
			existingMounts = append(existingMounts, vm)
		}
	}
	return existingMounts
}

func mountPathExists(volumeMounts []corev1.VolumeMount, mountPath string) bool {
	for _, vm := range volumeMounts {
		if vm.MountPath == mountPath {
			return true
		}
	}
	return false
}
