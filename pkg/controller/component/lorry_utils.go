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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	dataVolume   = "data"
	minAvailPort = 1
	maxAvailPort = 65535
)

var (
	// default probe setting for volume protection.
	defaultVolumeProtectionProbe = appsv1alpha1.RoleProbe{
		PeriodSeconds:  60,
		TimeoutSeconds: 5,
	}
)

// buildLorryContainers builds lorry containers for component.
// In the new ComponentDefinition API, StatusProbe and RunningProbe have been removed.
func buildLorryContainers(reqCtx intctrlutil.RequestCtx, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) error {
	// If it's not a built-in handler supported by Lorry, LorryContainers are not injected by default.
	builtinHandler, execActionHandlers, grpcActionHandlers, err := getActionHandlers(synthesizeComp)
	if err != nil {
		return err
	}
	if builtinHandler == appsv1alpha1.UnknownBuiltinActionHandler && len(execActionHandlers) == 0 && len(grpcActionHandlers) == 0 {
		return nil
	}

	if builtinHandler != appsv1alpha1.UnknownBuiltinActionHandler && len(grpcActionHandlers) > 0 {
		// grpc handler is supported by kb-agent,
		// builtin handler is supported by lorry,
		// exec handler supported by lorry and kb-agent.
		return errors.New("builtin handler and grpc handler cannot be set at the same time")
	}

	if len(grpcActionHandlers) > 0 {
		return buildKBAgentContainer(reqCtx, synthesizeComp, clusterCompSpec)
	}
	return buildLorryContainer(reqCtx, synthesizeComp, clusterCompSpec)

}

func buildKBAgentContainer(reqCtx intctrlutil.RequestCtx, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) error {
	// inject kb-agent container
	container, err := buildKBAgentServiceContainer(synthesizeComp, clusterCompSpec)
	if err != nil {
		return err
	}
	err = adaptKBAgentIfExecHandlerDefined(synthesizeComp, container)
	if err != nil {
		return err
	}

	// inject role probe container
	var compRoleProbe *appsv1alpha1.RoleProbe
	if synthesizeComp.LifecycleActions != nil {
		compRoleProbe = synthesizeComp.LifecycleActions.RoleProbe
	}
	if compRoleProbe != nil {
		reqCtx.Log.V(3).Info("kb-agent", "role probe settings", compRoleProbe)
		buildRoleProbeContainer(container, compRoleProbe, int(container.Ports[0].ContainerPort))
	}

	reqCtx.Log.V(1).Info("kb-agent", "container", container)
	synthesizeComp.PodSpec.Containers = append(synthesizeComp.PodSpec.Containers, *container)
	return nil
}

func buildKBAgentServiceContainer(synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*corev1.Container, error) {
	kbAgentPort := viper.GetInt32(constant.KBEnvAgentPort)
	availablePorts, err := getAvailableContainerPorts(synthesizeComp.PodSpec.Containers, []int32{kbAgentPort})
	if err != nil {
		return nil, errors.Wrap(err, "get kb-agent container port failed")
	}
	kbAgentPort = availablePorts[0]
	container := buildBasicContainer(int(kbAgentPort))
	container.Name = constant.KBAgentContainerName
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"kb_agent",
		"--port", strconv.Itoa(int(kbAgentPort)),
	}

	container.Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(kbAgentPort),
			Name:          constant.KBAgentPortName,
			Protocol:      "TCP",
		},
	}

	buildEnvs(container, synthesizeComp, clusterCompSpec)

	// set kb-agent container ports to host network
	if synthesizeComp.HostNetwork != nil {
		if synthesizeComp.HostNetwork.ContainerPorts == nil {
			synthesizeComp.HostNetwork.ContainerPorts = make([]appsv1alpha1.HostNetworkContainerPort, 0)
		}
		synthesizeComp.HostNetwork.ContainerPorts = append(
			synthesizeComp.HostNetwork.ContainerPorts,
			appsv1alpha1.HostNetworkContainerPort{
				Container: container.Name,
				Ports:     []string{constant.KBAgentPortName},
			})
	}
	return container, nil
}
func adaptKBAgentIfExecHandlerDefined(synthesizeComp *SynthesizedComponent, kbAgentContainer *corev1.Container) error {
	_, execHandlers, grpcHandlers, _ := getActionHandlers(synthesizeComp)
	for name, handler := range execHandlers {
		grpcHandlers[name] = handler
	}

	handlerSetttings, err := getHandlerSettings(synthesizeComp, grpcHandlers)
	if err != nil {
		return err
	}

	handlersJSON, _ := json.Marshal(handlerSetttings)
	kbAgentContainer.Env = append(kbAgentContainer.Env, corev1.EnvVar{
		Name:  constant.KBEnvActionHandlers,
		Value: string(handlersJSON),
	})

	if len(execHandlers) == 0 {
		return nil
	}
	execImage, execContainer, err := getExecImageAndContainer(synthesizeComp, execHandlers)
	if err != nil {
		return err
	}

	if execImage == "" {
		return nil
	}
	initContainer := buildKBAgentInitContainer()
	synthesizeComp.PodSpec.InitContainers = append(synthesizeComp.PodSpec.InitContainers, *initContainer)
	kbAgentContainer.Image = execImage
	kbAgentContainer.VolumeMounts = append(kbAgentContainer.VolumeMounts, corev1.VolumeMount{Name: "kubeblocks", MountPath: "/kubeblocks"})
	if execContainer == nil {
		return nil
	}

	envSet := sets.New([]string{}...)
	for _, env := range kbAgentContainer.Env {
		envSet.Insert(env.Name)
	}

	for _, env := range execContainer.Env {
		if envSet.Has(env.Name) {
			continue
		}
		kbAgentContainer.Env = append(kbAgentContainer.Env, env)
	}
	return nil
}

func buildLorryContainer(reqCtx intctrlutil.RequestCtx, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) error {
	// inject lorry container
	var lorryContainers []corev1.Container
	lorryHTTPPort := viper.GetInt32(constant.KBEnvLorryHTTPPort)
	lorryGRPCPort := viper.GetInt32(constant.KBEnvLorryGRPCPort)
	availablePorts, err := getAvailableContainerPorts(synthesizeComp.PodSpec.Containers, []int32{lorryHTTPPort, lorryGRPCPort})
	if err != nil {
		reqCtx.Log.Info("get lorry container port failed", "error", err)
		return err
	}
	lorryHTTPPort = availablePorts[0]
	lorryGRPCPort = availablePorts[1]
	container := buildBasicContainer(int(lorryHTTPPort))
	// inject role probe container
	var compRoleProbe *appsv1alpha1.RoleProbe
	if synthesizeComp.LifecycleActions != nil {
		compRoleProbe = synthesizeComp.LifecycleActions.RoleProbe
	}
	if compRoleProbe != nil {
		reqCtx.Log.V(3).Info("lorry", "role probe settings", compRoleProbe)
		roleChangedContainer := container.DeepCopy()
		buildRoleProbeContainer(roleChangedContainer, compRoleProbe, int(lorryHTTPPort))
		lorryContainers = append(lorryContainers, *roleChangedContainer)
	}

	// inject volume protection probe container
	if volumeProtectionEnabled(synthesizeComp) {
		c := container.DeepCopy()
		buildVolumeProtectionProbeContainer(c, int(lorryHTTPPort))
		lorryContainers = append(lorryContainers, *c)
	}

	if len(lorryContainers) == 0 {
		// need by other action handlers
		lorryContainer := container.DeepCopy()
		lorryContainers = append(lorryContainers, *lorryContainer)
	}

	buildLorryServiceContainer(synthesizeComp, &lorryContainers[0], int(lorryHTTPPort), int(lorryGRPCPort), clusterCompSpec)
	err = adaptLorryIfCustomHandlerDefined(synthesizeComp, &lorryContainers[0], int(lorryHTTPPort), int(lorryGRPCPort))
	if err != nil {
		return err
	}

	reqCtx.Log.V(1).Info("lorry", "containers", lorryContainers)
	synthesizeComp.PodSpec.Containers = append(synthesizeComp.PodSpec.Containers, lorryContainers...)

	return nil
}

func adaptLorryIfCustomHandlerDefined(synthesizeComp *SynthesizedComponent, lorryContainer *corev1.Container,
	lorryHTTPPort, lorryGRPCPort int) error {
	_, execHandlers, _, _ := getActionHandlers(synthesizeComp)
	if len(execHandlers) == 0 {
		return nil
	}
	execImage, execContainer, err := getExecImageAndContainer(synthesizeComp, execHandlers)
	if err != nil {
		return err
	}

	handlerSetttings, err := getHandlerSettings(synthesizeComp, execHandlers)
	if err != nil {
		return err
	}

	handlersJSON, _ := json.Marshal(handlerSetttings)
	lorryContainer.Env = append(lorryContainer.Env, corev1.EnvVar{
		Name:  constant.KBEnvActionHandlers,
		Value: string(handlersJSON),
	})

	if execImage == "" {
		return nil
	}
	initContainer := buildLorryInitContainer()
	synthesizeComp.PodSpec.InitContainers = append(synthesizeComp.PodSpec.InitContainers, *initContainer)
	lorryContainer.Image = execImage
	lorryContainer.VolumeMounts = append(lorryContainer.VolumeMounts, corev1.VolumeMount{Name: "kubeblocks", MountPath: "/kubeblocks"})
	lorryContainer.Command = []string{"/kubeblocks/lorry",
		"--port", strconv.Itoa(lorryHTTPPort),
		"--grpcport", strconv.Itoa(lorryGRPCPort),
		"--config-path", "/kubeblocks/config/lorry/components/",
	}
	if execContainer == nil {
		return nil
	}

	envSet := sets.New([]string{}...)
	for _, env := range lorryContainer.Env {
		envSet.Insert(env.Name)
	}

	for _, env := range execContainer.Env {
		if envSet.Has(env.Name) {
			continue
		}
		lorryContainer.Env = append(lorryContainer.Env, env)
	}
	return nil
}

func buildBasicContainer(lorryHTTPPort int) *corev1.Container {
	return builder.NewContainerBuilder("string").
		SetImage("infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/google_containers/pause:3.6").
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands("/pause").
		SetStartupProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(lorryHTTPPort)},
			}}).
		GetObject()
}

func buildLorryServiceContainer(synthesizeComp *SynthesizedComponent, container *corev1.Container,
	lorryHTTPPort, lorryGRPCPort int, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) {
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

	container.Env = append(container.Env, corev1.EnvVar{
		Name:      constant.KBEnvBuiltinHandler,
		Value:     string(getBuiltinActionHandler(synthesizeComp)),
		ValueFrom: nil,
	})
	buildEnvs(container, synthesizeComp, clusterCompSpec)

	// set lorry container ports to host network
	if synthesizeComp.HostNetwork != nil {
		if synthesizeComp.HostNetwork.ContainerPorts == nil {
			synthesizeComp.HostNetwork.ContainerPorts = make([]appsv1alpha1.HostNetworkContainerPort, 0)
		}
		synthesizeComp.HostNetwork.ContainerPorts = append(
			synthesizeComp.HostNetwork.ContainerPorts,
			appsv1alpha1.HostNetworkContainerPort{
				Container: container.Name,
				Ports:     []string{constant.LorryHTTPPortName, constant.LorryGRPCPortName},
			})
	}
}

func buildLorryInitContainer() *corev1.Container {
	container := &corev1.Container{}
	container.Image = viper.GetString(constant.KBToolsImage)
	container.Name = constant.LorryInitContainerName
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"cp", "-r", "/bin/lorry", "/config", "/kubeblocks/"}
	container.StartupProbe = nil
	container.ReadinessProbe = nil
	volumeMount := corev1.VolumeMount{Name: "kubeblocks", MountPath: "/kubeblocks"}
	container.VolumeMounts = []corev1.VolumeMount{volumeMount}
	return container
}

func buildKBAgentInitContainer() *corev1.Container {
	container := &corev1.Container{}
	container.Image = viper.GetString(constant.KBToolsImage)
	container.Name = constant.KBAgentInitContainerName
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Command = []string{"cp", "-r", "/bin/kb_agent", "/kubeblocks/"}
	container.StartupProbe = nil
	container.ReadinessProbe = nil
	volumeMount := corev1.VolumeMount{Name: "kubeblocks", MountPath: "/kubeblocks"}
	container.VolumeMounts = []corev1.VolumeMount{volumeMount}
	return container
}

func buildEnvs(container *corev1.Container, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) {
	envs := []corev1.EnvVar{}

	envs = append(envs, buildEnv4DBAccount(synthesizeComp, clusterCompSpec)...)

	mainContainer := getMainContainer(synthesizeComp.PodSpec.Containers)
	if mainContainer != nil {
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

	if volumeProtectionEnabled(synthesizeComp) {
		envs = append(envs, buildEnv4VolumeProtection(synthesizeComp))
	}
	envs = append(envs, buildEnv4CronJobs(synthesizeComp)...)

	container.Env = append(container.Env, envs...)
}

func buildRoleProbeContainer(roleChangedContainer *corev1.Container, roleProbe *appsv1alpha1.RoleProbe, probeSvcHTTPPort int) {
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = constant.LorryRoleProbePath
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe := &corev1.Probe{}
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = roleProbe.PeriodSeconds
	probe.TimeoutSeconds = roleProbe.TimeoutSeconds
	probe.FailureThreshold = 3
	roleChangedContainer.ReadinessProbe = probe
	roleChangedContainer.Env = append(roleChangedContainer.Env, corev1.EnvVar{
		Name:  constant.KBEnvRoleProbePeriod,
		Value: strconv.Itoa(int(roleProbe.PeriodSeconds)),
	})
}

func volumeProtectionEnabled(component *SynthesizedComponent) bool {
	for _, v := range component.Volumes {
		if v.HighWatermark > 0 {
			return true
		}
	}
	return false
}

func buildVolumeProtectionProbeContainer(c *corev1.Container, probeSvcHTTPPort int) {
	c.Name = constant.VolumeProtectionProbeContainerName
	probe := &corev1.Probe{}
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = constant.LorryVolumeProtectPath
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = defaultVolumeProtectionProbe.PeriodSeconds
	probe.TimeoutSeconds = defaultVolumeProtectionProbe.TimeoutSeconds
	probe.FailureThreshold = 3
	c.ReadinessProbe = probe
}

func buildEnv4DBAccount(synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) []corev1.EnvVar {
	var (
		secretName     string
		sysInitAccount *appsv1alpha1.SystemAccount
	)

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
	envs := []corev1.EnvVar{}
	if secretName == "" {
		return envs
	}

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
	return envs
}

func buildEnv4VolumeProtection(synthesizedComp *SynthesizedComponent) corev1.EnvVar {
	spec := &appsv1alpha1.VolumeProtectionSpec{}
	for i, v := range synthesizedComp.Volumes {
		if v.HighWatermark > 0 {
			spec.Volumes = append(spec.Volumes, appsv1alpha1.ProtectedVolume{
				Name:          v.Name,
				HighWatermark: &synthesizedComp.Volumes[i].HighWatermark,
			})
		}
	}

	value, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Sprintf("marshal volume protection spec error: %s", err.Error()))
	}
	return corev1.EnvVar{
		Name:  constant.KBEnvVolumeProtectionSpec,
		Value: string(value),
	}
}

func buildEnv4CronJobs(_ *SynthesizedComponent) []corev1.EnvVar {
	return nil
	// if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.HealthyCheck == nil {
	// 	return nil
	// }
	// healthyCheck := synthesizeComp.LifecycleActions.HealthyCheck
	// healthCheckSetting := make(map[string]string)
	// healthCheckSetting["periodSeconds"] = strconv.Itoa(int(healthyCheck.PeriodSeconds))
	// healthCheckSetting["timeoutSeconds"] = strconv.Itoa(int(healthyCheck.TimeoutSeconds))
	// healthCheckSetting["failureThreshold"] = strconv.Itoa(int(healthyCheck.FailureThreshold))
	// healthCheckSetting["successThreshold"] = strconv.Itoa(int(healthyCheck.SuccessThreshold))
	// cronJobs := make(map[string]map[string]string)
	// cronJobs["healthyCheck"] = healthCheckSetting

	// jsonStr, _ := json.Marshal(cronJobs)
	// return []corev1.EnvVar{
	// 	{
	// 		Name:  constant.KBEnvCronJobs,
	// 		Value: string(jsonStr),
	// 	},
	// }
}

// getBuiltinActionHandler gets the built-in handler.
// The BuiltinActionHandler within the same synthesizeComp LifecycleActions should be consistent, we can take any one of them.
func getBuiltinActionHandler(synthesizeComp *SynthesizedComponent) appsv1alpha1.BuiltinActionHandlerType {
	builtinHandler, _, _, _ := getActionHandlers(synthesizeComp)
	if builtinHandler == appsv1alpha1.UnknownBuiltinActionHandler {
		builtinHandler = appsv1alpha1.CustomActionHandler
	}
	return builtinHandler
}

func getActionHandlers(synthesizeComp *SynthesizedComponent) (appsv1alpha1.BuiltinActionHandlerType,
	map[string]*appsv1alpha1.LifecycleActionHandler, map[string]*appsv1alpha1.LifecycleActionHandler, error) {
	builtinHandler := appsv1alpha1.UnknownBuiltinActionHandler
	if synthesizeComp.LifecycleActions == nil {
		return builtinHandler, nil, nil, nil
	}

	actionHandlers := map[string]*appsv1alpha1.LifecycleActionHandler{
		constant.PostProvisionAction: synthesizeComp.LifecycleActions.PostProvision,
		constant.PreTerminateAction:  synthesizeComp.LifecycleActions.PreTerminate,
		constant.MemberJoinAction:    synthesizeComp.LifecycleActions.MemberJoin,
		constant.MemberLeaveAction:   synthesizeComp.LifecycleActions.MemberLeave,
		constant.ReadonlyAction:      synthesizeComp.LifecycleActions.Readonly,
		constant.ReadWriteAction:     synthesizeComp.LifecycleActions.Readwrite,
		constant.DataDumpAction:      synthesizeComp.LifecycleActions.DataDump,
		constant.DataLoadAction:      synthesizeComp.LifecycleActions.DataLoad,
		// "reconfigure":                synthesizeComp.LifecycleActions.Reconfigure,
		// "accountProvision": synthesizeComp.LifecycleActions.AccountProvision,
	}

	if synthesizeComp.LifecycleActions.RoleProbe != nil {
		actionHandlers[constant.RoleProbeAction] = &synthesizeComp.LifecycleActions.RoleProbe.LifecycleActionHandler
	}

	execActionHandlers := map[string]*appsv1alpha1.LifecycleActionHandler{}
	grpcActionHandlers := map[string]*appsv1alpha1.LifecycleActionHandler{}
	for name, handler := range actionHandlers {
		if handler == nil {
			continue
		}

		if handler.BuiltinHandler != nil &&
			*handler.BuiltinHandler != appsv1alpha1.CustomActionHandler &&
			*handler.BuiltinHandler != appsv1alpha1.UnknownBuiltinActionHandler {
			if builtinHandler == appsv1alpha1.UnknownBuiltinActionHandler {
				builtinHandler = *handler.BuiltinHandler
			}
			if builtinHandler != *handler.BuiltinHandler {
				err := fmt.Errorf("builtin handler conflict: %s, %s", builtinHandler, *handler.BuiltinHandler)
				return appsv1alpha1.UnknownBuiltinActionHandler, nil, nil, err
			}
		}

		if handler.CustomHandler != nil {
			if handler.CustomHandler.Exec != nil {
				execActionHandlers[name] = handler
			} else if handler.CustomHandler.GRPC != nil {
				grpcActionHandlers[name] = handler
			}
		}
	}
	return builtinHandler, execActionHandlers, grpcActionHandlers, nil
}

func getExecImageAndContainer(synthesizeComp *SynthesizedComponent, execHandlers map[string]*appsv1alpha1.LifecycleActionHandler) (string, *corev1.Container, error) {
	var toolImage string
	var containerName string

	hasCommand := false
	for _, handler := range execHandlers {
		if handler == nil || handler.CustomHandler == nil {
			continue
		}

		handlers := util.Handlers{}
		if handler.CustomHandler.Exec != nil {
			hasCommand = true
			handlers.Command = handler.CustomHandler.Exec.Command
			if handler.CustomHandler.Image != "" {
				if toolImage == "" {
					toolImage = handler.CustomHandler.Image
				}
				if toolImage != handler.CustomHandler.Image {
					return "", nil, errors.New("exec image conflict")
				}
			}
			if handler.CustomHandler.Container != "" {
				if containerName == "" {
					containerName = handler.CustomHandler.Container
				}
				if containerName != handler.CustomHandler.Container {
					return "", nil, errors.New("exec container conflict")
				}
			}
		}
	}

	if !hasCommand {
		return "", nil, nil
	}

	execContainer := getExecContainer(synthesizeComp.PodSpec.Containers, containerName)
	if toolImage == "" {
		if execContainer == nil {
			return "", nil, errors.New("no exec container found")
		}
		toolImage = execContainer.Image
	}
	return toolImage, execContainer, nil
}

func getHandlerSettings(synthesizeComp *SynthesizedComponent, actionHandlers map[string]*appsv1alpha1.LifecycleActionHandler) (map[string]util.Handlers, error) {
	handlerSettings := map[string]util.Handlers{}
	for name, handler := range actionHandlers {
		if handler == nil || handler.CustomHandler == nil {
			continue
		}

		handlers := util.Handlers{}
		if handler.CustomHandler.Exec != nil {
			handlers.Command = handler.CustomHandler.Exec.Command
		} else if handler.CustomHandler.GRPC != nil {
			port, err := getPort(synthesizeComp.PodSpec.Containers, handler.CustomHandler.GRPC.Port)
			if err != nil {
				return nil, err
			}
			handlers.GPRC = map[string]string{}
			handlers.GPRC["port"] = strconv.Itoa(int(port))
			if handler.CustomHandler.GRPC.Host != "" {
				handlers.GPRC["host"] = handler.CustomHandler.GRPC.Host
			}
		}
		handlerSettings[name] = handlers
	}
	return handlerSettings, nil
}

func getPort(containers []corev1.Container, port intstr.IntOrString) (int32, error) {
	var portValue int32
	var err error
	if port.Type == intstr.Int {
		portValue = port.IntVal
	} else {
		portValue, err = intctrlutil.GetPortByPortName(containers, port.StrVal)
		if err != nil {
			return 0, err
		}
	}

	if portValue < 1 || portValue > 65535 {
		return 0, fmt.Errorf("invalid port value: %d", portValue)
	}
	return portValue, nil
}

func getExecContainer(containers []corev1.Container, name string) *corev1.Container {
	if name == "" {
		return getMainContainer(containers)
	}

	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func getMainContainer(containers []corev1.Container) *corev1.Container {
	if len(containers) > 0 {
		return &containers[0]
	}
	return nil
}

// get available container ports, increased by one if conflict with exist ports
// util no conflicts.
func getAvailableContainerPorts(containers []corev1.Container, containerPorts []int32) ([]int32, error) {
	set, err := getAllContainerPorts(containers)
	if err != nil {
		return nil, err
	}

	iterAvailPort := func(p int32) (int32, error) {
		// The TCP/IP port numbers below 1024 are privileged ports, which are special
		// in that normal users are not allowed to run servers on them.
		// Ports below 1024 can be allocated, as the port manager will automatically reallocate ports under 100.
		if p < minAvailPort || p > maxAvailPort {
			p = minAvailPort
		}
		sentinel := p
		for {
			if _, ok := set[p]; !ok {
				set[p] = true
				return p, nil
			}
			p++
			if p == sentinel {
				return -1, errors.New("no available port for container")
			}
			if p > maxAvailPort {
				p = minAvailPort
			}
		}
	}

	for i, p := range containerPorts {
		if containerPorts[i], err = iterAvailPort(p); err != nil {
			return []int32{}, err
		}
	}
	return containerPorts, nil
}

func getAllContainerPorts(containers []corev1.Container) (map[int32]bool, error) {
	set := map[int32]bool{}
	for _, container := range containers {
		for _, v := range container.Ports {
			_, ok := set[v.ContainerPort]
			if ok {
				return nil, fmt.Errorf("containerPorts conflict: [%+v]", v.ContainerPort)
			}
			set[v.ContainerPort] = true
		}
	}
	return set, nil
}
