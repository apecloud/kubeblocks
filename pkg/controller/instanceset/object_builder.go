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

package instanceset

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func buildHeadlessSvc(its workloads.InstanceSet, labels, selectors map[string]string) *corev1.Service {
	annotations := ParseAnnotationsOfScope(HeadlessServiceScope, its.Annotations)
	hdlBuilder := builder.NewHeadlessServiceBuilder(its.Namespace, getHeadlessSvcName(its.Name)).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		AddAnnotationsInMap(annotations).
		SetPublishNotReadyAddresses(true)

	portNames := sets.New[string]()
	for _, container := range its.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Protocol: port.Protocol,
				Port:     port.ContainerPort,
			}
			switch {
			case len(port.Name) > 0 && !portNames.Has(port.Name):
				portNames.Insert(port.Name)
				servicePort.Name = port.Name
			default:
				servicePort.Name = fmt.Sprintf("%s-%d", strings.ToLower(string(port.Protocol)), port.ContainerPort)
			}
			hdlBuilder.AddPorts(servicePort)
		}
	}
	return hdlBuilder.GetObject()
}

func getHeadlessSvcName(itsName string) string {
	return strings.Join([]string{itsName, "headless"}, "-")
}

func BuildPodTemplate(its *workloads.InstanceSet) *corev1.PodTemplateSpec {
	template := its.Spec.Template.DeepCopy()
	injectRoleProbeContainer(its, template)

	return template
}

func injectRoleProbeContainer(its *workloads.InstanceSet, template *corev1.PodTemplateSpec) {
	roleProbe := its.Spec.RoleProbe
	if roleProbe == nil {
		return
	}
	credential := its.Spec.Credential
	credentialEnv := make([]corev1.EnvVar, 0)
	if credential != nil {
		credentialEnv = append(credentialEnv,
			corev1.EnvVar{
				Name:      usernameCredentialVarName,
				Value:     credential.Username.Value,
				ValueFrom: credential.Username.ValueFrom,
			},
			corev1.EnvVar{
				Name:      passwordCredentialVarName,
				Value:     credential.Password.Value,
				ValueFrom: credential.Password.ValueFrom,
			})
	}

	actionSvcPorts := buildActionSvcPorts(template, roleProbe.CustomHandler)

	actionSvcList, _ := json.Marshal(actionSvcPorts)
	injectRoleProbeBaseContainer(its, template, string(actionSvcList), credentialEnv)

	if roleProbe.CustomHandler != nil {
		injectCustomRoleProbeContainer(its, template, actionSvcPorts, credentialEnv)
	}
}

func buildActionSvcPorts(template *corev1.PodTemplateSpec, actions []workloads.Action) []int32 {
	findAllUsedPorts := func() []int32 {
		allUsedPorts := make([]int32, 0)
		for _, container := range template.Spec.Containers {
			for _, port := range container.Ports {
				allUsedPorts = append(allUsedPorts, port.ContainerPort)
				allUsedPorts = append(allUsedPorts, port.HostPort)
			}
		}
		return allUsedPorts
	}

	findNextAvailablePort := func(base int32, allUsedPorts []int32) int32 {
		for port := base + 1; port < 65535; port++ {
			available := true
			for _, usedPort := range allUsedPorts {
				if port == usedPort {
					available = false
					break
				}
			}
			if available {
				return port
			}
		}
		return 0
	}

	allUsedPorts := findAllUsedPorts()
	svcPort := actionSvcPortBase
	var actionSvcPorts []int32
	for range actions {
		svcPort = findNextAvailablePort(svcPort, allUsedPorts)
		actionSvcPorts = append(actionSvcPorts, svcPort)
	}
	return actionSvcPorts
}

func injectRoleProbeBaseContainer(its *workloads.InstanceSet, template *corev1.PodTemplateSpec, actionSvcList string, credentialEnv []corev1.EnvVar) {
	// compute parameters for role probe base container
	roleProbe := its.Spec.RoleProbe
	if roleProbe == nil {
		return
	}

	// already has role probe container, for test purpose
	if _, c := controllerutil.GetContainerByName(template.Spec.Containers, roleProbeContainerName); c != nil {
		return
	}

	image := viper.GetString(constant.KBToolsImage)
	probeHTTPPort := viper.GetInt("ROLE_SERVICE_HTTP_PORT")
	if probeHTTPPort == 0 {
		probeHTTPPort = defaultRoleProbeDaemonPort
	}
	probeGRPCPort := viper.GetInt("ROLE_PROBE_GRPC_PORT")
	if probeGRPCPort == 0 {
		probeGRPCPort = defaultRoleProbeGRPCPort
	}
	env := credentialEnv
	env = append(env,
		corev1.EnvVar{
			Name:  actionSvcListVarName,
			Value: actionSvcList,
		})

	// inject role update mechanism env
	env = append(env,
		corev1.EnvVar{
			Name:  RoleUpdateMechanismVarName,
			Value: string(roleProbe.RoleUpdateMechanism),
		})

	// inject role probe timeout env
	env = append(env,
		corev1.EnvVar{
			Name:  roleProbeTimeoutVarName,
			Value: strconv.Itoa(int(roleProbe.TimeoutSeconds)),
		})

	readinessProbe := &corev1.Probe{
		InitialDelaySeconds: roleProbe.InitialDelaySeconds,
		TimeoutSeconds:      roleProbe.TimeoutSeconds,
		PeriodSeconds:       roleProbe.PeriodSeconds,
		SuccessThreshold:    roleProbe.SuccessThreshold,
		FailureThreshold:    roleProbe.FailureThreshold,
	}

	readinessProbe.ProbeHandler = corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				grpcHealthProbeBinaryPath,
				fmt.Sprintf(grpcHealthProbeArgsFormat, probeGRPCPort),
			},
		},
	}

	// if role probe container doesn't exist, create a new one
	// build container
	container := builder.NewContainerBuilder(roleProbeContainerName).
		SetImage(image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands([]string{
			roleProbeBinaryName,
			"--port", strconv.Itoa(probeHTTPPort),
			"--grpcport", strconv.Itoa(probeGRPCPort),
		}...).
		AddEnv(env...).
		AddPorts(
			corev1.ContainerPort{
				ContainerPort: int32(probeHTTPPort),
				Name:          roleProbeContainerName,
				Protocol:      "TCP",
			},
			corev1.ContainerPort{
				ContainerPort: int32(probeGRPCPort),
				Name:          roleProbeGRPCPortName,
				Protocol:      "TCP",
			},
		).
		SetReadinessProbe(*readinessProbe).
		GetObject()

	// inject role probe container
	template.Spec.Containers = append(template.Spec.Containers, *container)
}

func injectCustomRoleProbeContainer(its *workloads.InstanceSet, template *corev1.PodTemplateSpec, actionSvcPorts []int32, credentialEnv []corev1.EnvVar) {
	if its.Spec.RoleProbe == nil {
		return
	}

	// inject shared volume
	agentVolume := corev1.Volume{
		Name: roleAgentVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	template.Spec.Volumes = append(template.Spec.Volumes, agentVolume)

	// inject init container
	agentVolumeMount := corev1.VolumeMount{
		Name:      roleAgentVolumeName,
		MountPath: roleAgentVolumeMountPath,
	}
	agentPath := strings.Join([]string{roleAgentVolumeMountPath, roleAgentName}, "/")
	initContainer := corev1.Container{
		Name:            roleAgentInstallerName,
		Image:           shell2httpImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts:    []corev1.VolumeMount{agentVolumeMount},
		Command: []string{
			"cp",
			shell2httpBinaryPath,
			agentPath,
		},
	}
	template.Spec.InitContainers = append(template.Spec.InitContainers, initContainer)

	// inject action containers based on utility images
	for i, action := range its.Spec.RoleProbe.CustomHandler {
		image := action.Image
		if len(image) == 0 {
			image = defaultActionImage
		}
		command := []string{
			agentPath,
			"-port", fmt.Sprintf("%d", actionSvcPorts[i]),
			"-export-all-vars",
			"-form",
			shell2httpServePath,
			strings.Join(action.Command, " "),
		}
		container := corev1.Container{
			Name:            fmt.Sprintf("action-%d", i),
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{agentVolumeMount},
			Env:             credentialEnv,
			Command:         command,
		}
		template.Spec.Containers = append(template.Spec.Containers, container)
	}
}
