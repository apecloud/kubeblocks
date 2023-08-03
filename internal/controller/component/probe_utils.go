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

package component

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	// http://localhost:<port>/v1.0/bindings/<binding_type>
	checkRoleURIFormat        = "/v1.0/bindings/%s?operation=checkRole"
	getGlobalInfoFormat       = "/v1.0/bindings/%s?operation=getGlobalInfo"
	checkRunningURIFormat     = "/v1.0/bindings/%s?operation=checkRunning"
	checkStatusURIFormat      = "/v1.0/bindings/%s?operation=checkStatus"
	volumeProtectionURIFormat = "/v1.0/bindings/%s?operation=volumeProtection"
)

var (
	//go:embed cue/*
	cueTemplates embed.FS

	// default probe setting for volume protection.
	defaultVolumeProtectionProbe = appsv1alpha1.ClusterDefinitionProbe{
		PeriodSeconds:    60,
		TimeoutSeconds:   5,
		FailureThreshold: 3,
	}
)

func buildProbeContainers(reqCtx intctrlutil.RequestCtx, component *SynthesizedComponent) error {
	container, err := buildProbeContainer()
	if err != nil {
		return err
	}

	probeContainers := []corev1.Container{}
	componentProbes := component.Probes
	if componentProbes == nil {
		return nil
	}
	reqCtx.Log.V(3).Info("probe", "settings", componentProbes)
	probeSvcHTTPPort := viper.GetInt32("PROBE_SERVICE_HTTP_PORT")
	probeSvcGRPCPort := viper.GetInt32("PROBE_SERVICE_GRPC_PORT")
	availablePorts, err := getAvailableContainerPorts(component.PodSpec.Containers, []int32{probeSvcHTTPPort, probeSvcGRPCPort})
	probeSvcHTTPPort = availablePorts[0]
	probeSvcGRPCPort = availablePorts[1]
	if err != nil {
		reqCtx.Log.Info("get probe container port failed", "error", err)
		return err
	}

	injectHttp2Shell(component.PodSpec)

	if componentProbes.RoleProbe != nil {
		roleChangedContainer := container.DeepCopy()
		buildRoleProbeContainer(component.CharacterType, roleChangedContainer, componentProbes.RoleProbe, int(probeSvcHTTPPort), component.PodSpec)
		probeContainers = append(probeContainers, *roleChangedContainer)
	}

	if componentProbes.StatusProbe != nil {
		statusProbeContainer := container.DeepCopy()
		buildStatusProbeContainer(component.CharacterType, statusProbeContainer, componentProbes.StatusProbe, int(probeSvcHTTPPort))
		probeContainers = append(probeContainers, *statusProbeContainer)
	}

	if componentProbes.RunningProbe != nil {
		runningProbeContainer := container.DeepCopy()
		buildRunningProbeContainer(component.CharacterType, runningProbeContainer, componentProbes.RunningProbe, int(probeSvcHTTPPort))
		probeContainers = append(probeContainers, *runningProbeContainer)
	}

	if volumeProtectionEnabled(component) {
		c := container.DeepCopy()
		buildVolumeProtectionProbeContainer(component.CharacterType, c, int(probeSvcHTTPPort))
		probeContainers = append(probeContainers, *c)
	}

	if len(probeContainers) >= 1 {
		container := &probeContainers[0]
		buildProbeServiceContainer(component, container, int(probeSvcHTTPPort), int(probeSvcGRPCPort))
	}

	reqCtx.Log.V(1).Info("probe", "containers", probeContainers)
	component.PodSpec.Containers = append(component.PodSpec.Containers, probeContainers...)
	return nil
}

func buildProbeContainer() (*corev1.Container, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("probe_template.cue"))
	if err != nil {
		return nil, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	probeContainerByte, err := cueValue.Lookup("probeContainer")
	if err != nil {
		return nil, err
	}
	container := &corev1.Container{}
	if err = json.Unmarshal(probeContainerByte, container); err != nil {
		return nil, err
	}
	return container, nil
}

func buildProbeServiceContainer(component *SynthesizedComponent, container *corev1.Container, probeSvcHTTPPort int, probeSvcGRPCPort int) {
	container.Image = viper.GetString(constant.KBProbeImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"probe",
		"--port", "3501"} // fixme: port shouldn't be const, it should be probeSvcHTTPPort

	if len(component.PodSpec.Containers) > 0 && len(component.PodSpec.Containers[0].Ports) > 0 {
		mainContainer := component.PodSpec.Containers[0]
		port := mainContainer.Ports[0]
		dbPort := port.ContainerPort
		container.Env = append(container.Env, corev1.EnvVar{
			Name:      constant.KBPrefix + "_SERVICE_PORT",
			Value:     strconv.Itoa(int(dbPort)),
			ValueFrom: nil,
		})
	}

	roles := getComponentRoles(component)
	rolesJSON, _ := json.Marshal(roles)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBPrefix + "_SERVICE_ROLES",
		Value:     string(rolesJSON),
		ValueFrom: nil,
	})

	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBPrefix + "_SERVICE_CHARACTER_TYPE",
		Value:     component.CharacterType,
		ValueFrom: nil,
	})

	// todo: only support consensus now, to enable ReplicationSet in the future
	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBPrefix + "_CONSENSUS_SET_ACTION_SVC_LIST",
		Value:     viper.GetString(constant.KBPrefix + "_CONSENSUS_SET_ACTION_SVC_LIST"),
		ValueFrom: nil,
	})

	container.Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(probeSvcHTTPPort),
			Name:          constant.ProbeHTTPPortName,
			Protocol:      "TCP",
		},
		{
			ContainerPort: int32(probeSvcGRPCPort),
			Name:          constant.ProbeGRPCPortName,
			Protocol:      "TCP",
		}}

	// pass the volume protection spec to probe container through env.
	if volumeProtectionEnabled(component) {
		container.Env = append(container.Env, env4VolumeProtection(*component.VolumeProtection))
	}
}

func getComponentRoles(component *SynthesizedComponent) map[string]string {
	var roles = map[string]string{}
	if component.ConsensusSpec == nil {
		return roles
	}

	consensus := component.ConsensusSpec
	roles[strings.ToLower(consensus.Leader.Name)] = string(consensus.Leader.AccessMode)
	for _, follower := range consensus.Followers {
		roles[strings.ToLower(follower.Name)] = string(follower.AccessMode)
	}
	if consensus.Learner != nil {
		roles[strings.ToLower(consensus.Learner.Name)] = string(consensus.Learner.AccessMode)
	}
	return roles
}

func buildRoleProbeContainer(characterType string, roleChangedContainer *corev1.Container,
	probeSetting *appsv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int, pod *corev1.PodSpec) {
	roleChangedContainer.Name = constant.RoleProbeContainerName
	probe := roleChangedContainer.ReadinessProbe
	bindingType := strings.ToLower(characterType)
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(getGlobalInfoFormat, bindingType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	probe.TimeoutSeconds = probeSetting.TimeoutSeconds
	probe.FailureThreshold = probeSetting.FailureThreshold
	roleChangedContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)

	base := probeSvcHTTPPort + 2
	portNeeded := len(probeSetting.Actions)
	activePorts := make([]int32, portNeeded)
	for i := 0; i < portNeeded; i++ {
		activePorts[i] = int32(base + i)
	}
	activePorts, err := getAvailableContainerPorts(pod.Containers, activePorts)
	if err != nil {
		return
	}
	marshal, err := json.Marshal(activePorts)
	if err != nil {
		return
	}
	viper.Set("KB_CONSENSUS_SET_ACTION_SVC_LIST", string(marshal))

	addTokenEnv(roleChangedContainer)
	injectProbeUtilImages(pod, probeSetting, activePorts, "/role", "checkrole", roleChangedContainer.Env)
}

func buildStatusProbeContainer(characterType string, statusProbeContainer *corev1.Container,
	probeSetting *appsv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	statusProbeContainer.Name = constant.StatusProbeContainerName
	probe := statusProbeContainer.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(checkStatusURIFormat, characterType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	probe.TimeoutSeconds = probeSetting.TimeoutSeconds
	probe.FailureThreshold = probeSetting.FailureThreshold
	statusProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
	addTokenEnv(statusProbeContainer)
}

func buildRunningProbeContainer(characterType string, runningProbeContainer *corev1.Container,
	probeSetting *appsv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	runningProbeContainer.Name = constant.RunningProbeContainerName
	probe := runningProbeContainer.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(checkRunningURIFormat, characterType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	probe.TimeoutSeconds = probeSetting.TimeoutSeconds
	probe.FailureThreshold = probeSetting.FailureThreshold
	runningProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
	addTokenEnv(runningProbeContainer)
}

func volumeProtectionEnabled(component *SynthesizedComponent) bool {
	return component.VolumeProtection != nil && viper.GetBool(constant.EnableRBACManager)
}

func buildVolumeProtectionProbeContainer(characterType string, c *corev1.Container, probeSvcHTTPPort int) {
	c.Name = constant.VolumeProtectionProbeContainerName
	probe := c.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(volumeProtectionURIFormat, characterType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = defaultVolumeProtectionProbe.PeriodSeconds
	probe.TimeoutSeconds = defaultVolumeProtectionProbe.TimeoutSeconds
	probe.FailureThreshold = defaultVolumeProtectionProbe.FailureThreshold
	c.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
	addTokenEnv(c)
}

func env4VolumeProtection(spec appsv1alpha1.VolumeProtectionSpec) corev1.EnvVar {
	value, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Sprintf("marshal volume protection spec error: %s", err.Error()))
	}
	return corev1.EnvVar{
		Name:  constant.KBEnvVolumeProtectionSpec,
		Value: string(value),
	}
}

func addTokenEnv(container *corev1.Container) {
	token := viper.GetString("PROBE_SERVICE_TOKEN")
	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBPrefix + "_PROBE_TOKEN",
		Value:     token,
		ValueFrom: nil,
	})
}

func injectHttp2Shell(pod *corev1.PodSpec) {
	// inject shared volume
	agentVolume := corev1.Volume{
		Name: constant.ProbeAgentMountName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	pod.Volumes = append(pod.Volumes, agentVolume)

	// inject shell2http
	volumeMount := corev1.VolumeMount{
		Name:      constant.ProbeAgentMountName,
		MountPath: constant.ProbeAgentMountPath,
	}
	binPath := strings.Join([]string{constant.ProbeAgentMountPath, constant.ProbeAgent}, "/")
	initContainer := corev1.Container{
		Name:            constant.ProbeAgent,
		Image:           constant.ProbeAgentImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts:    []corev1.VolumeMount{volumeMount},
		Command: []string{
			"cp",
			constant.OriginBinaryPath,
			binPath,
		},
	}
	pod.InitContainers = append(pod.InitContainers, initContainer)
}

func injectProbeUtilImages(pod *corev1.PodSpec, probeSetting *appsv1alpha1.ClusterDefinitionProbe,
	port []int32, path, usage string,
	credentialEnv []corev1.EnvVar) {
	actions := probeSetting.Actions
	volumeMount := corev1.VolumeMount{
		Name:      constant.ProbeAgentMountName,
		MountPath: constant.ProbeAgentMountPath,
	}
	binPath := strings.Join([]string{constant.ProbeAgentMountPath, constant.ProbeAgent}, "/")

	for i, action := range actions {
		image := action.Image
		if len(action.Image) == 0 {
			image = constant.DefaultActionImage
		}

		command := []string{
			binPath,
			"-port", fmt.Sprintf("%d", port[i]),
			"-export-all-vars",
			"-form",
			path,
			strings.Join(action.Command, " "),
		}

		container := corev1.Container{
			Name:            fmt.Sprintf("%s-action-%d", usage, i),
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{volumeMount},
			Env:             credentialEnv,
			Command:         command,
		}

		pod.Containers = append(pod.Containers, container)
	}
}
