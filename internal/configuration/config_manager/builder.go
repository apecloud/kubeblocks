/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configmanager

import (
	"context"
	client2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
)

func BuildConfigManagerContainerArgs(reloadOptions *appsv1alpha1.ReloadOptions, volumeDirs []corev1.VolumeMount, cli client2.ReadonlyClient, ctx context.Context, manager *ConfigManagerParams) error {
	switch {
	case reloadOptions.UnixSignalTrigger != nil:
		manager.Args = buildSignalArgs(*reloadOptions.UnixSignalTrigger, volumeDirs)
		return nil
	case reloadOptions.ShellTrigger != nil:
		return buildShellArgs(*reloadOptions.ShellTrigger, volumeDirs, manager)
	case reloadOptions.TPLScriptTrigger != nil:
		return buildTPLScriptArgs(reloadOptions.TPLScriptTrigger, volumeDirs, cli, ctx, manager)
	}
	return cfgutil.MakeError("not support reload.")
}

func buildTPLScriptArgs(options *appsv1alpha1.TPLScriptTrigger, volumeDirs []corev1.VolumeMount, cli client2.ReadonlyClient, ctx context.Context, manager *ConfigManagerParams) error {
	const (
		scriptName       = "reload.tpl"
		tplConfigName    = "reload.yaml"
		scriptVolumeName = "reload-manager-reload"
		scriptVolumePath = "/opt/config/reload"
	)

	tplScript := corev1.ConfigMap{}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}, &tplScript); err != nil {
		return err
	}
	if _, ok := tplScript.Data[scriptName]; !ok {
		return cfgutil.MakeError("configmap not exist script: %s", scriptName)
	}

	args := buildConfigManagerCommonArgs(volumeDirs)
	args = append(args, "--notify-type", string(appsv1alpha1.TPLScriptType))
	args = append(args, "--tpl-config", filepath.Join(scriptVolumePath, tplConfigName))
	manager.Args = args
	manager.Volumes = append(manager.Volumes, corev1.VolumeMount{
		Name:      scriptVolumeName,
		MountPath: scriptVolumePath,
	})
	manager.ScriptVolume = &corev1.Volume{
		Name: scriptVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: options.ScriptConfigMapRef},
			},
		},
	}
	return nil
}

func buildShellArgs(options appsv1alpha1.ShellTrigger, volumeDirs []corev1.VolumeMount, manager *ConfigManagerParams) error {
	command := strings.Trim(options.Exec, " \t")
	if command == "" {
		return cfgutil.MakeError("invalid command: [%s]", options.Exec)
	}
	args := buildConfigManagerCommonArgs(volumeDirs)
	args = append(args, "--notify-type", string(appsv1alpha1.ShellType))
	args = append(args, "---command", command)
	manager.Args = args
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
	for _, volume := range volumeDirs {
		args = append(args, "--volume-dir", volume.MountPath)
	}
	return args
}
