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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	scriptName         = "reload.tpl"
	configTemplateName = "reload.yaml"
	scriptVolumeName   = "reload-manager-reload"
	scriptVolumePath   = "/opt/config/reload"
)

func BuildConfigManagerContainerArgs(reloadOptions *appsv1alpha1.ReloadOptions, volumeDirs []corev1.VolumeMount, cli client.Client, ctx context.Context, manager *ConfigManagerParams) error {
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

func buildTPLScriptArgs(options *appsv1alpha1.TPLScriptTrigger, volumeDirs []corev1.VolumeMount, cli client.Client, ctx context.Context, manager *ConfigManagerParams) error {
	scriptCMName := fmt.Sprintf("%s-%s", options.ScriptConfigMapRef, manager.Cluster.GetName())
	if err := checkOrCreateScriptCM(options, client.ObjectKey{
		Namespace: manager.Cluster.GetNamespace(),
		Name:      scriptCMName,
	}, cli, ctx, manager.Cluster); err != nil {
		return err
	}

	args := buildConfigManagerCommonArgs(volumeDirs)
	if options.Sync != nil && *options.Sync {
		args = append(args, "--operator-update-enable")
	}
	args = append(args, "--tcp", viper.GetString(constant.ConfigManagerGPRCPortEnv))
	args = append(args, "--notify-type", string(appsv1alpha1.TPLScriptType))
	args = append(args, "--tpl-config", filepath.Join(scriptVolumePath, configTemplateName))
	manager.Args = args
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
	return nil
}

func checkOrCreateScriptCM(options *appsv1alpha1.TPLScriptTrigger, scriptCMKey client.ObjectKey, cli client.Client, ctx context.Context, cluster *appsv1alpha1.Cluster) error {
	var (
		err error

		ccCM      = corev1.ConfigMap{}
		sidecarCM = corev1.ConfigMap{}
	)

	if err = cli.Get(ctx, client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}, &ccCM); err != nil {
		return err
	}
	if _, ok := ccCM.Data[scriptName]; !ok {
		return cfgutil.MakeError("configmap not exist script: %s", scriptName)
	}

	if err = cli.Get(ctx, scriptCMKey, &sidecarCM); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		sidecarCM.Data = ccCM.Data
		sidecarCM.SetLabels(ccCM.GetLabels())
		sidecarCM.SetName(scriptCMKey.Name)
		sidecarCM.SetNamespace(scriptCMKey.Namespace)
		sidecarCM.SetLabels(ccCM.Labels)
		if err := controllerutil.SetOwnerReference(cluster, &sidecarCM, scheme); err != nil {
			return err
		}
		if err := cli.Create(ctx, &sidecarCM); err != nil {
			return err
		}
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
	args = append(args, "--log-level", viper.GetString(constant.ConfigManagerLogLevel))
	for _, volume := range volumeDirs {
		args = append(args, "--volume-dir", volume.MountPath)
	}
	return args
}
