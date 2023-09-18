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
	"github.com/leaanthony/debme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

const (
	// http://localhost:<port>/v1.0/bindings/<binding_type>
	getGlobalInfoFormat       = "/v1.0/bindings/%s?operation=getGlobalInfo"
	checkRunningURIFormat     = "/v1.0/bindings/%s?operation=checkRunning"
	checkStatusURIFormat      = "/v1.0/bindings/%s?operation=checkStatus"
	volumeProtectionURIFormat = "/v1.0/bindings/%s?operation=volumeProtection"

	dataVolume = "data"
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

func buildLorryContainers(reqCtx intctrlutil.RequestCtx, component *SynthesizedComponent) error {
	container, err := buildLorryContainer()
	if err != nil {
		return err
	}

	lorryContainers := []corev1.Container{}
	componentLorry := component.Probes
	if componentLorry == nil {
		return nil
	}
	reqCtx.Log.V(3).Info("lorry", "settings", componentLorry)
	lorrySvcHTTPPort := viper.GetInt32("PROBE_SERVICE_HTTP_PORT")
	lorrySvcGRPCPort := viper.GetInt32("PROBE_SERVICE_GRPC_PORT")
	// override by new env name
	if viper.IsSet("LORRY_SERVICE_HTTP_PORT") {
		lorrySvcHTTPPort = viper.GetInt32("LORRY_SERVICE_HTTP_PORT")
	}
	if viper.IsSet("LORRY_SERVICE_GRPC_PORT") {
		lorrySvcGRPCPort = viper.GetInt32("LORRY_SERVICE_GRPC_PORT")
	}
	availablePorts, err := getAvailableContainerPorts(component.PodSpec.Containers, []int32{lorrySvcHTTPPort, lorrySvcGRPCPort})
	lorrySvcHTTPPort = availablePorts[0]
	lorrySvcGRPCPort = availablePorts[1]
	if err != nil {
		reqCtx.Log.Info("get lorry container port failed", "error", err)
		return err
	}

	if componentLorry.StatusProbe != nil {
		statusProbeContainer := container.DeepCopy()
		buildStatusProbeContainer(component.CharacterType, statusProbeContainer, componentLorry.StatusProbe, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *statusProbeContainer)
	}

	if componentLorry.RunningProbe != nil {
		runningProbeContainer := container.DeepCopy()
		buildRunningProbeContainer(component.CharacterType, runningProbeContainer, componentLorry.RunningProbe, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *runningProbeContainer)
	}

	if volumeProtectionEnabled(component) {
		c := container.DeepCopy()
		buildVolumeProtectionProbeContainer(component.CharacterType, c, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *c)
	}

	if len(lorryContainers) >= 1 {
		container := &lorryContainers[0]
		buildLorryServiceContainer(component, container, int(lorrySvcHTTPPort), int(lorrySvcGRPCPort))
	}

	reqCtx.Log.V(1).Info("lorry", "containers", lorryContainers)
	component.PodSpec.Containers = append(component.PodSpec.Containers, lorryContainers...)
	return nil
}

func buildLorryContainer() (*corev1.Container, error) {
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

func buildLorryServiceContainer(component *SynthesizedComponent, container *corev1.Container, probeSvcHTTPPort int, probeSvcGRPCPort int) {
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"lorry",
		"--port", strconv.Itoa(probeSvcHTTPPort)}

	if len(component.PodSpec.Containers) > 0 {
		mainContainer := component.PodSpec.Containers[0]
		if len(mainContainer.Ports) > 0 {
			port := mainContainer.Ports[0]
			dbPort := port.ContainerPort
			container.Env = append(container.Env, corev1.EnvVar{
				Name:      constant.KBEnvServicePort,
				Value:     strconv.Itoa(int(dbPort)),
				ValueFrom: nil,
			})
		}

		dataVolumeName := dataVolume
		for _, v := range component.VolumeTypes {
			if v.Type == appsv1alpha1.VolumeTypeData {
				dataVolumeName = v.Name
			}
		}
		for _, volumeMount := range mainContainer.VolumeMounts {
			if volumeMount.Name != dataVolumeName {
				continue
			}
			vm := volumeMount.DeepCopy()
			container.VolumeMounts = []corev1.VolumeMount{*vm}
			container.Env = append(container.Env, corev1.EnvVar{
				Name:      constant.KBEnvDataPath,
				Value:     vm.MountPath,
				ValueFrom: nil,
			})
		}
	}

	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBEnvCharacterType,
		Value:     component.CharacterType,
		ValueFrom: nil,
	})

	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBEnvWorkloadType,
		Value:     string(component.WorkloadType),
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

	// pass the volume protection spec to lorry container through env.
	if volumeProtectionEnabled(component) {
		container.Env = append(container.Env, env4VolumeProtection(*component.VolumeProtection))
	}
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
}

func volumeProtectionEnabled(component *SynthesizedComponent) bool {
	return component.VolumeProtection != nil
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
