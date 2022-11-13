/*
Copyright 2022.

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

package configmap

import (
	"path/filepath"
	"regexp"

	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
)

type ConfigManagerSidecar struct {
	ManagerName string          `json:"name"`
	Image       string          `json:"sidecarImage"`
	Args        []string        `json:"args"`
	Envs        []corev1.EnvVar `json:"envs"`

	Volumes []corev1.VolumeMount `json:"volumes"`
}

func NeedBuildConfigSidecar(autoReload bool, reloadType string, configuration dbaasv1alpha1.ReloadConfiguration) (bool, error) {
	if autoReload || reloadType == "" {
		return false, nil
	}

	switch reloadType {
	case dbaasv1alpha1.UnixSignal:
		return checkSignalType(configuration)
	case dbaasv1alpha1.SqlReload, dbaasv1alpha1.ExecReload, dbaasv1alpha1.HttpReload:
		// TODO support other way
		return false, cfgutil.MakeError("This special reload type [%s] is not supported for now!", reloadType)
	default:
		return false, cfgutil.MakeError("Invalid Features: %s", reloadType)
	}
}

func checkSignalType(configuration dbaasv1alpha1.ReloadConfiguration) (bool, error) {
	if IsValidUnixSignal(configuration.Signal) {
		return false, cfgutil.MakeError("This special signal [%s] is not supported for now!", configuration.Signal)
	}
	if configuration.ProcessName == "" {
		return false, cfgutil.MakeError("require set process name!")
	}
	return true, nil
}

func BuildReloadSidecarParams(reloadType string, configuration dbaasv1alpha1.ReloadConfiguration, volumeDirs []corev1.VolumeMount) []string {
	switch reloadType {
	case dbaasv1alpha1.UnixSignal:
		return buildSignalArgs(configuration, volumeDirs)
	default:
		// not walk here
		return nil
	}
}

func buildSignalArgs(configuration dbaasv1alpha1.ReloadConfiguration, volumeDirs []corev1.VolumeMount) []string {
	args := make([]string, 0)
	args = append(args, "--process", configuration.ProcessName)
	for _, volume := range volumeDirs {
		args = append(args, "--volume-dir", volume.MountPath)
	}
	return args
}

func CreateCfgRegexFilter(regexString string) (NotifyEventFilter, error) {
	regxPattern, err := regexp.Compile(regexString)
	if err != nil {
		return nil, cfgutil.WrapError(err, "failed to create regexp [%s]", regexString)
	}

	return func(event fsnotify.Event) (bool, error) {
		return regxPattern.MatchString(event.Name), nil
	}, nil
}

// CreateValidConfigMapFilter process configmap volume
// https://github.com/ossrs/srs/issues/1635
func CreateValidConfigMapFilter() NotifyEventFilter {
	return func(event fsnotify.Event) (bool, error) {
		if event.Op&fsnotify.Create != fsnotify.Create {
			return false, nil
		}
		if filepath.Base(event.Name) != "..data" {
			return false, nil
		}
		return true, nil
	}
}
