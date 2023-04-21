/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package configmanager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	configTemplateName   = "reload.yaml"
	scriptVolumeName     = "reload-manager-reload"
	scriptVolumePath     = "/opt/config/reload"
	scriptConfigField    = "scripts"
	formatterConfigField = "formatterConfig"
)

func BuildConfigManagerContainerArgs(reloadOptions *appsv1alpha1.ReloadOptions, volumeDirs []corev1.VolumeMount, cli client.Client, ctx context.Context, manager *CfgManagerBuildParams, formatterConfig *appsv1alpha1.FormatterConfig) error {
	switch {
	case reloadOptions.UnixSignalTrigger != nil:
		manager.Args = buildSignalArgs(*reloadOptions.UnixSignalTrigger, volumeDirs)
		return nil
	case reloadOptions.ShellTrigger != nil:
		return buildShellArgs(*reloadOptions.ShellTrigger, volumeDirs, manager, cli, ctx)
	case reloadOptions.TPLScriptTrigger != nil:
		return buildTPLScriptArgs(reloadOptions.TPLScriptTrigger, volumeDirs, cli, ctx, manager, formatterConfig)
	}
	return cfgutil.MakeError("not support reload.")
}

func buildTPLScriptArgs(options *appsv1alpha1.TPLScriptTrigger, volumeDirs []corev1.VolumeMount, cli client.Client, ctx context.Context, manager *CfgManagerBuildParams, formatterConfig *appsv1alpha1.FormatterConfig) error {
	reloadYamlFn := func(cm *corev1.ConfigMap) error {
		newData, err := checkAndUpdateReloadYaml(cm.Data, configTemplateName, formatterConfig)
		if err != nil {
			return err
		}
		cm.Data = newData
		return nil
	}
	referenceCMKey := client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}
	scriptCMKey := client.ObjectKey{
		Namespace: manager.Cluster.GetNamespace(),
		Name:      fmt.Sprintf("%s-%s", options.ScriptConfigMapRef, manager.Cluster.GetName()),
	}
	if err := checkOrCreateScriptCM(referenceCMKey, scriptCMKey, cli, ctx, manager.Cluster, reloadYamlFn); err != nil {
		return err
	}

	var args []string
	if options.Sync != nil && *options.Sync {
		args = append(args, "--operator-update-enable")
		args = append(args, "--log-level", viper.GetString(constant.ConfigManagerLogLevel))
	} else {
		args = buildConfigManagerCommonArgs(volumeDirs)
	}
	args = append(args, "--tcp", viper.GetString(constant.ConfigManagerGPRCPortEnv))
	args = append(args, "--notify-type", string(appsv1alpha1.TPLScriptType))
	args = append(args, "--tpl-config", filepath.Join(scriptVolumePath, configTemplateName))
	manager.Args = args

	buildReloadScriptVolume(scriptCMKey.Name, manager)
	return nil
}

func buildReloadScriptVolume(scriptCMName string, manager *CfgManagerBuildParams) {
	manager.Volumes = append(manager.Volumes, corev1.VolumeMount{
		Name:      scriptVolumeName,
		MountPath: scriptVolumePath,
	})
	manager.ScriptVolume = &corev1.Volume{
		Name: scriptVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: scriptCMName},
			},
		},
	}
}

func checkOrCreateScriptCM(referenceCM client.ObjectKey, scriptCMKey client.ObjectKey, cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster, fn func(cm *corev1.ConfigMap) error) error {
	var (
		err error

		refCM     = corev1.ConfigMap{}
		sidecarCM = corev1.ConfigMap{}
	)

	if err = cli.Get(ctx, referenceCM, &refCM); err != nil {
		return err
	}
	if err = cli.Get(ctx, scriptCMKey, &sidecarCM); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		sidecarCM.Data = refCM.Data
		if fn != nil && fn(&sidecarCM) != nil {
			return err
		}
		sidecarCM.SetLabels(refCM.GetLabels())
		sidecarCM.SetName(scriptCMKey.Name)
		sidecarCM.SetNamespace(scriptCMKey.Namespace)
		sidecarCM.SetLabels(refCM.Labels)
		if err := controllerutil.SetOwnerReference(cluster, &sidecarCM, scheme); err != nil {
			return err
		}
		if err := cli.Create(ctx, &sidecarCM); err != nil {
			return err
		}
	}
	return nil
}

func checkAndUpdateReloadYaml(data map[string]string, reloadConfig string, formatterConfig *appsv1alpha1.FormatterConfig) (map[string]string, error) {
	configObject := make(map[string]interface{})
	if content, ok := data[reloadConfig]; ok {
		if err := yaml.Unmarshal([]byte(content), &configObject); err != nil {
			return nil, err
		}
	}
	if res, _, _ := unstructured.NestedFieldNoCopy(configObject, scriptConfigField); res == nil {
		return nil, cfgutil.MakeError("reload.yaml required field: %s", scriptConfigField)
	}

	formatObject, err := apiruntime.DefaultUnstructuredConverter.ToUnstructured(formatterConfig)
	if err != nil {
		return nil, err
	}
	if err := unstructured.SetNestedField(configObject, formatObject, formatterConfigField); err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(configObject)
	if err != nil {
		return nil, err
	}
	data[reloadConfig] = string(b)
	return data, nil
}

func buildShellArgs(options appsv1alpha1.ShellTrigger, volumeDirs []corev1.VolumeMount, manager *CfgManagerBuildParams, cli client.Client, ctx context.Context) error {
	command := strings.Trim(options.Exec, " \t")
	if command == "" {
		return cfgutil.MakeError("invalid command: [%s]", options.Exec)
	}
	args := buildConfigManagerCommonArgs(volumeDirs)
	args = append(args, "--notify-type", string(appsv1alpha1.ShellType))
	args = append(args, "---command", command)
	manager.Args = args

	if options.ScriptConfigMapRef == "" {
		return nil
	}

	return buildShellScriptCM(options, manager, cli, ctx)
}

func buildShellScriptCM(options appsv1alpha1.ShellTrigger, manager *CfgManagerBuildParams, cli client.Client, ctx context.Context) error {
	referenceCMKey := client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}
	scriptsCMKey := client.ObjectKey{
		Namespace: manager.Cluster.GetNamespace(),
		Name:      fmt.Sprintf("%s-%s", options.ScriptConfigMapRef, manager.Cluster.GetName()),
	}
	if err := checkOrCreateScriptCM(referenceCMKey, scriptsCMKey, cli, ctx, manager.Cluster, nil); err != nil {
		return err
	}
	buildReloadScriptVolume(scriptsCMKey.Name, manager)
	return nil
}

func buildSignalArgs(options appsv1alpha1.UnixSignalTrigger, volumeDirs []corev1.VolumeMount) []string {
	args := buildConfigManagerCommonArgs(volumeDirs)
	args = append(args, "--process", options.ProcessName)
	args = append(args, "--signal", string(options.Signal))
	args = append(args, "--notify-type", string(appsv1alpha1.UnixSignalType))
	return args
}

func buildConfigManagerCommonArgs(volumeDirs []corev1.VolumeMount) []string {
	args := make([]string, 0)
	// set grpc port
	// args = append(args, "--tcp", viper.GetString(cfgutil.ConfigManagerGPRCPortEnv))
	args = append(args, "--log-level", viper.GetString(constant.ConfigManagerLogLevel))
	for _, volume := range volumeDirs {
		args = append(args, "--volume-dir", volume.MountPath)
	}
	return args
}
