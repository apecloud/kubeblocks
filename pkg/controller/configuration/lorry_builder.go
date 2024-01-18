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

package configuration

import (
	"context"
	"golang.org/x/exp/slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

// BuildConfigManagerWithComponentForLorry inject the config manager service into Lorry container if configuration reload option is on
func BuildConfigManagerWithComponentForLorry(podSpec *corev1.PodSpec, configSpecs []appsv1alpha1.ComponentConfigSpec, container *corev1.Container,
	ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, synthesizedComp *component.SynthesizedComponent) error {
	var err error
	var buildParams *cfgcm.CfgManagerBuildParams

	volumeDirs, usingConfigSpecs := getUsingVolumesByConfigSpecs(podSpec, configSpecs)
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

	if buildParams, err = buildLorryConfigManagerParams(cli, ctx, cluster, synthesizedComp, configSpecMetas, volumeDirs, podSpec); err != nil {
		return err
	}
	if buildParams == nil {
		return nil
	}

	// This sidecar container will be able to view and signal processes from other containers
	checkAndUpdateShareProcessNamespace(podSpec, buildParams, configSpecMetas)

	// for lorry
	buildConfigManagerForLorryContainer(container, buildParams, synthesizedComp)

	updateEnvPath(container, buildParams)
	updateCfgManagerVolumes(podSpec, buildParams)
	if len(buildParams.ToolsContainers) > 0 {
		podSpec.InitContainers = append(podSpec.InitContainers, buildParams.ToolsContainers...)
	}

	filter := func(c *corev1.Container) bool {
		names := []string{container.Name}
		for _, cc := range buildParams.ToolsContainers {
			names = append(names, cc.Name)
		}
		return slices.Contains(names, c.Name)
	}
	component.InjectEnvVars4Containers(synthesizedComp, synthesizedComp.EnvVars, synthesizedComp.EnvFromSources, filter)
	return nil
}

func buildLorryConfigManagerParams(cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster, comp *component.SynthesizedComponent, configSpecBuildParams []cfgcm.ConfigSpecMeta, volumeDirs []corev1.VolumeMount, podSpec *corev1.PodSpec) (*cfgcm.CfgManagerBuildParams, error) {
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

func buildConfigManagerForLorryContainer(container *corev1.Container, buildParam *cfgcm.CfgManagerBuildParams, synthesizedComp *component.SynthesizedComponent) {
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
