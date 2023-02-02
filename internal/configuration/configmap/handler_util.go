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

func IsSupportReload(reload *dbaasv1alpha1.ReloadOptions) bool {
	return reload != nil && (reload.ShellTrigger != nil || reload.UnixSignalTrigger != nil)
}

func NeedBuildConfigSidecar(reloadOptions *dbaasv1alpha1.ReloadOptions) error {
	switch {
	case reloadOptions.UnixSignalTrigger != nil:
		signal := reloadOptions.UnixSignalTrigger.Signal
		if !IsValidUnixSignal(signal) {
			return cfgutil.MakeError("this special signal [%s] is not supported for now!", signal)
		}
		return nil
	default:
		// TODO support sql or http
		return cfgutil.MakeError("this special reload type [%s] is not supported for now!", dbaasv1alpha1.SQLType)
	}
}

func BuildSignalArgs(configuration dbaasv1alpha1.UnixSignalTrigger, volumeDirs []corev1.VolumeMount) []string {
	args := make([]string, 0)
	args = append(args, "--process", configuration.ProcessName)
	args = append(args, "--signal", string(configuration.Signal))
	// set grpc port
	// args = append(args, "--tcp", viper.GetString(cfgutil.ConfigManagerGPRCPortEnv))
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
		if !event.Has(fsnotify.Create) {
			return false, nil
		}
		if filepath.Base(event.Name) != "..data" {
			return false, nil
		}
		return true, nil
	}
}
