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
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	// http://localhost:<port>/v1.0/bindings/<binding_type>
	// checkRoleURIFormat        = "/v1.0/bindings/%s?operation=checkRole&workloadType=%s"
	checkRoleURIFormat        = "/v1.0/checkrole"
	checkRunningURIFormat     = "/v1.0/bindings/%s?operation=checkRunning"
	checkStatusURIFormat      = "/v1.0/bindings/%s?operation=checkStatus"
	volumeProtectionURIFormat = "/v1.0/bindings/%s?operation=volumeProtection"

	dataVolume = "data"
)

var (
	// default probe setting for volume protection.
	defaultVolumeProtectionProbe = appsv1alpha1.ClusterDefinitionProbe{
		PeriodSeconds:    60,
		TimeoutSeconds:   5,
		FailureThreshold: 3,
	}
)

func buildLorryContainers(reqCtx intctrlutil.RequestCtx, component *SynthesizedComponent) error {
	container := buildBasicContainer()
	var lorryContainers []corev1.Container
	componentProbes := component.Probes
	if componentProbes == nil {
		return nil
	}
	reqCtx.Log.V(3).Info("lorry", "settings", componentProbes)
	lorrySvcHTTPPort := viper.GetInt32("PROBE_SERVICE_HTTP_PORT")
	// override by new env name
	if viper.IsSet("LORRY_SERVICE_HTTP_PORT") {
		lorrySvcHTTPPort = viper.GetInt32("LORRY_SERVICE_HTTP_PORT")
	}
	availablePorts, err := getAvailableContainerPorts(component.PodSpec.Containers, []int32{lorrySvcHTTPPort})
	lorrySvcHTTPPort = availablePorts[0]
	if err != nil {
		reqCtx.Log.Info("get lorry container port failed", "error", err)
		return err
	}
	lorrySvcGRPCPort := viper.GetInt("PROBE_SERVICE_GRPC_PORT")

	if componentProbes.RoleProbe != nil && (component.RSMSpec == nil || component.RSMSpec.RoleProbe == nil) {
		roleChangedContainer := container.DeepCopy()
		buildRoleProbeContainer(component, roleChangedContainer, componentProbes.RoleProbe, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *roleChangedContainer)
	}

	if componentProbes.StatusProbe != nil {
		statusProbeContainer := container.DeepCopy()
		buildStatusProbeContainer(component.CharacterType, statusProbeContainer, componentProbes.StatusProbe, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *statusProbeContainer)
	}

	if componentProbes.RunningProbe != nil {
		runningProbeContainer := container.DeepCopy()
		buildRunningProbeContainer(component.CharacterType, runningProbeContainer, componentProbes.RunningProbe, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *runningProbeContainer)
	}

	if volumeProtectionEnabled(component) {
		c := container.DeepCopy()
		buildVolumeProtectionProbeContainer(component.CharacterType, c, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *c)
	}

	// inject WeSyncer(currently part of Lorry) in cluster controller.
	// as all the above features share the lorry service, only one lorry need to be injected.
	// if none of the above feature enabled, WeSyncer still need to be injected for the HA feature functions well.
	if len(lorryContainers) == 0 {
		weSyncerContainer := container.DeepCopy()
		buildWeSyncerContainer(weSyncerContainer, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *weSyncerContainer)
	}

	buildLorryServiceContainer(component, &lorryContainers[0], int(lorrySvcHTTPPort), lorrySvcGRPCPort)

	reqCtx.Log.V(1).Info("lorry", "containers", lorryContainers)
	component.PodSpec.Containers = append(component.PodSpec.Containers, lorryContainers...)
	return nil
}

func buildBasicContainer() *corev1.Container {
	return builder.NewContainerBuilder("string").
		SetImage("infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/google_containers/pause:3.6").
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands("/pause").
		AddEnv(corev1.EnvVar{
			Name: "KB_SERVICE_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  "username",
					LocalObjectReference: corev1.LocalObjectReference{Name: "$(CONN_CREDENTIAL_SECRET_NAME)"},
				},
			}},
			corev1.EnvVar{
				Name: "KB_SERVICE_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "password",
						LocalObjectReference: corev1.LocalObjectReference{Name: "$(CONN_CREDENTIAL_SECRET_NAME)"},
					},
				},
			}).
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(3501)},
			}}).
		GetObject()
}

func buildLorryServiceContainer(component *SynthesizedComponent, container *corev1.Container, lorrySvcHTTPPort, lorrySvcGRPCPort int) {
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"lorry",
		"--port", strconv.Itoa(lorrySvcHTTPPort),
		"--config-path", "/config/lorry/components/",
		"--grpcport", strconv.Itoa(lorrySvcGRPCPort),
	}

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

	secretName := fmt.Sprintf("%s-conn-credential", component.ClusterName)
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:      constant.KBEnvCharacterType,
			Value:     component.CharacterType,
			ValueFrom: nil,
		},
		corev1.EnvVar{
			Name:      constant.KBEnvWorkloadType,
			Value:     string(component.WorkloadType),
			ValueFrom: nil,
		},
		corev1.EnvVar{
			Name: constant.KBEnvServiceUser,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountNameForSecret,
				},
			},
		},
		corev1.EnvVar{
			Name: constant.KBEnvServicePassword,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountPasswdForSecret,
				},
			},
		})

	container.Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(lorrySvcHTTPPort),
			Name:          constant.LorryHTTPPortName,
			Protocol:      "TCP",
		},
		{
			ContainerPort: int32(lorrySvcGRPCPort),
			Name:          constant.LorryGRPCPortName,
			Protocol:      "TCP",
		},
	}

	// pass the volume protection spec to lorry container through env.
	if volumeProtectionEnabled(component) {
		container.Env = append(container.Env, env4VolumeProtection(*component.VolumeProtection))
	}
}

func buildWeSyncerContainer(weSyncerContainer *corev1.Container, probeSvcHTTPPort int) {
	weSyncerContainer.Name = constant.WeSyncerContainerName
	weSyncerContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildRoleProbeContainer(component *SynthesizedComponent, roleChangedContainer *corev1.Container,
	probeSetting *appsv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	roleChangedContainer.Name = constant.RoleProbeContainerName
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = checkRoleURIFormat
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe := &corev1.Probe{}
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	probe.TimeoutSeconds = probeSetting.TimeoutSeconds
	probe.FailureThreshold = probeSetting.FailureThreshold
	roleChangedContainer.ReadinessProbe = probe
	roleChangedContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildStatusProbeContainer(characterType string, statusProbeContainer *corev1.Container,
	probeSetting *appsv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	statusProbeContainer.Name = constant.StatusProbeContainerName
	probe := &corev1.Probe{}
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(checkStatusURIFormat, characterType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	probe.TimeoutSeconds = probeSetting.TimeoutSeconds
	probe.FailureThreshold = probeSetting.FailureThreshold
	statusProbeContainer.ReadinessProbe = probe
	statusProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildRunningProbeContainer(characterType string, runningProbeContainer *corev1.Container,
	probeSetting *appsv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	runningProbeContainer.Name = constant.RunningProbeContainerName
	probe := &corev1.Probe{}
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(checkRunningURIFormat, characterType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	probe.TimeoutSeconds = probeSetting.TimeoutSeconds
	probe.FailureThreshold = probeSetting.FailureThreshold
	runningProbeContainer.ReadinessProbe = probe
	runningProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func volumeProtectionEnabled(component *SynthesizedComponent) bool {
	return component.VolumeProtection != nil
}

func buildVolumeProtectionProbeContainer(characterType string, c *corev1.Container, probeSvcHTTPPort int) {
	c.Name = constant.VolumeProtectionProbeContainerName
	probe := &corev1.Probe{}
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = fmt.Sprintf(volumeProtectionURIFormat, characterType)
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = defaultVolumeProtectionProbe.PeriodSeconds
	probe.TimeoutSeconds = defaultVolumeProtectionProbe.TimeoutSeconds
	probe.FailureThreshold = defaultVolumeProtectionProbe.FailureThreshold
	c.ReadinessProbe = probe
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
