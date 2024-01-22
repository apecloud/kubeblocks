package component

import (
	"context"
	"fmt"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/lorry/util/kubernetes"
)

const (
	toolsVolumeName                  = "kb-tools"
	initSecRenderedToolContainerName = "init-secondary-rendered-tool"

	tplRenderToolPath = "/bin/config_render"
)

// BuildConfigManagerWithComponentForLorry inject the config manager service into Lorry container if configuration reload option is on
func BuildConfigManagerWithComponentForLorry(container *corev1.Container, ctx context.Context, cluster *appsv1alpha1.Cluster, synthesizedComp *SynthesizedComponent) error {
	var buildParams *cfgcm.CfgManagerBuildParams

	cli, err := kubernetes.GetControllerRuntimeClient()
	if err != nil {
		return err
	}

	volumeDirs, usingConfigSpecs := getUsingVolumesByConfigSpecs(synthesizedComp.PodSpec, synthesizedComp.ConfigTemplates)
	if len(volumeDirs) == 0 {
		return nil
	}
	configSpecMetas, err := cfgcm.GetSupportReloadConfigSpecs(usingConfigSpecs, cli, ctx)
	if err != nil {
		return err
	}
	// Configmap uses subPath case: https://github.com/kubernetes/kubernetes/issues/50345
	// The files are being updated on the host VM, but can't be updated in the container.
	configSpecMetas = cfgcm.FilterSubPathVolumeMount(configSpecMetas, volumeDirs)
	if len(configSpecMetas) == 0 {
		return nil
	}

	if buildParams, err = buildLorryConfigManagerParams(cli, ctx, cluster, synthesizedComp, configSpecMetas, volumeDirs, synthesizedComp.PodSpec); err != nil {
		return err
	}
	if buildParams == nil {
		return nil
	}

	// This sidecar container will be able to view and signal processes from other containers
	checkAndUpdateShareProcessNamespace(synthesizedComp.PodSpec, buildParams, configSpecMetas)

	// for lorry
	buildConfigManagerForLorryContainer(container, buildParams, synthesizedComp)

	updateEnvPath(container, buildParams)
	updateCfgManagerVolumes(synthesizedComp.PodSpec, buildParams)
	if len(buildParams.ToolsContainers) > 0 {
		synthesizedComp.PodSpec.InitContainers = append(synthesizedComp.PodSpec.InitContainers, buildParams.ToolsContainers...)
	}

	filter := func(c *corev1.Container) bool {
		names := []string{container.Name}
		for _, cc := range buildParams.ToolsContainers {
			names = append(names, cc.Name)
		}
		return slices.Contains(names, c.Name)
	}
	InjectEnvVars4Containers(synthesizedComp, synthesizedComp.EnvVars, synthesizedComp.EnvFromSources, filter)
	return nil
}

func getUsingVolumesByConfigSpecs(podSpec *corev1.PodSpec, configSpecs []appsv1alpha1.ComponentConfigSpec) ([]corev1.VolumeMount, []appsv1alpha1.ComponentConfigSpec) {
	// Ignore useless configTemplate
	usingConfigSpecs := make([]appsv1alpha1.ComponentConfigSpec, 0, len(configSpecs))
	config2Containers := make(map[string][]*corev1.Container)
	for _, configSpec := range configSpecs {
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, configSpec.VolumeName)
		if len(usingContainers) == 0 {
			continue
		}
		usingConfigSpecs = append(usingConfigSpecs, configSpec)
		config2Containers[configSpec.Name] = usingContainers
	}

	// No container using any config template
	if len(usingConfigSpecs) == 0 {
		log.Log.Info(fmt.Sprintf("configSpec config is not used by any container, and pass. configSpec configs: %v", configSpecs))
		return nil, nil
	}

	// Find out which configurations are used by the container
	volumeDirs := make([]corev1.VolumeMount, 0, len(configSpecs)+1)
	for _, configSpec := range usingConfigSpecs {
		// Ignore config template, e.g scripts configmap
		if !core.NeedReloadVolume(configSpec) {
			continue
		}
		sets := cfgutil.NewSet()
		for _, container := range config2Containers[configSpec.Name] {
			volume := intctrlutil.GetVolumeMountByVolume(container, configSpec.VolumeName)
			if volume != nil && !sets.InArray(volume.Name) {
				volumeDirs = append(volumeDirs, *volume)
				sets.Add(volume.Name)
			}
		}
	}
	return volumeDirs, usingConfigSpecs
}

func buildLorryConfigManagerParams(cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster, comp *SynthesizedComponent, configSpecBuildParams []cfgcm.ConfigSpecMeta, volumeDirs []corev1.VolumeMount, podSpec *corev1.PodSpec) (*cfgcm.CfgManagerBuildParams, error) {
	cfgManagerParams := &cfgcm.CfgManagerBuildParams{
		// ManagerName:               constant.ConfigSidecarName,
		CharacterType: comp.CharacterType,
		ComponentName: comp.Name,
		SecreteName:   constant.GenerateDefaultConnCredential(cluster.Name),
		// Image:                     viper.GetString(constant.KBToolsImage),
		Volumes:                   volumeDirs,
		Cluster:                   cluster,
		ConfigSpecsBuildParams:    configSpecBuildParams,
		ConfigLazyRenderedVolumes: make(map[string]corev1.VolumeMount),
		// lorry already has grpc port
		// ContainerPort:             viper.GetInt32(constant.ConfigManagerGPRCPortEnv),
	}

	//if podSpec.HostNetwork {
	//	containerPort, err := GetConfigManagerGRPCPort(podSpec.Containers)
	//	if err != nil {
	//		return nil, err
	//	}
	//	cfgManagerParams.ContainerPort = containerPort
	//}

	if err := cfgcm.BuildConfigManagerContainerParams(cli, ctx, cfgManagerParams, volumeDirs); err != nil {
		return nil, err
	}
	if err := buildConfigToolsContainer(cfgManagerParams, podSpec, comp); err != nil {
		return nil, err
	}
	return cfgManagerParams, nil
}

func buildConfigManagerForLorryContainer(container *corev1.Container, buildParam *cfgcm.CfgManagerBuildParams, synthesizedComp *SynthesizedComponent) {
	container.Env = append(container.Env, corev1.EnvVar{
		Name: "CONFIG_MANAGER_POD_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "status.podIP",
			},
		},
	})
	if len(synthesizedComp.CharacterType) > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "DB_TYPE",
			Value: buildParam.CharacterType,
		})
	}
	if synthesizedComp.CharacterType == "mysql" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: "MYSQL_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  "username",
					LocalObjectReference: corev1.LocalObjectReference{Name: buildParam.SecreteName},
				},
			},
		},
			corev1.EnvVar{
				Name: "MYSQL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "password",
						LocalObjectReference: corev1.LocalObjectReference{Name: buildParam.SecreteName},
					},
				},
			},
			corev1.EnvVar{
				Name:  "DATA_SOURCE_NAME",
				Value: "$(MYSQL_USER):$(MYSQL_PASSWORD)@(localhost:3306)/",
			},
		)
	}

	if buildParam.ShareProcessNamespace {
		user := int64(0)
		container.SecurityContext = &corev1.SecurityContext{
			RunAsUser: &user,
		}
	}
	for _, v := range buildParam.Volumes {
		if !slices.Contains(container.VolumeMounts, v) {
			container.VolumeMounts = append(container.VolumeMounts, v)
		}
	}
	container.Args = append(container.Args, buildParam.Args...)
}

func checkAndUpdateShareProcessNamespace(podSpec *corev1.PodSpec, buildParams *cfgcm.CfgManagerBuildParams, configSpecMetas []cfgcm.ConfigSpecMeta) {
	shared := cfgcm.NeedSharedProcessNamespace(configSpecMetas)
	if shared {
		podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
	}
	buildParams.ShareProcessNamespace = shared
}

func updateEnvPath(container *corev1.Container, params *cfgcm.CfgManagerBuildParams) {
	if len(params.ScriptVolume) == 0 {
		return
	}
	scriptPath := make([]string, 0, len(params.ScriptVolume))
	for _, volume := range params.ScriptVolume {
		if vm := cfgcm.FindVolumeMount(params.Volumes, volume.Name); vm != nil {
			scriptPath = append(scriptPath, vm.MountPath)
		}
	}
	if len(scriptPath) != 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  cfgcm.KBConfigManagerPathEnv,
			Value: strings.Join(scriptPath, ":"),
		})
	}
}

func updateCfgManagerVolumes(podSpec *corev1.PodSpec, configManager *cfgcm.CfgManagerBuildParams) {
	scriptVolumes := configManager.ScriptVolume
	if len(scriptVolumes) == 0 && len(configManager.CMConfigVolumes) == 0 {
		return
	}

	podVolumes := podSpec.Volumes
	for _, vm := range []*[]corev1.Volume{
		&configManager.ScriptVolume,
		&configManager.CMConfigVolumes,
	} {
		for i := range *vm {
			podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, (*vm)[i].Name, func(string) corev1.Volume {
				return (*vm)[i]
			}, nil)
		}
	}
	podSpec.Volumes = podVolumes

	for volumeName, volume := range configManager.ConfigLazyRenderedVolumes {
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, volumeName)
		for _, container := range usingContainers {
			container.VolumeMounts = append(container.VolumeMounts, volume)
		}
	}
}

func buildConfigToolsContainer(cfgManagerParams *cfgcm.CfgManagerBuildParams, podSpec *corev1.PodSpec, comp *SynthesizedComponent) error {
	if len(cfgManagerParams.ConfigSpecsBuildParams) == 0 {
		return nil
	}

	// construct config manager tools volume
	toolContainers := make([]appsv1alpha1.ToolConfig, 0)
	toolsMap := make(map[string]cfgcm.ConfigSpecMeta)
	for _, buildParam := range cfgManagerParams.ConfigSpecsBuildParams {
		if buildParam.ToolsImageSpec == nil {
			continue
		}
		for _, toolConfig := range buildParam.ToolsImageSpec.ToolConfigs {
			if _, ok := toolsMap[toolConfig.Name]; !ok {
				replaceToolsImageHolder(&toolConfig, podSpec, buildParam.ConfigSpec.VolumeName)
				toolContainers = append(toolContainers, toolConfig)
				toolsMap[toolConfig.Name] = buildParam
			}
		}
		buildToolsVolumeMount(cfgManagerParams, podSpec, buildParam.ConfigSpec.VolumeName, buildParam.ToolsImageSpec.MountPoint)
	}

	// Ensure that the order in which iniContainers are generated does not change
	toolContainers = checkAndInstallToolsImageVolume(toolContainers, cfgManagerParams.ConfigSpecsBuildParams)
	if len(toolContainers) == 0 {
		return nil
	}

	containers, err := factory.BuildCfgManagerToolsContainer(cfgManagerParams, comp, toolContainers, toolsMap)
	if err == nil {
		cfgManagerParams.ToolsContainers = containers
	}
	return err
}

func checkAndInstallToolsImageVolume(toolContainers []appsv1alpha1.ToolConfig, buildParams []cfgcm.ConfigSpecMeta) []appsv1alpha1.ToolConfig {
	for _, buildParam := range buildParams {
		if buildParam.ToolsImageSpec != nil && buildParam.ConfigSpec.LegacyRenderedConfigSpec != nil {
			// auto install config_render tool
			toolContainers = checkAndCreateRenderedInitContainer(toolContainers, buildParam.ToolsImageSpec.MountPoint)
		}
	}
	return toolContainers
}

func checkAndCreateRenderedInitContainer(toolContainers []appsv1alpha1.ToolConfig, mountPoint string) []appsv1alpha1.ToolConfig {
	kbToolsImage := viper.GetString(constant.KBToolsImage)
	for _, container := range toolContainers {
		if container.Name == initSecRenderedToolContainerName {
			return nil
		}
	}
	toolContainers = append(toolContainers, appsv1alpha1.ToolConfig{
		Name:    initSecRenderedToolContainerName,
		Image:   kbToolsImage,
		Command: []string{"cp", tplRenderToolPath, mountPoint},
	})
	return toolContainers
}

func replaceToolsImageHolder(toolConfig *appsv1alpha1.ToolConfig, podSpec *corev1.PodSpec, volumeName string) {
	switch {
	case toolConfig.Image == constant.KBToolsImagePlaceHolder:
		toolConfig.Image = viper.GetString(constant.KBToolsImage)
	case toolConfig.Image == "":
		usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, volumeName)
		if len(usingContainers) != 0 {
			toolConfig.Image = usingContainers[0].Image
		}
	}
}

func buildToolsVolumeMount(cfgManagerParams *cfgcm.CfgManagerBuildParams, podSpec *corev1.PodSpec, volumeName string, mountPoint string) {
	if cfgcm.FindVolumeMount(cfgManagerParams.Volumes, toolsVolumeName) != nil {
		return
	}
	cfgManagerParams.ScriptVolume = append(cfgManagerParams.ScriptVolume, corev1.Volume{
		Name: toolsVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	n := len(cfgManagerParams.Volumes)
	cfgManagerParams.Volumes = append(cfgManagerParams.Volumes, corev1.VolumeMount{
		Name:      toolsVolumeName,
		MountPath: mountPoint,
	})

	usingContainers := intctrlutil.GetPodContainerWithVolumeMount(podSpec, volumeName)
	for _, container := range usingContainers {
		container.VolumeMounts = append(container.VolumeMounts, cfgManagerParams.Volumes[n])
	}
}
