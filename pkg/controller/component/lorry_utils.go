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

	"golang.org/x/exp/slices"
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

// buildLorryContainers builds lorry containers for component.
// In the new ComponentDefinition API, StatusProbe and RunningProbe have been removed.
func buildLorryContainers(reqCtx intctrlutil.RequestCtx, synthesizeComp *SynthesizedComponent) error {
	// If it's not a built-in handler supported by Lorry, LorryContainers are not injected by default.
	builtinHandler := getBuiltinActionHandler(synthesizeComp)
	if builtinHandler == appsv1alpha1.UnknownBuiltinActionHandler {
		return nil
	}

	container := buildBasicContainer(synthesizeComp)
	var lorryContainers []corev1.Container
	lorrySvcHTTPPort := viper.GetInt32("PROBE_SERVICE_HTTP_PORT")
	// override by new env name
	if viper.IsSet("LORRY_SERVICE_HTTP_PORT") {
		lorrySvcHTTPPort = viper.GetInt32("LORRY_SERVICE_HTTP_PORT")
	}
	availablePorts, err := getAvailableContainerPorts(synthesizeComp.PodSpec.Containers, []int32{lorrySvcHTTPPort})
	lorrySvcHTTPPort = availablePorts[0]
	if err != nil {
		reqCtx.Log.Info("get lorry container port failed", "error", err)
		return err
	}
	lorrySvcGRPCPort := viper.GetInt("PROBE_SERVICE_GRPC_PORT")

	// inject role probe container
	var compRoleProbe *appsv1alpha1.RoleProbeSpec
	if synthesizeComp.LifecycleActions != nil {
		compRoleProbe = synthesizeComp.LifecycleActions.RoleProbe
	}
	if compRoleProbe != nil {
		reqCtx.Log.V(3).Info("lorry", "settings", compRoleProbe)
		roleChangedContainer := container.DeepCopy()
		buildRoleProbeContainer(roleChangedContainer, compRoleProbe, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *roleChangedContainer)
	}

	// inject volume protection probe container
	if volumeProtectionEnabled(synthesizeComp) {
		c := container.DeepCopy()
		buildVolumeProtectionProbeContainer(synthesizeComp.CharacterType, c, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *c)
	}

	// inject WeSyncer(currently part of lorry) in cluster controller.
	// as all the above features share the lorry service, only one lorry need to be injected.
	// if none of the above feature enabled, WeSyncer still need to be injected for the HA feature functions well.
	if len(lorryContainers) == 0 && isSupportWeSyncer(synthesizeComp) {
		weSyncerContainer := container.DeepCopy()
		buildWeSyncerContainer(weSyncerContainer, int(lorrySvcHTTPPort))
		lorryContainers = append(lorryContainers, *weSyncerContainer)
	}

	if len(lorryContainers) == 0 {
		return nil
	}

	buildLorryServiceContainer(synthesizeComp, &lorryContainers[0], int(lorrySvcHTTPPort), lorrySvcGRPCPort)

	reqCtx.Log.V(1).Info("lorry", "containers", lorryContainers)
	synthesizeComp.PodSpec.Containers = append(synthesizeComp.PodSpec.Containers, lorryContainers...)
	return nil
}

func buildBasicContainer(synthesizeComp *SynthesizedComponent) *corev1.Container {
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
	if sysInitAccount != nil {
		secretName = constant.GenerateComponentConnCredential(synthesizeComp.ClusterName, synthesizeComp.Name, sysInitAccount.Name)
	} else {
		secretName = constant.GenerateDefaultConnCredential(synthesizeComp.ClusterName)
	}
	return builder.NewContainerBuilder("string").
		SetImage("infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/google_containers/pause:3.6").
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands("/pause").
		AddEnv(corev1.EnvVar{
			Name: constant.KBEnvServiceUser,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: constant.AccountNameForSecret,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					}},
			}},
			corev1.EnvVar{
				Name: constant.KBEnvServicePassword,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: constant.AccountPasswdForSecret,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						}},
				},
			}).
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(3501)},
			}}).
		GetObject()
}

func buildLorryServiceContainer(synthesizeComp *SynthesizedComponent, container *corev1.Container, lorrySvcHTTPPort, lorrySvcGRPCPort int) {
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"lorry",
		"--port", strconv.Itoa(lorrySvcHTTPPort),
		"--config-path", "/config/lorry/components/",
		"--grpcport", strconv.Itoa(lorrySvcGRPCPort),
	}

	if len(synthesizeComp.PodSpec.Containers) > 0 {
		mainContainer := synthesizeComp.PodSpec.Containers[0]
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
			container.Env = append(container.Env, corev1.EnvVar{
				Name:      constant.KBEnvDataPath,
				Value:     vm.MountPath,
				ValueFrom: nil,
			})
		}
	}

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
	if sysInitAccount != nil {
		secretName = constant.GenerateComponentConnCredential(synthesizeComp.ClusterName, synthesizeComp.Name, sysInitAccount.Name)
	} else {
		secretName = constant.GenerateDefaultConnCredential(synthesizeComp.ClusterName)
	}
	container.Env = append(container.Env,
		// inject the default built-in handler env to lorry container.
		corev1.EnvVar{
			Name:      constant.KBEnvBuiltinHandler,
			Value:     string(getBuiltinActionHandler(synthesizeComp)),
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
	// TODO(xingran & leon):  volume protection should be based on componentDefinition.Spec.Volume
	if volumeProtectionEnabled(synthesizeComp) {
		container.Env = append(container.Env, env4VolumeProtection(*synthesizeComp.VolumeProtection))
	}
}

func buildWeSyncerContainer(weSyncerContainer *corev1.Container, probeSvcHTTPPort int) {
	weSyncerContainer.Name = constant.WeSyncerContainerName
	weSyncerContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildRoleProbeContainer(roleChangedContainer *corev1.Container, roleProbe *appsv1alpha1.RoleProbeSpec, probeSvcHTTPPort int) {
	roleChangedContainer.Name = constant.RoleProbeContainerName
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = checkRoleURIFormat
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

// getBuiltinActionHandler gets the built-in handler.
// The BuiltinActionHandler within the same synthesizeComp LifecycleActions should be consistent, we can take any one of them.
func getBuiltinActionHandler(synthesizeComp *SynthesizedComponent) appsv1alpha1.BuiltinActionHandlerType {
	if synthesizeComp.LifecycleActions == nil {
		return appsv1alpha1.UnknownBuiltinActionHandler
	}

	if synthesizeComp.LifecycleActions.RoleProbe != nil && synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil {
		return *synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler
	}

	actions := []struct {
		LifeCycleActionHandlers *appsv1alpha1.LifecycleActionHandler
	}{
		{synthesizeComp.LifecycleActions.PostStart},
		{synthesizeComp.LifecycleActions.PreStop},
		{synthesizeComp.LifecycleActions.MemberJoin},
		{synthesizeComp.LifecycleActions.MemberLeave},
		{synthesizeComp.LifecycleActions.Readonly},
		{synthesizeComp.LifecycleActions.Readwrite},
		{synthesizeComp.LifecycleActions.DataPopulate},
		{synthesizeComp.LifecycleActions.DataAssemble},
		{synthesizeComp.LifecycleActions.Reconfigure},
		{synthesizeComp.LifecycleActions.AccountProvision},
	}

	for _, action := range actions {
		if action.LifeCycleActionHandlers != nil && action.LifeCycleActionHandlers.BuiltinHandler != nil {
			return *action.LifeCycleActionHandlers.BuiltinHandler
		}
	}

	return appsv1alpha1.UnknownBuiltinActionHandler
}

// isSupportWeSyncer checks if we need to inject a kb-we-syncer container
// TODO(xingran&xuanchi): which needs to be refactored
func isSupportWeSyncer(synthesizeComp *SynthesizedComponent) bool {
	if synthesizeComp.CharacterType == "" {
		return false
	}

	if !slices.Contains(constant.GetSupportWeSyncerType(), synthesizeComp.CharacterType) {
		return false
	}

	return true
}
