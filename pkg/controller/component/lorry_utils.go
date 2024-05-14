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
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	dataVolume = "data"
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
	builtinHandler := getBuiltinActionHandler(synthesizeComp)
	if builtinHandler == appsv1alpha1.UnknownBuiltinActionHandler {
		return nil
	}

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
	adaptLorryIfCustomHandlerDefined(synthesizeComp, &lorryContainers[0], int(lorryHTTPPort), int(lorryGRPCPort))

	reqCtx.Log.V(1).Info("lorry", "containers", lorryContainers)
	synthesizeComp.PodSpec.Containers = append(synthesizeComp.PodSpec.Containers, lorryContainers...)

	return nil
}

func adaptLorryIfCustomHandlerDefined(synthesizeComp *SynthesizedComponent, lorryContainer *corev1.Container,
	lorryHTTPPort, lorryGRPCPort int) {
	actions, execImage, execContainer := getActionsWithExecImageOrContainer(synthesizeComp)
	if len(actions) == 0 {
		return
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
	actionJSON, _ := json.Marshal(actions)
	lorryContainer.Env = append(lorryContainer.Env, corev1.EnvVar{
		Name:  constant.KBEnvActionHandlers,
		Value: string(actionJSON),
	})

	if execContainer == nil {
		return
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

	buildLorryEnvs(container, synthesizeComp, clusterCompSpec)
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

func buildLorryEnvs(container *corev1.Container, synthesizeComp *SynthesizedComponent, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) {
	envs := []corev1.EnvVar{
		// inject the default built-in handler env to lorry container.
		{
			Name:      constant.KBEnvBuiltinHandler,
			Value:     string(getBuiltinActionHandler(synthesizeComp)),
			ValueFrom: nil,
		},
	}

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
	roleChangedContainer.Name = constant.RoleProbeContainerName
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
	if synthesizeComp.LifecycleActions == nil {
		return appsv1alpha1.UnknownBuiltinActionHandler
	}

	if synthesizeComp.LifecycleActions.RoleProbe != nil {
		if synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil &&
			*synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != appsv1alpha1.UnknownBuiltinActionHandler {
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
		synthesizeComp.LifecycleActions.DataDump,
		synthesizeComp.LifecycleActions.DataLoad,
		synthesizeComp.LifecycleActions.Reconfigure,
		// synthesizeComp.LifecycleActions.AccountProvision,
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

func getActionsWithExecImageOrContainer(synthesizeComp *SynthesizedComponent) (map[string]util.Handlers, string, *corev1.Container) {
	if synthesizeComp.LifecycleActions == nil {
		return nil, "", nil
	}

	actions := map[string]*appsv1alpha1.LifecycleActionHandler{
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
		actions[constant.RoleProbeAction] = &synthesizeComp.LifecycleActions.RoleProbe.LifecycleActionHandler
	}

	var toolImage string
	var containerName string
	actionNames := []string{}
	for name := range actions {
		actionNames = append(actionNames, name)
	}
	sort.Strings(actionNames)

	hasCommand := false
	actionHandlers := map[string]util.Handlers{}
	for _, name := range actionNames {
		handler := actions[name]
		if handler == nil || handler.CustomHandler == nil {
			continue
		}

		handlers := util.Handlers{}
		if handler.CustomHandler.Exec != nil {
			hasCommand = true
			handlers.Command = handler.CustomHandler.Exec.Command
			if handler.CustomHandler.Image != "" {
				toolImage = handler.CustomHandler.Image
			}
			if handler.CustomHandler.Container != "" {
				containerName = handler.CustomHandler.Container
			}
		} else if handler.CustomHandler.GRPC != nil {
			handlers.GPRC = map[string]string{}
			handlers.GPRC["host"] = handler.CustomHandler.GRPC.Host
			handlers.GPRC["port"] = handler.CustomHandler.GRPC.Port
			handlers.GPRC["service"] = handler.CustomHandler.GRPC.Service
		}
		actionHandlers[name] = handlers
	}
	if !hasCommand {
		return actionHandlers, "", nil
	}

	execContainer := getExecContainer(synthesizeComp.PodSpec.Containers, containerName)
	if toolImage == "" {
		if execContainer == nil {
			return actionHandlers, "", nil
		}
		toolImage = execContainer.Image
	}
	return actionHandlers, toolImage, execContainer
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
