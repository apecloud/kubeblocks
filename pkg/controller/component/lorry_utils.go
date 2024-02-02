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

// buildLorryContainers builds lorry containers for component.
// In the new ComponentDefinition API, StatusProbe and RunningProbe have been removed.
func buildLorryContainers(reqCtx intctrlutil.RequestCtx, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) error {
	// If it's not a built-in handler supported by Lorry, LorryContainers are not injected by default.
	builtinHandler := getBuiltinActionHandler(synthesizeComp)
	if builtinHandler == appsv1alpha1.UnknownBuiltinActionHandler {
		return nil
	}

	container := buildBasicContainer(synthesizeComp)
	var lorryContainers []corev1.Container
	lorryHTTPPort := viper.GetInt32(constant.KBEnvLorryHTTPPort)
	lorryGRPCPort := viper.GetInt32(constant.KBEnvLorryGRPCPort)
	if synthesizeComp.PodSpec.HostNetwork {
		lorryHTTPPort = 51
		lorryGRPCPort = 61
	}
	availablePorts, err := getAvailableContainerPorts(synthesizeComp.PodSpec.Containers, []int32{lorryHTTPPort, lorryGRPCPort})
	if err != nil {
		reqCtx.Log.Info("get lorry container port failed", "error", err)
		return err
	}
	lorryHTTPPort = availablePorts[0]
	lorryGRPCPort = availablePorts[1]
	if synthesizeComp.PodSpec.HostNetwork {
		if lorryGRPCPort >= 100 || lorryHTTPPort >= 100 {
			return fmt.Errorf("port numbers need to be less than 100 when using the host network! "+
				"lorry http port: %d, lorry grpc port: %d", lorryHTTPPort, lorryGRPCPort)
		}
	}

	// inject role probe container
	var compRoleProbe *appsv1alpha1.RoleProbe
	if synthesizeComp.LifecycleActions != nil {
		compRoleProbe = synthesizeComp.LifecycleActions.RoleProbe
	}
	if compRoleProbe != nil {
		reqCtx.Log.V(3).Info("lorry", "settings", compRoleProbe)
		roleChangedContainer := container.DeepCopy()
		buildRoleProbeContainer(roleChangedContainer, compRoleProbe, int(lorryHTTPPort))
		lorryContainers = append(lorryContainers, *roleChangedContainer)
	}

	// inject volume protection probe container
	if volumeProtectionEnabled(synthesizeComp) {
		c := container.DeepCopy()
		buildVolumeProtectionProbeContainer(synthesizeComp.CharacterType, c, int(lorryHTTPPort))
		lorryContainers = append(lorryContainers, *c)
	}

	if len(lorryContainers) == 0 {
		// need by other action handlers
		lorryContainer := container.DeepCopy()
		lorryContainers = append(lorryContainers, *lorryContainer)
	}

	buildLorryServiceContainer(synthesizeComp, &lorryContainers[0], int(lorryHTTPPort), int(lorryGRPCPort), clusterCompSpec)

	reqCtx.Log.V(1).Info("lorry", "containers", lorryContainers)
	synthesizeComp.PodSpec.Containers = append(synthesizeComp.PodSpec.Containers, lorryContainers...)
	return nil
}

func buildBasicContainer(synthesizeComp *SynthesizedComponent) *corev1.Container {
	return builder.NewContainerBuilder("string").
		SetImage("infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/google_containers/pause:3.6").
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands("/pause").
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(3501)},
			}}).
		GetObject()
}

func buildLorryServiceContainer(synthesizeComp *SynthesizedComponent, container *corev1.Container, lorryHTTPPort,
	lorryGRPCPort int, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) {
	container.Name = constant.LorryContainerName
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"lorry",
		"--port", strconv.Itoa(lorryHTTPPort),
		"--grpcport", strconv.Itoa(lorryGRPCPort),
	}

	container.Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(lorryHTTPPort),
			Name:          constant.LorryHTTPPortName,
			Protocol:      "TCP",
		},
		{
			ContainerPort: int32(lorryGRPCPort),
			Name:          constant.LorryGRPCPortName,
			Protocol:      "TCP",
		},
	}

	buildLorryEnvs(container, synthesizeComp, clusterCompSpec)
}

func buildLorryEnvs(container *corev1.Container, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) {
	var (
		secretName     string
		sysInitAccount *appsv1alpha1.SystemAccount
	)

	// TODO(lorry): use the buildIn kbprobe system account as the default credential
	for index, sysAccount := range synthesizeComp.SystemAccounts {
		if sysAccount.InitAccount {
			sysInitAccount = &synthesizeComp.SystemAccounts[index]
			break
		}
	}
	if clusterCompSpec == nil || clusterCompSpec.ComponentDef != "" {
		if sysInitAccount != nil {
			secretName = constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, sysInitAccount.Name)
		}
	} else {
		secretName = constant.GenerateDefaultConnCredential(synthesizeComp.ClusterName)
	}
	envs := []corev1.EnvVar{
		// inject the default built-in handler env to lorry container.
		{
			Name:      constant.KBEnvBuiltinHandler,
			Value:     string(getBuiltinActionHandler(synthesizeComp)),
			ValueFrom: nil,
		},
	}
	if secretName != "" {
		envs = append(envs,
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
	}

	if len(synthesizeComp.PodSpec.Containers) > 0 {
		mainContainer := synthesizeComp.PodSpec.Containers[0]
		if len(mainContainer.Ports) > 0 {
			port := mainContainer.Ports[0]
			dbPort := port.ContainerPort
			envs = append(envs, corev1.EnvVar{
				Name:      constant.KBEnvServicePort,
				Value:     strconv.Itoa(int(dbPort)),
				ValueFrom: nil,
			})
		}

		dataVolumeName := dataVolume
		for _, v := range synthesizeComp.Volumes {
			// TODO(xingran): how to convert needSnapshot to original volumeTypeData ?
			if v.NeedSnapshot {
				dataVolumeName = v.Name
			}
		}
		for _, volumeMount := range mainContainer.VolumeMounts {
			if volumeMount.Name != dataVolumeName {
				continue
			}
			vm := volumeMount.DeepCopy()
			container.VolumeMounts = []corev1.VolumeMount{*vm}
			envs = append(envs, corev1.EnvVar{
				Name:      constant.KBEnvDataPath,
				Value:     vm.MountPath,
				ValueFrom: nil,
			})
		}
	}

	// pass the volume protection spec to lorry container through env.
	// TODO(xingran & leon):  volume protection should be based on componentDefinition.Spec.Volume
	if volumeProtectionEnabled(synthesizeComp) {
		envs = append(envs, buildEnv4VolumeProtection(*synthesizeComp.VolumeProtection))
	}

	container.Env = append(container.Env, envs...)
}

func buildRoleProbeContainer(roleChangedContainer *corev1.Container, roleProbe *appsv1alpha1.RoleProbe, probeSvcHTTPPort int) {
	roleChangedContainer.Name = constant.RoleProbeContainerName
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = constant.LorryRoleProbePath
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe := &corev1.Probe{}
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = roleProbe.PeriodSeconds
	probe.TimeoutSeconds = roleProbe.TimeoutSeconds
	probe.FailureThreshold = roleProbe.FailureThreshold
	roleChangedContainer.ReadinessProbe = probe
	roleChangedContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func volumeProtectionEnabled(component *SynthesizedComponent) bool {
	return component.VolumeProtection != nil
}

func buildVolumeProtectionProbeContainer(characterType string, c *corev1.Container, probeSvcHTTPPort int) {
	c.Name = constant.VolumeProtectionProbeContainerName
	probe := &corev1.Probe{}
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = constant.LorryVolumeProtectPath
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = defaultVolumeProtectionProbe.PeriodSeconds
	probe.TimeoutSeconds = defaultVolumeProtectionProbe.TimeoutSeconds
	probe.FailureThreshold = defaultVolumeProtectionProbe.FailureThreshold
	c.ReadinessProbe = probe
	c.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildEnv4VolumeProtection(spec appsv1alpha1.VolumeProtectionSpec) corev1.EnvVar {
	value, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Sprintf("marshal volume protection spec error: %s", err.Error()))
	}
	return corev1.EnvVar{
		Name:  constant.KBEnvVolumeProtectionSpec,
		Value: string(value),
	}
}

// getBuiltinActionHandler gets the built-in handler.
// The BuiltinActionHandler within the same synthesizeComp LifecycleActions should be consistent, we can take any one of them.
func getBuiltinActionHandler(synthesizeComp *SynthesizedComponent) appsv1alpha1.BuiltinActionHandlerType {
	if synthesizeComp.LifecycleActions == nil {
		return appsv1alpha1.UnknownBuiltinActionHandler
	}

	if synthesizeComp.LifecycleActions.RoleProbe != nil {
		if synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil {
			return *synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler
		} else {
			return appsv1alpha1.CustomActionHandler
		}
	}

	actions := []*appsv1alpha1.LifecycleActionHandler{
		synthesizeComp.LifecycleActions.PostProvision,
		synthesizeComp.LifecycleActions.PreTerminate,
		synthesizeComp.LifecycleActions.MemberJoin,
		synthesizeComp.LifecycleActions.MemberLeave,
		synthesizeComp.LifecycleActions.Readonly,
		synthesizeComp.LifecycleActions.Readwrite,
		synthesizeComp.LifecycleActions.DataPopulate,
		synthesizeComp.LifecycleActions.DataAssemble,
		synthesizeComp.LifecycleActions.Reconfigure,
		synthesizeComp.LifecycleActions.AccountProvision,
	}

	hasAction := false
	for _, action := range actions {
		if action != nil {
			hasAction = true
			if action.BuiltinHandler != nil {
				return *action.BuiltinHandler
			}
		}
	}
	if hasAction {
		return appsv1alpha1.CustomActionHandler
	}
	return appsv1alpha1.UnknownBuiltinActionHandler
}
